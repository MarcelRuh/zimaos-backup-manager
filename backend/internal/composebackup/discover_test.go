package composebackup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/dockerapi"
)

type fakeDocker struct {
	containers []dockerapi.Container
	err        error
}

func (f fakeDocker) ListContainers(ctx context.Context) ([]dockerapi.Container, error) {
	_ = ctx
	return f.containers, f.err
}

func TestDiscoverProjectsMapsAppData(t *testing.T) {
	root := t.TempDir()
	composeHost := "/var/lib/casaos/apps/jellyfin/docker-compose.yml"
	composeLocalDir := filepath.Join(root, "casaos", "jellyfin")
	_ = os.MkdirAll(composeLocalDir, 0o755)
	composeLocal := filepath.Join(composeLocalDir, "docker-compose.yml")
	_ = os.WriteFile(composeLocal, []byte("services: {}\n"), 0o644)

	appDataLocal := filepath.Join(root, "appdata")
	_ = os.MkdirAll(filepath.Join(appDataLocal, "jellyfin"), 0o755)

	docker := fakeDocker{containers: []dockerapi.Container{{
		ID:    "abc",
		Names: []string{"/jellyfin"},
		Labels: map[string]string{
			"com.docker.compose.project":              "jellyfin",
			"com.docker.compose.project.config_files": composeHost,
		},
		Mounts: []dockerapi.Mount{{
			Type:   "bind",
			Source: "/DATA/AppData/jellyfin/config",
		}},
	}}}

	projects, err := DiscoverProjects(context.Background(), docker,
		"/DATA/AppData", filepath.ToSlash(appDataLocal),
		"/var/lib/casaos/apps", filepath.ToSlash(filepath.Join(root, "casaos")),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects=%d", len(projects))
	}
	p := projects[0]
	if p.AppDataName != "jellyfin" {
		t.Fatalf("appdata=%q", p.AppDataName)
	}
	if p.Name != "jellyfin" {
		t.Fatalf("name=%q", p.Name)
	}
	if !p.ComposeReadable {
		t.Fatalf("compose not readable: %s", p.ComposeLocal)
	}
}

func TestCopyComposeIntoAppData(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.yml")
	_ = os.WriteFile(src, []byte("services:\n  a: {}\n"), 0o644)
	dstDir := filepath.Join(root, "AppData", "myapp")
	path, n, err := copyComposeIntoAppData(src, dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Fatal("bytes")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "services:\n  a: {}\n" {
		t.Fatalf("content=%q", got)
	}
}

func TestAppDataNameFromMounts(t *testing.T) {
	name := appDataNameFromMounts([]dockerapi.Mount{
		{Source: "/DATA/AppData/portainer/data"},
	}, "/DATA/AppData")
	if name != "portainer" {
		t.Fatalf("got %q", name)
	}
}

func TestResolveComposeLocalViaAppData(t *testing.T) {
	root := t.TempDir()
	appDataLocal := filepath.Join(root, "appdata", "plex")
	_ = os.MkdirAll(appDataLocal, 0o755)
	composeInAppData := filepath.Join(appDataLocal, "docker-compose.yml")
	_ = os.WriteFile(composeInAppData, []byte("services: {}\n"), 0o644)

	// Label points at host AppData path; CasaOS apps mount is empty — resolve via AppData.
	got := resolveComposeLocal(
		"/DATA/AppData/plex/docker-compose.yml",
		"plex", "plex",
		"/var/lib/casaos/apps", filepath.Join(root, "casaos"),
		"/DATA/AppData", filepath.Join(root, "appdata"),
	)
	if got != filepath.ToSlash(composeInAppData) && cleanSlash(got) != cleanSlash(composeInAppData) {
		t.Fatalf("got %q want %q", got, composeInAppData)
	}
}

func TestResolveComposeLocalFallbackByName(t *testing.T) {
	root := t.TempDir()
	composeDir := filepath.Join(root, "casaos", "plex")
	_ = os.MkdirAll(composeDir, 0o755)
	composeFile := filepath.Join(composeDir, "docker-compose.yml")
	_ = os.WriteFile(composeFile, []byte("services: {}\n"), 0o644)

	// Label path does not match configured roots; fall back to name under compose mount.
	got := resolveComposeLocal(
		"/some/other/path/docker-compose.yml",
		"plex", "plex",
		"/var/lib/casaos/apps", filepath.Join(root, "casaos"),
		"/DATA/AppData", filepath.Join(root, "appdata"),
	)
	if cleanSlash(got) != cleanSlash(composeFile) {
		t.Fatalf("got %q want %q", got, composeFile)
	}
}
