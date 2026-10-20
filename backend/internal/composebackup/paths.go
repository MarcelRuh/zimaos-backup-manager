package composebackup

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Known host/container locations for CasaOS/ZimaOS app compose files.
var defaultSourceCandidates = []string{
	"/host/casaos/apps",
	"/var/lib/casaos/apps",
	"/DATA/AppData/casaos/apps",
	"/media/casaos/apps",
	"/mnt/casaos/apps",
}

type PathCandidate struct {
	Path        string `json:"path"`
	Available   bool   `json:"available"`
	AppCount    int    `json:"app_count,omitempty"`
	Writable    bool   `json:"writable,omitempty"`
	Recommended bool   `json:"recommended,omitempty"`
	Label       string `json:"label,omitempty"`
}

func lookLikeAppsDir(dir string) (ok bool, appCount int) {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false, 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, 0
	}
	for _, ent := range entries {
		if !ent.IsDir() || !safeAppName(ent.Name()) {
			continue
		}
		appDir := filepath.Join(dir, ent.Name())
		if hasComposeFile(appDir) {
			appCount++
		}
	}
	return appCount > 0, appCount
}

func hasComposeFile(appDir string) bool {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if isComposeFilename(strings.ToLower(ent.Name())) {
			return true
		}
	}
	return false
}

func isWritableDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	test := filepath.Join(dir, ".zbm-write-test")
	if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}

func DetectSourceCandidates(envDefault string) []PathCandidate {
	seen := map[string]struct{}{}
	var out []PathCandidate
	add := func(path, label string) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		ok, n := lookLikeAppsDir(path)
		available := false
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			available = true
		}
		out = append(out, PathCandidate{
			Path:        path,
			Available:   available,
			AppCount:    n,
			Recommended: ok,
			Label:       label,
		})
	}

	if envDefault != "" {
		add(envDefault, "env")
	}
	for _, p := range defaultSourceCandidates {
		add(p, "known")
	}
	// Prefer directories that already contain compose files.
	sortSourceCandidates(out)
	return out
}

func sortSourceCandidates(items []PathCandidate) {
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			ai, aj := items[i], items[j]
			si := scoreSource(ai)
			sj := scoreSource(aj)
			if sj > si {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func scoreSource(c PathCandidate) int {
	score := 0
	if c.Available {
		score += 10
	}
	if c.Recommended {
		score += 100
	}
	score += c.AppCount * 5
	if c.Label == "env" {
		score += 2
	}
	return score
}

func ResolveSourceDir(configured, envDefault string) (path string, auto bool, candidates []PathCandidate) {
	candidates = DetectSourceCandidates(envDefault)
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return filepath.Clean(configured), false, candidates
	}
	for _, c := range candidates {
		if c.Recommended {
			return c.Path, true, candidates
		}
	}
	for _, c := range candidates {
		if c.Available {
			return c.Path, true, candidates
		}
	}
	fallback := strings.TrimSpace(envDefault)
	if fallback == "" {
		fallback = DefaultSourceDir
	}
	return filepath.Clean(fallback), true, candidates
}

func DefaultOutDir(dataDir string) string {
	return filepath.Join(dataDir, "casaos-apps-backups")
}

func DetectStorageCandidates(dataDir string) []PathCandidate {
	dataDir = filepath.Clean(dataDir)
	seen := map[string]struct{}{}
	var out []PathCandidate
	add := func(path, label string, recommended bool) {
		path = filepath.Clean(strings.TrimSpace(path))
		if path == "" || path == "." {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		available := false
		writable := false
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			available = true
			writable = isWritableDir(path)
		} else if recommended {
			// Default target is created on first backup; still offer it.
			parent := filepath.Dir(path)
			if isWritableDir(parent) || isWritableDir(dataDir) {
				available = true
				writable = true
			}
		}
		out = append(out, PathCandidate{
			Path:        path,
			Available:   available,
			Writable:    writable,
			Recommended: recommended && writable,
			Label:       label,
		})
	}

	add(DefaultOutDir(dataDir), "default", true)
	add(filepath.Join(dataDir, "backups", "casaos-apps"), "data", false)
	add(filepath.Join(dataDir, "backups"), "data", false)

	// Common extra mounts users bind into the container for large storage.
	for _, root := range []string{"/host/storage", "/host/backups", "/mnt/storage", "/mnt/backups", "/media/backups"} {
		add(filepath.Join(root, "casaos-apps-backups"), "mount", false)
		add(root, "mount", false)
	}
	if entries, err := os.ReadDir("/host"); err == nil {
		for _, ent := range entries {
			if !ent.IsDir() || !safeAppName(ent.Name()) {
				continue
			}
			name := strings.ToLower(ent.Name())
			if strings.Contains(name, "backup") || strings.Contains(name, "storage") || strings.Contains(name, "data") {
				base := filepath.Join("/host", ent.Name())
				add(filepath.Join(base, "casaos-apps-backups"), "mount", false)
				add(base, "mount", false)
			}
		}
	}
	return out
}

func ResolveOutDir(configured, dataDir string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return DefaultOutDir(dataDir), nil
	}
	path := filepath.Clean(configured)
	if err := ValidateOutDir(path, dataDir); err != nil {
		return "", err
	}
	return path, nil
}

