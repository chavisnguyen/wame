package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"wame/internal/config"
	"wame/internal/mcpserver"
	"wame/internal/store"
	"wame/internal/whatsapp"
)

// Version is overridden at release build time via -ldflags "-X main.Version=...".
var Version = "2.1.0"

func init() {
	config.LoadDotEnv(".env", filepath.Join(config.HomeDir(), ".env"))
}

func main() {
	defaultPort := "3026"
	if p := os.Getenv("PORT"); p != "" {
		defaultPort = p
	}
	defaultHost := os.Getenv("HOST")

	sseMode := flag.Bool("sse", false, "Run in HTTP/SSE Server daemon mode (shared for multiple agents)")
	stdioMode := flag.Bool("stdio", false, "Run MCP server over stdio")
	port := flag.String("port", defaultPort, "Port to bind HTTP/SSE server (e.g. 3026)")
	host := flag.String("host", defaultHost, "Specific host to bind (default empty: binds dual-stack 127.0.0.1 and [::1])")
	storeDirFlag := flag.String("store-dir", "", "Directory for WhatsApp session and message DB (default ~/.wame/store)")
	verbose := flag.Bool("verbose", false, "Enable verbose WhatsApp client logs")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	vFlag := flag.Bool("v", false, "Print version and exit")

	flag.Usage = printUsage
	flag.Parse()

	cmd := "serve"
	if flag.NArg() > 0 {
		cmd = flag.Arg(0)
	}

	if *versionFlag || *vFlag || cmd == "version" {
		fmt.Printf("wame v%s\n", Version)
		return
	}

	if cmd == "help" {
		printUsage()
		return
	}

	resolvedStoreDir, migratedFrom, err := config.ResolveStoreDir(*storeDirFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: store migration failed: %v\n", err)
	} else if migratedFrom != "" {
		fmt.Fprintf(os.Stderr, "📦 Migrated existing store from '%s' to '%s'\n", migratedFrom, resolvedStoreDir)
	}

	if *sseMode || cmd == "daemon" {
		runDaemon(*port, *host, resolvedStoreDir, *verbose)
		return
	}

	if *stdioMode {
		runServe(resolvedStoreDir, *verbose)
		return
	}

	switch cmd {
	case "setup":
		if err := runSetup(*port, resolvedStoreDir); err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
		return

	case "login":
		runLogin(resolvedStoreDir, *verbose)
		return

	case "status":
		runStatus(resolvedStoreDir)
		return

	case "doctor":
		runDoctor(*port, resolvedStoreDir)
		return

	case "serve":
		runServe(resolvedStoreDir, *verbose)
		return

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", cmd)
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, `wame v%s — Lightweight WhatsApp MCP Server

Usage:
  wame [flags] [command]

Commands:
  setup       One-command setup: prepare store & register "whatsapp" MCP in AI clients
  daemon      Run HTTP/SSE daemon (shared endpoint http://localhost:3026/mcp)
  serve       Run MCP server over stdio (default when -sse is omitted)
  login       Pair WhatsApp account by scanning QR code in terminal
  status      Check current WhatsApp login & session status
  doctor      Diagnose store, WhatsApp session, FFmpeg, daemon, and AI client configs
  version     Show application version
  help        Show this help

Flags:
`, Version)
	flag.PrintDefaults()
}

