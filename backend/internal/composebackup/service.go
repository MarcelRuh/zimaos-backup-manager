package composebackup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
)

const (
	DefaultSourceDir = "/host/casaos/apps"
	DefaultCron      = "0 3 * * *"
	DefaultKeep      = 14
	archivePrefix    = "casaos-apps-"
	maxFileBytes     = 8 << 20
)

var (
	ErrSourceMissing = errors.New("casaos apps directory not mounted or missing")
	ErrDisabled      = errors.New("compose backup disabled")
	ErrBusy          = errors.New("compose backup already running")
)

type Result struct {
	ArchivePath string
	AppCount    int
	FileCount   int
	Bytes       int64
	CreatedAt   time.Time
}

type Service struct {
	envSourceDir string
	dataDir      string
	settings     *settings.Store
	audit        *store.AuditStore
	logger       *slog.Logger

	mu      sync.Mutex
	running bool
	cron    *cron.Cron

	candMu        sync.Mutex
	outCands      []PathCandidate
	outCandsAt    time.Time
	sourceCands   []PathCandidate
	sourceCandsAt time.Time
}

func New(sourceDir, dataDir string, settingsStore *settings.Store, audit *store.AuditStore, logger *slog.Logger) *Service {
	if strings.TrimSpace(sourceDir) == "" {
		sourceDir = DefaultSourceDir
	}
	return &Service{
		envSourceDir: filepath.Clean(sourceDir),
		dataDir:      filepath.Clean(dataDir),
		settings:     settingsStore,
		audit:        audit,
		logger:       logger,
	}
}

func (s *Service) SourceDir() string {
	path, _, _ := ResolveSourceDir(s.settings.Get(settings.KeyComposeBackupSourceDir), s.envSourceDir)
	return path
}

func (s *Service) OutDir() string {
	path, err := ResolveOutDir(s.settings.Get(settings.KeyComposeBackupOutDir), s.dataDir)
	if err != nil {
		return DefaultOutDir(s.dataDir)
	}
	return path
}

func (s *Service) Status() map[string]any {
	sourceDir, sourceAuto, _ := ResolveSourceDir(s.settings.Get(settings.KeyComposeBackupSourceDir), s.envSourceDir)
	outDir, outErr := ResolveOutDir(s.settings.Get(settings.KeyComposeBackupOutDir), s.dataDir)
	if outErr != nil {
		outDir = DefaultOutDir(s.dataDir)
	}
	return map[string]any{
		"enabled":           s.settings.GetBool(settings.KeyComposeBackupEnabled),
		"source_dir":        sourceDir,
		"source_configured": strings.TrimSpace(s.settings.Get(settings.KeyComposeBackupSourceDir)),
		"source_auto":       sourceAuto,
		"source_available":  dirExists(sourceDir),
		"source_candidates": s.cachedSourceCandidates(),
		"out_dir":           outDir,
		"out_configured":    strings.TrimSpace(s.settings.Get(settings.KeyComposeBackupOutDir)),
		"out_candidates":    s.cachedOutCandidates(),
		"cron":              s.cronExpr(),
		"keep":              s.keepCount(),
		"last_ok_at":        nullIfEmpty(s.settings.Get(settings.KeyComposeBackupLastOK)),
		"last_error":        nullIfEmpty(s.settings.Get(settings.KeyComposeBackupLastError)),
		"last_archive":      nullIfEmpty(s.settings.Get(settings.KeyComposeBackupLastFile)),
		"last_app_count":    s.settings.GetInt(settings.KeyComposeBackupLastApps, 0),
		"archives":          s.listArchives(outDir, 8),
	}
}

func (s *Service) cachedSourceCandidates() []PathCandidate {
	const ttl = 30 * time.Second
	s.candMu.Lock()
	defer s.candMu.Unlock()
	if time.Since(s.sourceCandsAt) < ttl && s.sourceCands != nil {
		return s.sourceCands
	}
	s.sourceCands = DetectSourceCandidates(s.envSourceDir)
	s.sourceCandsAt = time.Now()
	return s.sourceCands
}

func (s *Service) cachedOutCandidates() []PathCandidate {
	const ttl = 30 * time.Second
	s.candMu.Lock()
	defer s.candMu.Unlock()
	if time.Since(s.outCandsAt) < ttl && s.outCands != nil {
		return s.outCands
	}
	s.outCands = DetectStorageCandidates(s.dataDir)
	s.outCandsAt = time.Now()
	return s.outCands
}

func (s *Service) InvalidatePathCache() {
	s.candMu.Lock()
	defer s.candMu.Unlock()
	s.sourceCands = nil
	s.outCands = nil
	s.sourceCandsAt = time.Time{}
	s.outCandsAt = time.Time{}
}

func (s *Service) Start(ctx context.Context) error {
	_ = os.MkdirAll(s.OutDir(), 0o750)
	return s.Reload(ctx)
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
}

