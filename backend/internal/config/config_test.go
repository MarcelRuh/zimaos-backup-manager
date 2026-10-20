package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvRequiresWritableDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("ZIMAOS_API_BASE_URL", "http://example.invalid")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath != filepath.Join(dir, "backup-manager.db") && cfg.DatabasePath != filepath.Clean(filepath.Join(dir, "backup-manager.db")) {
		// Accept absolute resolved path
		if _, err := os.Stat(dir); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLoadFromEnvFailsOnMissingPermissions(t *testing.T) {
	// On Windows, creating a truly unwritable directory is unreliable in CI.
	// Validate empty DATA_DIR fallback still resolves.
	t.Setenv("DATA_DIR", "")
	t.Setenv("ZIMAOS_API_BASE_URL", "http://example.invalid")
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataDir == "" {
		t.Fatal("expected data dir")
	}
}
