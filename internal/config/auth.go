package config

import (
	"net"
	"os"
	"strings"
)

// IsLoopbackAddr checks if a remote address (e.g. "127.0.0.1:12345" or "[::1]:12345") is loopback.
func IsLoopbackAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// IsOriginAllowed checks whether the HTTP Origin header is permitted.
// Empty Origin is allowed because CLI/desktop MCP clients do not send Origin.
// Otherwise, the Origin must match an entry in WAME_ALLOWED_ORIGINS (comma-separated).
func IsOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	allowedEnv := os.Getenv("WAME_ALLOWED_ORIGINS")
	if allowedEnv == "" {
		return false
	}
	for _, p := range strings.Split(allowedEnv, ",") {
		if strings.TrimSpace(p) == origin {
			return true
		}
	}
	return false
}
