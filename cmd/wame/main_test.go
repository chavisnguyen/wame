package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wame/internal/mcpserver"
	"wame/internal/store"
)

func TestSetupHTTPHandlerSecurityAndHealth(t *testing.T) {
	msgStore, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore error: %v", err)
	}
	defer msgStore.Close()

	srv := mcpserver.NewServer(nil, msgStore, Version)
	handler := setupHTTPHandler(srv, nil, msgStore)

	// 1. /health without Origin succeeds and returns JSON stats.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", rec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid /health JSON: %v", err)
	}
	if payload["status"] != "ok" || payload["version"] != Version {
		t.Errorf("unexpected /health payload: %v", payload)
	}

	// 2. Disallowed Origin is rejected with 403 on /mcp, /sse, /health.
	t.Setenv("WAME_ALLOWED_ORIGINS", "")
	for _, path := range []string{"/mcp", "/sse", "/health"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Origin", "https://evil.example.com")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s with disallowed Origin returned %d, want 403", path, rec.Code)
		}
	}

	// 3. Allowed Origin passes middleware.
	t.Setenv("WAME_ALLOWED_ORIGINS", "https://trusted.local")
	req = httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://trusted.local")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("/health with allowlisted Origin returned %d, want 200", rec.Code)
	}

	// 4. notifications/cancelled returns 200 immediately.
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/cancelled"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("notifications/cancelled returned %d, want 200", rec.Code)
	}

	// 5. /.well-known/ returns 404.
	req = httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("/.well-known/ returned %d, want 404", rec.Code)
	}
}
