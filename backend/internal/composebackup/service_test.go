package composebackup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/db"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
)

func TestRunArchivesComposeFiles(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "apps")
	data := filepath.Join(root, "data")
	_ = os.MkdirAll(data, 0o755)
	_ = os.MkdirAll(filepath.Join(src, "jellyfin"), 0o755)
	_ = os.MkdirAll(filepath.Join(src, "portainer"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "jellyfin", "docker-compose.yml"), []byte("services:\n  jellyfin: {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "portainer", "compose.yaml"), []byte("services:\n  portainer: {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "jellyfin", "notes.txt"), []byte("skip"), 0o644)

	database, err := db.Open(filepath.Join(data, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	cfg := config.Config{DataDir: data, Timezone: "UTC"}
	st, err := settings.NewStore(database.SQL, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Set(context.Background(), settings.KeyComposeBackupEnabled, "true")
	svc := New(src, data, st, store.NewAuditStore(database.SQL), slog.Default())

	res, err := svc.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.AppCount != 2 || res.FileCount != 2 {
		t.Fatalf("apps=%d files=%d", res.AppCount, res.FileCount)
	}
	names := readTarNames(t, res.ArchivePath)
	if len(names) != 2 {
		t.Fatalf("tar entries: %v", names)
	}
}

func TestRunRequiresMount(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	_ = os.MkdirAll(data, 0o755)
	database, err := db.Open(filepath.Join(data, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	st, err := settings.NewStore(database.SQL, config.Config{DataDir: data, Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Set(context.Background(), settings.KeyComposeBackupEnabled, "true")
	svc := New(filepath.Join(root, "missing"), data, st, store.NewAuditStore(database.SQL), slog.Default())
	if _, err := svc.Run(context.Background()); err != ErrSourceMissing {
		t.Fatalf("got %v", err)
	}
}

func readTarNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, hdr.Name)
		_, _ = io.Copy(io.Discard, tr)
	}
	return names
}