func (s *Service) Reload(ctx context.Context) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
	if !s.settings.GetBool(settings.KeyComposeBackupEnabled) {
		return nil
	}
	tzName := s.settings.Get(settings.KeyTimezone)
	if tzName == "" {
		tzName = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.Local
	}
	c := cron.New(cron.WithLocation(loc), cron.WithParser(cron.NewParser(
		cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
	)))
	expr := s.cronExpr()
	if _, err := c.AddFunc(expr, func() {
		if _, err := s.Run(context.Background()); err != nil && !errors.Is(err, ErrBusy) {
			s.logger.Warn("scheduled compose backup failed", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("compose backup cron %q: %w", expr, err)
	}
	s.cron = c
	c.Start()
	return nil
}

func (s *Service) Run(ctx context.Context) (*Result, error) {
	if !s.settings.GetBool(settings.KeyComposeBackupEnabled) {
		return nil, ErrDisabled
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	sourceDir := s.SourceDir()
	if !dirExists(sourceDir) {
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, ErrSourceMissing.Error())
		return nil, ErrSourceMissing
	}
	outDir, err := ResolveOutDir(s.settings.Get(settings.KeyComposeBackupOutDir), s.dataDir)
	if err != nil {
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, err.Error())
		return nil, err
	}

	res, err := s.createArchive(ctx, sourceDir, outDir)
	if err != nil {
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, err.Error())
		_ = s.audit.Add(ctx, "error", "compose_backup", "casaos apps backup failed", map[string]any{"error": err.Error()})
		return nil, err
	}
	_ = s.settings.SetMany(ctx, map[string]string{
		settings.KeyComposeBackupLastOK:    res.CreatedAt.UTC().Format(time.RFC3339Nano),
		settings.KeyComposeBackupLastError: "",
		settings.KeyComposeBackupLastFile:  filepath.Base(res.ArchivePath),
		settings.KeyComposeBackupLastApps:  strconv.Itoa(res.AppCount),
	})
	_ = s.prune(outDir, s.keepCount())
	_ = s.audit.Add(ctx, "info", "compose_backup", "casaos apps backup created", map[string]any{
		"archive": filepath.Base(res.ArchivePath),
		"apps":    res.AppCount,
		"files":   res.FileCount,
		"bytes":   res.Bytes,
		"out_dir": outDir,
	})
	return res, nil
}

func (s *Service) createArchive(ctx context.Context, sourceDir, outDir string) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	outPath := filepath.Join(outDir, fmt.Sprintf("%s%s.tar.gz", archivePrefix, now.Format("20060102-150405")))
	tmpPath := outPath + ".partial"

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	appCount := 0
	fileCount := 0

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		cleanupWriters(tw, gz, f, tmpPath)
		return nil, err
	}
	for _, ent := range entries {
		if err := ctx.Err(); err != nil {
			cleanupWriters(tw, gz, f, tmpPath)
			return nil, err
		}
		if !ent.IsDir() {
			continue
		}
		appName := ent.Name()
		if !safeAppName(appName) {
			continue
		}
		added, err := addAppComposeFiles(tw, filepath.Join(sourceDir, appName), appName)
		if err != nil {
			cleanupWriters(tw, gz, f, tmpPath)
			return nil, err
		}
		if added > 0 {
			appCount++
			fileCount += added
		}
	}

	if err := tw.Close(); err != nil {
		cleanupWriters(nil, gz, f, tmpPath)
		return nil, err
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return nil, err
	}
	info, _ := os.Stat(outPath)
	var bytes int64
	if info != nil {
		bytes = info.Size()
	}
	return &Result{
		ArchivePath: outPath,
		AppCount:    appCount,
		FileCount:   fileCount,
		Bytes:       bytes,
		CreatedAt:   now,
	}, nil
}

func addAppComposeFiles(tw *tar.Writer, appDir, appName string) (int, error) {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		if !isComposeFilename(strings.ToLower(ent.Name())) {
			continue
		}
		path := filepath.Join(appDir, ent.Name())
		info, err := ent.Info()
		if err != nil {
			return added, err
		}
		if info.Size() > maxFileBytes {
			continue
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return added, err
		}
		hdr.Name = filepath.ToSlash(filepath.Join(appName, ent.Name()))
		hdr.Uid = 0
		hdr.Gid = 0
		if err := tw.WriteHeader(hdr); err != nil {
			return added, err
		}
		f, err := os.Open(path)
		if err != nil {
			return added, err
		}
		_, copyErr := io.Copy(tw, io.LimitReader(f, maxFileBytes))
		_ = f.Close()
		if copyErr != nil {
			return added, copyErr
		}
		added++
	}
	return added, nil
}

func isComposeFilename(name string) bool {
	switch name {
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return true
	default:
		return false
	}
}

func safeAppName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	return true
}

func cleanupWriters(tw *tar.Writer, gz *gzip.Writer, f *os.File, tmpPath string) {
	if tw != nil {
		_ = tw.Close()
	}
	if gz != nil {
		_ = gz.Close()
	}
	if f != nil {
		_ = f.Close()
	}
	_ = os.Remove(tmpPath)
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func (s *Service) cronExpr() string {
	v := strings.TrimSpace(s.settings.Get(settings.KeyComposeBackupCron))
	if v == "" {
		return DefaultCron
	}
	return v
}

func (s *Service) keepCount() int {
	n := s.settings.GetInt(settings.KeyComposeBackupKeep, DefaultKeep)
	if n < 1 {
		return 1
	}
	if n > 90 {
		return 90
	}
	return n
}

func (s *Service) prune(outDir string, keep int) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, archivePrefix) && strings.HasSuffix(n, ".tar.gz") {
			names = append(names, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for i := keep; i < len(names); i++ {
		_ = os.Remove(filepath.Join(outDir, names[i]))
	}
	return nil
}

func (s *Service) listArchives(outDir string, limit int) []map[string]any {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, archivePrefix) && strings.HasSuffix(n, ".tar.gz") {
			names = append(names, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if limit > 0 && len(names) > limit {
		names = names[:limit]
	}
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		info, err := os.Stat(filepath.Join(outDir, n))
		item := map[string]any{"name": n}
		if err == nil {
			item["bytes"] = info.Size()
			item["modified_at"] = info.ModTime().UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
