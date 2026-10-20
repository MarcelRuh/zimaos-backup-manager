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
	_ = st.Set(context.Background(), settings.KeyComposeBackupSourceDir, filepath.Join(root, "missing"))
	svc := New(filepath.Join(root, "missing"), data, st, store.NewAuditStore(database.SQL), slog.Default())
	if _, err := svc.Run(context.Background()); err != ErrSourceMissing {
		t.Fatalf("got %v", err)
	}
}

func TestResolveSourceDirAutoPicksComposeApps(t *testing.T) {
	root := t.TempDir()
	apps := filepath.Join(root, "apps")
	_ = os.MkdirAll(filepath.Join(apps, "app1"), 0o755)
	_ = os.WriteFile(filepath.Join(apps, "app1", "docker-compose.yml"), []byte("services: {}\n"), 0o644)
	empty := filepath.Join(root, "empty")
	_ = os.MkdirAll(empty, 0o755)

	path, auto, _ := ResolveSourceDir("", apps)
	if !auto || path != filepath.Clean(apps) {
		t.Fatalf("auto path=%q auto=%v", path, auto)
	}
	forced := filepath.Join(root, "forced")
	_ = os.MkdirAll(forced, 0o755)
	path, auto, _ = ResolveSourceDir(forced, apps)
	if auto || path != filepath.Clean(forced) {
		t.Fatalf("forced path=%q auto=%v", path, auto)
	}
}

func TestResolveOutDirDefaultAndCustom(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	_ = os.MkdirAll(data, 0o755)

	got, err := ResolveOutDir("", data)
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultOutDir(data) {
		t.Fatalf("default=%q", got)
	}
	custom := filepath.Join(data, "my-backups")
	got, err = ResolveOutDir(custom, data)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(custom) {
		t.Fatalf("custom=%q", got)
	}
	if err := ValidateOutDir(filepath.Join(root, "outside"), data); err == nil {
		t.Fatal("expected outside path rejected")
	}
}

func TestRunUsesConfiguredOutDir(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "apps")
	data := filepath.Join(root, "data")
	out := filepath.Join(data, "custom-out")
	_ = os.MkdirAll(data, 0o755)
	_ = os.MkdirAll(filepath.Join(src, "jellyfin"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "jellyfin", "docker-compose.yml"), []byte("services:\n  jellyfin: {}\n"), 0o644)

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
	_ = st.Set(context.Background(), settings.KeyComposeBackupSourceDir, src)
	_ = st.Set(context.Background(), settings.KeyComposeBackupOutDir, out)
	svc := New(src, data, st, store.NewAuditStore(database.SQL), slog.Default())

	res, err := svc.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(res.ArchivePath) != filepath.Clean(out) {
		t.Fatalf("archive in %q want under %q", res.ArchivePath, out)
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