func runLogin(storeDir string, verbose bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nCancelled by user.")
		cancel()
	}()

	msgStore, err := store.NewStore(storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening message store: %v\n", err)
		os.Exit(1)
	}
	defer msgStore.Close()

	client, err := whatsapp.NewClient(ctx, storeDir, msgStore, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing WhatsApp client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	if client.IsLoggedIn() {
		status := client.GetStatus()
		fmt.Printf("✓ Device is already authenticated with WhatsApp! (Account: %s, Phone: %s)\n", status.PushName, status.Phone)
		return
	}

	fmt.Println("Generating pairing QR code...")
	if err := client.Connect(ctx, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Login error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Login complete! You can now start the daemon ('brew services start wame' or 'make daemon').")
}

func runStatus(storeDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msgStore, err := store.NewStore(storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening store: %v\n", err)
		os.Exit(1)
	}
	defer msgStore.Close()

	client, err := whatsapp.NewClient(ctx, storeDir, msgStore, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking status: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	if client.IsLoggedIn() {
		status := client.GetStatus()
		fmt.Printf("Status: Authenticated (Phone: %s, Account: %s, Store: %s)\n", status.Phone, status.PushName, storeDir)
	} else {
		fmt.Printf("Status: Not authenticated. Run 'wame login' to scan QR code. (Store: %s)\n", storeDir)
	}
}

func runServe(storeDir string, verbose bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgStore, err := store.NewStore(storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to initialize message store: %v\n", err)
		os.Exit(1)
	}
	defer msgStore.Close()

	client, err := whatsapp.NewClient(ctx, storeDir, msgStore, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to initialize WhatsApp client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// Connect in background so MCP stdio server starts immediately
	go func() {
		// Output QR code to stderr if device needs pairing so stdout (JSON-RPC) is clean
		if err := client.Connect(ctx, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "[wame] Connection error: %v\n", err)
		}
	}()

	srv := mcpserver.NewServer(client, msgStore, Version)
	if err := srv.ServeStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "[wame] Server error: %v\n", err)
		os.Exit(1)
	}
}

