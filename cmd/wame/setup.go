package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// runSetup prepares the persistent store directory (~/.wame/store).
// MCP client registration is handled centrally by mcps.
func runSetup(port, storeDir string) error {
	fmt.Printf("🔧 wame setup (v%s)\n\n", Version)

	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return fmt.Errorf("create store dir %s: %w", storeDir, err)
	}
	fmt.Printf("✅ Store directory ready: %s\n", storeDir)

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
     wame service start   (or: brew services start wame)
`)
	} else {
		fmt.Printf("2. Daemon is already running at http://localhost:%s/mcp\n", port)
	}
	fmt.Printf(`3. Register endpoint via mcps:
     mcps add whatsapp --url http://localhost:%s/mcp
4. Run diagnostics anytime:
     wame doctor
========================================================
`, port)
	return nil
}
