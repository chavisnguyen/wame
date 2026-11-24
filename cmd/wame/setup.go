package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"wame"
)

// runSetup prepares the persistent store directory and registers the "whatsapp" MCP server
// across all detected AI clients using the embedded install.sh script.
func runSetup(port, storeDir string) error {
	fmt.Printf("🔧 wame setup (v%s)\n\n", Version)

	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return fmt.Errorf("create store dir %s: %w", storeDir, err)
	}
	fmt.Printf("✅ Store directory ready: %s\n", storeDir)

	exe, _ := os.Executable()
	cmd := exec.Command("bash", "-s", "sse")
	cmd.Stdin = strings.NewReader(string(wame.InstallScript))
	cmd.Env = append(os.Environ(),
		"WAME_BIN="+exe,
		"PORT="+port,
	)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Could not auto-register MCP in your agents (%v).\n   Add this URL manually: http://localhost:%s/mcp\n", err, port)
	}

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	daemonRunning := false
	if resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port)); err == nil {
		_ = resp.Body.Close()
		daemonRunning = true
	}

	fmt.Printf(`
========================================================
🎉 wame setup complete!
========================================================
1. Link WhatsApp (if not paired yet):
     wame login
`)
	if !daemonRunning {
		fmt.Printf(`2. Start background daemon:
     brew services start wame   (or: make daemon)
`)
	} else {
		fmt.Printf("2. Daemon is already running at http://localhost:%s/mcp\n", port)
	}
	fmt.Printf(`3. Run diagnostics anytime:
     wame doctor
========================================================
`)
	return nil
}
