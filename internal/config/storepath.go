package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// HomeDir returns ~/.wame as the user-scoped config/data directory.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".wame"
	}
	return filepath.Join(home, ".wame")
}

// DefaultStoreDir returns the default persistent store directory (~/.wame/store).
func DefaultStoreDir() string {
	return filepath.Join(HomeDir(), "store")
}

// LegacyStoreCandidates returns legacy relative store directories from pre-rename installs.
func LegacyStoreCandidates() []string {
	candidates := []string{"store"}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		candidates = append(candidates,
			filepath.Join(filepath.Dir(exe), "..", "store"),
			filepath.Join(filepath.Dir(exe), "store"),
		)
	}
	return candidates
}

// ResolveStoreDir determines the store directory from explicit flag, WAME_STORE_DIR env var,
// or DefaultStoreDir(). When using DefaultStoreDir(), it migrates legacy data if needed.
// Returns the resolved directory and any migration source path (empty if none occurred).
func ResolveStoreDir(explicitFlag string) (string, string, error) {
	if strings.TrimSpace(explicitFlag) != "" {
		return explicitFlag, "", nil
	}
	if envDir := strings.TrimSpace(os.Getenv("WAME_STORE_DIR")); envDir != "" {
		return envDir, "", nil
	}

	target := DefaultStoreDir()
	migratedFrom, err := MigrateLegacyStoreIfNeeded(target, LegacyStoreCandidates()...)
	if err != nil {
		return target, "", err
	}
	return target, migratedFrom, nil
}

// hasStoreDB checks whether dir already holds WhatsApp session or message SQLite data.
func hasStoreDB(dir string) bool {
	for _, name := range []string{"whatsapp.db", "messages.db"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

// MigrateLegacyStoreIfNeeded copies data from the first populated legacy store directory
// into targetDir when targetDir does not yet contain a WhatsApp or message database.
func MigrateLegacyStoreIfNeeded(targetDir string, legacyDirs ...string) (string, error) {
	if hasStoreDB(targetDir) {
		return "", nil
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		absTarget = targetDir
	}

	for _, cand := range legacyDirs {
		if cand == "" {
			continue
		}
		absCand, err := filepath.Abs(cand)
		if err != nil {
			absCand = cand
		}
		if absCand == absTarget {
			continue
		}
		if !hasStoreDB(cand) {
			continue
		}
		if err := copyStoreTree(cand, targetDir); err != nil {
			return "", fmt.Errorf("migrate store from %s to %s: %w", cand, targetDir, err)
		}
		return cand, nil
	}
	return "", nil
}

func copyStoreTree(src, dst string) error {
	if err := os.MkdirAll(dst, 0700); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