func setupHTTPHandler(srv *mcpserver.Server, client *whatsapp.Client, msgStore *store.Store) http.Handler {
	streamableServer := server.NewStreamableHTTPServer(srv.GetMCPServer(),
		server.WithStreamableHTTPCORS(),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", streamableServer)
	mux.Handle("/sse", streamableServer)
	mux.HandleFunc("/.well-known/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"status":  "ok",
			"version": Version,
		}
		if client != nil {
			status := client.GetStatus()
			resp["whatsapp_online"] = status.Connected
			resp["whatsapp_auth"] = status.LoggedIn
			resp["phone"] = status.Phone
		}
		if msgStore != nil {
			chatCount, msgCount, _ := msgStore.GetStats()
			resp["indexed_chats"] = chatCount
			resp["indexed_messages"] = msgCount
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject unexpected browser Origin headers on MCP/health endpoints (DNS rebinding / CSRF defense)
		if r.URL.Path == "/mcp" || r.URL.Path == "/sse" || r.URL.Path == "/health" {
			origin := r.Header.Get("Origin")
			if !config.IsOriginAllowed(origin) {
				http.Error(w, "Forbidden: Origin not allowed", http.StatusForbidden)
				return
			}
		}

		if r.Method == http.MethodPost {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				if strings.Contains(string(bodyBytes), `"notifications/cancelled"`) {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func runDaemon(port, host, storeDir string, verbose bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	msgStore, err := store.NewStore(storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to initialize message store: %v\n", err)
		os.Exit(1)
	}
	defer msgStore.Close()

	client, err := whatsapp.NewClient(ctx, storeDir, msgStore, verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to initialize WhatsApp client: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// Connect to WhatsApp in background
	go func() {
		if err := client.Connect(ctx, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "[wame] Background connection error: %v\n", err)
		}
	}()

	srv := mcpserver.NewServer(client, msgStore, Version)

	var listeners []net.Listener
	if host != "" {
		ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to bind %s:%s: %v\n", host, port, err)
			os.Exit(1)
		}
		listeners = append(listeners, ln)
	} else {
		// Dual-stack loopback binding: IPv4 and IPv6
		ln4, err4 := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", port))
		if err4 != nil {
			fmt.Fprintf(os.Stderr, "Failed to bind IPv4 loopback 127.0.0.1:%s: %v\n", port, err4)
		} else {
			listeners = append(listeners, ln4)
		}

		ln6, err6 := net.Listen("tcp6", net.JoinHostPort("::1", port))
		if err6 != nil {
			if err4 != nil {
				fmt.Fprintf(os.Stderr, "Failed to bind IPv6 loopback [::1]:%s: %v\n", port, err6)
			}
		} else {
			listeners = append(listeners, ln6)
		}

		if len(listeners) == 0 {
			fmt.Fprintf(os.Stderr, "Error: failed to bind any loopback interface on port %s\n", port)
			os.Exit(1)
		}
	}

	httpServer := &http.Server{
		Handler: setupHTTPHandler(srv, client, msgStore),
	}

	fmt.Printf("🚀 Starting wame WhatsApp MCP Server (HTTP/SSE Shared Daemon) on port %s\n", port)
	fmt.Printf("   Store: %s\n", storeDir)
	for _, l := range listeners {
		fmt.Printf("   Listening on http://%s/mcp\n", l.Addr())
	}
	fmt.Printf("   Endpoints supported: /mcp, /sse, /health\n")

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	for _, l := range listeners {
		go func(ln net.Listener) {
			if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "HTTP Server error on %s: %v\n", ln.Addr(), err)
			}
		}(l)
	}

	<-stopChan
	fmt.Println("\n🛑 Shutting down wame MCP Server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	fmt.Println("👋 Goodbye!")
}

func runDoctor(port, storeDir string) {
	fmt.Printf("🩺 wame Doctor (v%s)\n\n", Version)

	// 1. Store & WhatsApp Authentication
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msgStore, err := store.NewStore(storeDir)
	if err != nil {
		fmt.Printf("🔴 SQLite Store: Error opening store in '%s': %v\n", storeDir, err)
	} else {
		chatCount, msgCount, _ := msgStore.GetStats()
		fmt.Printf("🟢 SQLite Store: OK in '%s' (%d chats, %d messages indexed)\n", storeDir, chatCount, msgCount)
		client, clientErr := whatsapp.NewClient(ctx, storeDir, msgStore, false)
		if clientErr == nil {
			if client.IsLoggedIn() {
				status := client.GetStatus()
				fmt.Printf("🟢 WhatsApp Session: Authenticated (Phone: %s, Account: %s)\n", status.Phone, status.PushName)
			} else {
				fmt.Printf("🟡 WhatsApp Session: Not authenticated yet. Run 'wame login' to scan QR code.\n")
			}
			client.Disconnect()
		}
		msgStore.Close()
	}

	// 2. FFmpeg check
	if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
		fmt.Printf("🟢 FFmpeg: Installed at %s (Voice messages / Opus PTT enabled)\n", ffmpegPath)
	} else {
		fmt.Printf("🟡 FFmpeg: Not found in PATH (Voice audio will fall back to send_file)\n")
	}

	// 3. HTTP/SSE Daemon status
	daemonURL := fmt.Sprintf("http://127.0.0.1:%s/health", port)
	httpClient := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := httpClient.Get(daemonURL)
	if err != nil {
		fmt.Printf("⚪ Daemon (:%s): Not running (Run 'brew services start wame' or 'make daemon')\n", port)
	} else {
		_ = resp.Body.Close()
		fmt.Printf("🟢 Daemon (:%s): Running and reachable at http://localhost:%s/mcp\n", port, port)
	}

	// 4. AI Client Configuration Checks
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	fmt.Println("\n🔍 AI Client Configuration Status:")

	checkClientConfig("Antigravity (AGY)", filepath.Join(home, ".gemini/config/mcp_config.json"))
	checkOpenCodeConfig(filepath.Join(home, ".config/opencode"))
	checkClientConfig("Claude Code (CLI)", filepath.Join(home, ".claude.json"))
	checkClientConfig("Kiro", filepath.Join(home, ".kiro/settings/mcp.json"))
	checkClientConfig("Claude Desktop", filepath.Join(home, "Library/Application Support/Claude/claude_desktop_config.json"))
	checkClientConfig("Codex", filepath.Join(codexHome, "config.toml"))

	fmt.Printf("\n✨ Doctor check complete! Run 'wame setup' to configure clients automatically.\n")
}

func checkOpenCodeConfig(dir string) {
	for _, name := range []string{"opencode.jsonc", "opencode.json"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			checkClientConfig("OpenCode", p)
			return
		}
	}
	fmt.Printf("  ⚪ OpenCode: Config file not found in %s\n", dir)
}

func checkClientConfig(name, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("  ⚪ %s: Config file not found at %s\n", name, path)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("  🔴 %s: Cannot read %s: %v\n", name, path, err)
		return
	}

	content := string(data)
	if strings.Contains(content, "1craft-wa-mcp") || strings.Contains(content, "whatsapp-mcp") {
		fmt.Printf("  🟡 %s: Legacy path found in %s (run 'wame setup' to update)\n", name, path)
		return
	}
	if strings.Contains(content, "whatsapp") {
		fmt.Printf("  🟢 %s: Configured in %s\n", name, path)
	} else {
		fmt.Printf("  🟡 %s: File exists but 'whatsapp' server is not configured yet\n", name)
	}
}