func isAbsPath(path string) bool {
	return filepath.IsAbs(path) || strings.HasPrefix(path, "/")
}

func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "/") {
		// Keep Docker/Linux container paths stable on Windows hosts.
		return path.Clean(p)
	}
	return filepath.Clean(p)
}

func ValidateOutDir(path, dataDir string) error {
	path = cleanPath(path)
	if path == "" || path == "." || path == string(filepath.Separator) {
		return fmt.Errorf("invalid storage path")
	}
	if !isAbsPath(path) {
		return fmt.Errorf("storage path must be absolute")
	}
	dataDir = filepath.Clean(dataDir)
	if isUnder(path, dataDir) {
		return nil
	}
	for _, c := range DetectStorageCandidates(dataDir) {
		if filepath.Clean(c.Path) == path {
			return nil
		}
		if isUnder(path, c.Path) && c.Available {
			return nil
		}
	}
	// Allow any existing writable absolute path under /host, /mnt, /media, /data.
	for _, root := range []string{"/host", "/mnt", "/media", "/data"} {
		if isUnder(path, root) {
			parent := path
			for {
				if isWritableDir(parent) {
					return nil
				}
				next := filepath.Dir(parent)
				if next == parent {
					break
				}
				parent = next
				if !isUnder(parent, root) {
					break
				}
			}
		}
	}
	return fmt.Errorf("storage path not allowed; use /data or a mounted backup/storage path")
}

func ValidateSourceDir(path, envDefault string) error {
	path = cleanPath(path)
	if path == "" {
		return nil
	}
	if !isAbsPath(path) {
		return fmt.Errorf("source path must be absolute")
	}
	allowedRoots := []string{"/host", "/mnt", "/media", "/var/lib", "/DATA", "/data"}
	if envDefault != "" {
		allowedRoots = append(allowedRoots, filepath.Clean(envDefault))
		allowedRoots = append(allowedRoots, filepath.Dir(filepath.Clean(envDefault)))
	}
	for _, root := range allowedRoots {
		if isUnder(path, root) {
			return nil
		}
	}
	for _, c := range DetectSourceCandidates(envDefault) {
		if filepath.Clean(c.Path) == path {
			return nil
		}
	}
	return fmt.Errorf("source path not allowed")
}

func isUnder(path, root string) bool {
	path = cleanPath(path)
	root = cleanPath(root)
	if path == "" || root == "" {
		return false
	}
	if path == root {
		return true
	}
	sep := "/"
	if !strings.HasPrefix(root, "/") {
		sep = string(filepath.Separator)
	}
	prefix := root + sep
	return strings.HasPrefix(path, prefix)
}
