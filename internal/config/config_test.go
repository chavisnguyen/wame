package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsOriginAllowed(t *testing.T) {
	t.Setenv("WAME_ALLOWED_ORIGINS", "")

	if !IsOriginAllowed("") {
		t.Error("expected empty Origin to be allowed for CLI clients")
	}
	if IsOriginAllowed("https://evil.example.com") {
		t.Error("expected non-empty Origin to be rejected when WAME_ALLOWED_ORIGINS is empty")
	}

	t.Setenv("WAME_ALLOWED_ORIGINS", "https://app.local, http://localhost:3000")
	if !IsOriginAllowed("https://app.local") {
		t.Error("expected allowlisted Origin to be permitted")
	}
	if !IsOriginAllowed("http://localhost:3000") {
		t.Error("expected trimmed allowlisted Origin to be permitted")
	}
	if IsOriginAllowed("https://other.example.com") {
		t.Error("expected unlisted Origin to be rejected")
	}
}

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:3026", true},
		{"[::1]:3026", true},
		{"127.0.0.1", true},
		{"192.168.1.10:3026", false},
		{"0.0.0.0:3026", false},
		{"not-an-ip", false},
	}

	for _, tc := range cases {
		if got := IsLoopbackAddr(tc.addr); got != tc.want {
			t.Errorf("IsLoopbackAddr(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := `
# Comment line
WAME_TEST_PORT=4099
export WAME_TEST_QUOTED="hello world"
WAME_TEST_SINGLE='single-val'
WAME_TEST_EXISTING=from-file
`
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("WAME_TEST_EXISTING", "already-set")
	// Clear target keys after test finishes.
	t.Setenv("WAME_TEST_PORT", "")
	t.Setenv("WAME_TEST_QUOTED", "")
	t.Setenv("WAME_TEST_SINGLE", "")
	_ = os.Unsetenv("WAME_TEST_PORT")
	_ = os.Unsetenv("WAME_TEST_QUOTED")
	_ = os.Unsetenv("WAME_TEST_SINGLE")

	LoadDotEnv(envFile)

	if got := os.Getenv("WAME_TEST_PORT"); got != "4099" {
		t.Errorf("WAME_TEST_PORT = %q, want 4099", got)
	}
	if got := os.Getenv("WAME_TEST_QUOTED"); got != "hello world" {
		t.Errorf("WAME_TEST_QUOTED = %q, want 'hello world'", got)
	}
	if got := os.Getenv("WAME_TEST_SINGLE"); got != "single-val" {
		t.Errorf("WAME_TEST_SINGLE = %q, want 'single-val'", got)
	}
	if got := os.Getenv("WAME_TEST_EXISTING"); got != "already-set" {
		t.Errorf("existing env var should not be overwritten, got %q", got)
	}
}

func TestMigrateLegacyStoreIfNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	legacy := filepath.Join(tmpDir, "legacy-store")
	target := filepath.Join(tmpDir, "home", ".wame", "store")

	if err := os.MkdirAll(filepath.Join(legacy, "media-chat"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "whatsapp.db"), []byte("wa-session"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "messages.db"), []byte("msg-db"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "media-chat", "pic.jpg"), []byte("img"), 0644); err != nil {
		t.Fatal(err)
	}

	migratedFrom, err := MigrateLegacyStoreIfNeeded(target, legacy)
	if err != nil {
		t.Fatalf("MigrateLegacyStoreIfNeeded error: %v", err)
	}
	if migratedFrom != legacy {
		t.Fatalf("migratedFrom = %q, want %q", migratedFrom, legacy)
	}

	for _, rel := range []string{"whatsapp.db", "messages.db", filepath.Join("media-chat", "pic.jpg")} {
		if _, err := os.Stat(filepath.Join(target, rel)); err != nil {
			t.Errorf("expected migrated file %s: %v", rel, err)
		}
	}

	// Second call must be a no-op since target already has DB files.
	second, err := MigrateLegacyStoreIfNeeded(target, legacy)
	if err != nil {
		t.Fatalf("second migration check error: %v", err)
	}
	if second != "" {
		t.Errorf("expected no migration on second run, got %q", second)
	}
}

func TestResolveStoreDirPrecedence(t *testing.T) {
	t.Setenv("WAME_STORE_DIR", "/tmp/env-store")

	got, _, err := ResolveStoreDir("/tmp/flag-store")
	if err != nil || got != "/tmp/flag-store" {
		t.Fatalf("flag should win over env, got %q (err=%v)", got, err)
	}

	got, _, err = ResolveStoreDir("")
	if err != nil || got != "/tmp/env-store" {
		t.Fatalf("env should win over default, got %q (err=%v)", got, err)
	}
}
