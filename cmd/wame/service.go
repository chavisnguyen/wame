package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wame/internal/config"
)

const wameLaunchdLabel = "com.chavisnguyen.wame"

func wamePlistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", wameLaunchdLabel+".plist")
}

func wameSystemdUnitPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "wame.service")
}

func runServiceCmd(action, port string) error {
	if action == "" {
		action = "status"
	}
	switch action {
	case "install", "start":
		return wameServiceStart(port)
	case "stop":
		return wameServiceStop()
	case "restart":
		_ = wameServiceStop()
		return wameServiceStart(port)
	case "uninstall":
		return wameServiceUninstall()
	case "status":
		wameServiceStatus(port)
		return nil
	default:
		return fmt.Errorf("unknown service action %q (expected: start | stop | restart | status | uninstall)", action)
	}
}

func resolveWameBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real, nil
	}
	return exe, nil
}

func wameServiceStart(port string) error {
	exe, err := resolveWameBinary()
	if err != nil {
		return err
	}
	dir := config.HomeDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	logFile := filepath.Join(dir, "daemon.log")

	switch runtime.GOOS {
	case "darwin":
		p := wamePlistPath()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>-sse</string>
    <string>-port</string>
    <string>%s</string>
  </array>
  <key>WorkingDirectory</key>
  <string>%s</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, wameLaunchdLabel, exe, port, dir, logFile, logFile)
		if err := os.WriteFile(p, []byte(plist), 0o644); err != nil {
			return err
		}
		_ = exec.Command("launchctl", "unload", p).Run()
		if out, err := exec.Command("launchctl", "load", "-w", p).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl load: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		fmt.Printf("🟢 wame service started on :%s (log: %s)\n", port, logFile)
		return nil

	case "linux":
		u := wameSystemdUnitPath()
		if err := os.MkdirAll(filepath.Dir(u), 0o755); err != nil {
			return err
		}
		unit := fmt.Sprintf(`[Unit]
Description=wame WhatsApp MCP Daemon
After=network.target

[Service]
ExecStart=%s -sse -port %s
WorkingDirectory=%s
Restart=always
RestartSec=2
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=default.target
`, exe, port, dir, logFile, logFile)
		if err := os.WriteFile(u, []byte(unit), 0o644); err != nil {
			return err
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		if out, err := exec.Command("systemctl", "--user", "enable", "--now", "wame.service").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl enable --now: %v (%s)", err, strings.TrimSpace(string(out)))
		}
		fmt.Printf("🟢 wame service started on :%s (log: %s)\n", port, logFile)
		return nil

	default:
		return fmt.Errorf("native service management is only supported on macOS and Linux")
	}
}

func wameServiceStop() error {
	switch runtime.GOOS {
	case "darwin":
		p := wamePlistPath()
		if _, err := os.Stat(p); err == nil {
			_ = exec.Command("launchctl", "unload", p).Run()
		}
		fmt.Println("🛑 wame service stopped")
		return nil
	case "linux":
		_ = exec.Command("systemctl", "--user", "stop", "wame.service").Run()
		fmt.Println("🛑 wame service stopped")
		return nil
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func wameServiceUninstall() error {
	_ = wameServiceStop()
	switch runtime.GOOS {
	case "darwin":
		_ = os.Remove(wamePlistPath())
	case "linux":
		_ = exec.Command("systemctl", "--user", "disable", "wame.service").Run()
		_ = os.Remove(wameSystemdUnitPath())
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	fmt.Println("🗑️  wame service uninstalled")
	return nil
}

func wameServiceStatus(port string) {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err == nil {
		_ = resp.Body.Close()
		fmt.Printf("🟢 wame daemon is running on http://localhost:%s/mcp\n", port)
	} else {
		fmt.Printf("⚪ wame daemon is not reachable on :%s (run `wame service start` or `brew services start wame`)\n", port)
	}
}
