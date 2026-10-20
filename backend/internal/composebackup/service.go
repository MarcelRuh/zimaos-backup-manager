package composebackup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/discord"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/dockerapi"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
)

const (
	DefaultCron       = "0 3 * * *"
	DefaultKeep       = 14 // unused for AppData sync; kept for settings compat
	maxFileBytes      = 8 << 20
	DefaultDockerSock = dockerapi.DefaultSock
)

var (
	ErrDisabled       = errors.New("compose backup disabled")
	ErrBusy           = errors.New("compose backup already running")
	ErrDockerMissing  = errors.New("docker socket not available")
	ErrAppDataMissing = errors.New("appdata directory not mounted or missing")
)

type ProjectResult struct {
	Name        string `json:"name"`
	AppDataName string `json:"appdata_name"`
	Target      string `json:"target,omitempty"`
	Bytes       int64  `json:"bytes,omitempty"`
	Error       string `json:"error,omitempty"`
	OK          bool   `json:"ok"`
}

type Result struct {
	OKCount     int             `json:"ok_count"`
	FailCount   int             `json:"fail_count"`
	SkipCount   int             `json:"skip_count"`
	Projects    []ProjectResult `json:"projects"`
	CreatedAt   time.Time       `json:"created_at"`
	TriggerTask string          `json:"trigger_task,omitempty"`
	TriggerName string          `json:"trigger_name,omitempty"`
}

type Service struct {
	dockerSock      string
	appDataDir      string
	appDataHostRoot string
	composeDir      string
	composeHostRoot string
	settings        *settings.Store
	audit           *store.AuditStore
	discord         *discord.Client
	logger          *slog.Logger

	mu      sync.Mutex
	running bool
	cron    *cron.Cron
}

type Paths struct {
	DockerSock      string
	AppDataDir      string
	AppDataHostRoot string
	ComposeDir      string
	ComposeHostRoot string
}

func New(paths Paths, settingsStore *settings.Store, audit *store.AuditStore, discordClient *discord.Client, logger *slog.Logger) *Service {
	if strings.TrimSpace(paths.DockerSock) == "" {
		paths.DockerSock = DefaultDockerSock
	}
	return &Service{
		dockerSock:      paths.DockerSock,
		appDataDir:      strings.TrimSpace(paths.AppDataDir),
		appDataHostRoot: strings.TrimSpace(paths.AppDataHostRoot),
		composeDir:      strings.TrimSpace(paths.ComposeDir),
		composeHostRoot: strings.TrimSpace(paths.ComposeHostRoot),
		settings:        settingsStore,
		audit:           audit,
		discord:         discordClient,
		logger:          logger,
	}
}

func (s *Service) dockerClient() *dockerapi.Client {
	return dockerapi.New(s.resolvedDockerSock())
}

func (s *Service) resolvedDockerSock() string {
	if v := strings.TrimSpace(s.settings.Get(settings.KeyComposeDockerSock)); v != "" {
		return v
	}
	return s.dockerSock
}

func (s *Service) resolvedAppDataDir() string {
	if v := strings.TrimSpace(s.settings.Get(settings.KeyComposeAppDataDir)); v != "" {
		return cleanSlash(v)
	}
	return cleanSlash(s.appDataDir)
}

func (s *Service) resolvedAppDataHostRoot() string {
	if v := strings.TrimSpace(s.settings.Get(settings.KeyComposeAppDataHostRoot)); v != "" {
		return cleanSlash(v)
	}
	return cleanSlash(s.appDataHostRoot)
}

func (s *Service) resolvedComposeDir() string {
	if v := strings.TrimSpace(s.settings.Get(settings.KeyComposeMountDir)); v != "" {
		return cleanSlash(v)
	}
	return cleanSlash(s.composeDir)
}

func (s *Service) resolvedComposeHostRoot() string {
	if v := strings.TrimSpace(s.settings.Get(settings.KeyComposeHostRoot)); v != "" {
		return cleanSlash(v)
	}
	return cleanSlash(s.composeHostRoot)
}

func (s *Service) Status() map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	docker := s.dockerClient()
	dockerErr := docker.Ping(ctx)
	dockerOK := dockerErr == nil
	appData := s.resolvedAppDataDir()
	var projects []Project
	if dockerOK {
		projects, _ = DiscoverProjects(ctx, docker, s.resolvedAppDataHostRoot(), appData, s.resolvedComposeHostRoot(), s.resolvedComposeDir())
	}
	var dockerErrMsg any
	if dockerErr != nil {
		dockerErrMsg = dockerErr.Error()
	}
	return map[string]any{
		"enabled":           s.settings.GetBool(settings.KeyComposeBackupEnabled),
		"docker_available":  dockerOK,
		"docker_error":      dockerErrMsg,
		"docker_sock":       s.resolvedDockerSock(),
		"appdata_dir":       appData,
		"appdata_available": dirExists(appData),
		"appdata_host_root": s.resolvedAppDataHostRoot(),
		"compose_dir":       s.resolvedComposeDir(),
		"compose_host_root": s.resolvedComposeHostRoot(),
		"projects":          projects,
		"project_count":     len(projects),
		"cron":              s.cronExpr(),
		"last_ok_at":        nullIfEmpty(s.settings.Get(settings.KeyComposeBackupLastOK)),
		"last_error":        nullIfEmpty(s.settings.Get(settings.KeyComposeBackupLastError)),
		"last_ok_count":     s.settings.GetInt(settings.KeyComposeBackupLastApps, 0),
		"last_fail_count":   s.settings.GetInt(settings.KeyComposeBackupLastFail, 0),
	}
}

func (s *Service) Start(ctx context.Context) error {
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
	// Optional cron still syncs compose into AppData; primary trigger is post-backup.
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
		if _, err := s.Run(context.Background(), "", ""); err != nil && !errors.Is(err, ErrBusy) && !errors.Is(err, ErrDisabled) {
			s.logger.Warn("scheduled compose→appdata sync failed", "error", err)
		}
	}); err != nil {
		return fmt.Errorf("compose backup cron %q: %w", expr, err)
	}
	s.cron = c
	c.Start()
	return nil
}

// RunAfterBackupSuccess syncs compose files into AppData and sends a Discord embed.
func (s *Service) RunAfterBackupSuccess(ctx context.Context, taskID, backupName string) {
	if !s.settings.GetBool(settings.KeyComposeBackupEnabled) {
		return
	}
	res, err := s.Run(ctx, taskID, backupName)
	s.notifyDiscord(ctx, res, err, taskID, backupName)
}

// Run discovers Docker Compose apps and copies compose files into their AppData folders.
func (s *Service) Run(ctx context.Context, triggerTaskID, triggerName string) (*Result, error) {
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

	appData := s.resolvedAppDataDir()
	if appData == "" || !dirExists(appData) {
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, ErrAppDataMissing.Error())
		return nil, ErrAppDataMissing
	}
	docker := s.dockerClient()
	if err := docker.Ping(ctx); err != nil {
		msg := fmt.Sprintf("%s: %v", ErrDockerMissing.Error(), err)
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, msg)
		s.logger.Warn("compose backup: docker unavailable", "sock", docker.Sock(), "error", err)
		return nil, fmt.Errorf("%w: %v", ErrDockerMissing, err)
	}

	projects, err := DiscoverProjects(ctx, docker, s.resolvedAppDataHostRoot(), appData, s.resolvedComposeHostRoot(), s.resolvedComposeDir())
	if err != nil {
		_ = s.settings.Set(ctx, settings.KeyComposeBackupLastError, err.Error())
		return nil, err
	}

	res := &Result{
		Projects:    make([]ProjectResult, 0, len(projects)),
		CreatedAt:   time.Now().UTC(),
		TriggerTask: triggerTaskID,
		TriggerName: triggerName,
	}

	for _, p := range projects {
		pr := ProjectResult{Name: p.Name, AppDataName: p.AppDataName}
		if p.ComposeLocal == "" || !fileReadable(p.ComposeLocal) {
			pr.Error = "compose source not readable"
			pr.OK = false
			res.FailCount++
			res.Projects = append(res.Projects, pr)
			continue
		}
		if p.AppDataLocal == "" {
			pr.Error = "appdata folder not mapped"
			pr.OK = false
			res.FailCount++
			res.Projects = append(res.Projects, pr)
			continue
		}
		dst, n, err := copyComposeIntoAppData(p.ComposeLocal, p.AppDataLocal)
		if err != nil {
			pr.Error = err.Error()
			pr.OK = false
			res.FailCount++
			res.Projects = append(res.Projects, pr)
			continue
		}
		pr.OK = true
		pr.Target = dst
		pr.Bytes = n
		res.OKCount++
		res.Projects = append(res.Projects, pr)
	}

	if res.OKCount == 0 && res.FailCount > 0 {
		errMsg := fmt.Sprintf("compose→appdata failed for all %d projects", res.FailCount)
		_ = s.settings.SetMany(ctx, map[string]string{
			settings.KeyComposeBackupLastError: errMsg,
			settings.KeyComposeBackupLastApps:  "0",
			settings.KeyComposeBackupLastFail:  strconv.Itoa(res.FailCount),
		})
		_ = s.audit.Add(ctx, "error", "compose_backup", errMsg, map[string]any{
			"fail": res.FailCount, "task_id": triggerTaskID,
		})
		return res, fmt.Errorf("%s", errMsg)
	}

	_ = s.settings.SetMany(ctx, map[string]string{
		settings.KeyComposeBackupLastOK:    res.CreatedAt.Format(time.RFC3339Nano),
		settings.KeyComposeBackupLastError: "",
		settings.KeyComposeBackupLastApps:  strconv.Itoa(res.OKCount),
		settings.KeyComposeBackupLastFail:  strconv.Itoa(res.FailCount),
		settings.KeyComposeBackupLastFile:  targetComposeName,
	})
	_ = s.audit.Add(ctx, "info", "compose_backup", "compose files synced into AppData", map[string]any{
		"ok": res.OKCount, "fail": res.FailCount, "task_id": triggerTaskID, "backup": triggerName,
	})
	return res, nil
}

func (s *Service) notifyDiscord(ctx context.Context, res *Result, runErr error, taskID, backupName string) {
	if !s.settings.GetBool(settings.KeyDiscordEnabled) {
		return
	}
	webhook := s.settings.Get(settings.KeyDiscordWebhookURL)
	if webhook == "" || s.discord == nil {
		return
	}
	loc := loadTZ(s.settings.Get(settings.KeyTimezone))
	now := time.Now().UTC()
	var embed discord.Embed
	if runErr != nil || (res != nil && res.FailCount > 0 && res.OKCount == 0) {
		if !s.settings.GetBool(settings.KeyDiscordNotifyFail) {
			return
		}
		msg := ""
		if runErr != nil {
			msg = runErr.Error()
		} else if res != nil {
			msg = fmt.Sprintf("%d Projekte fehlgeschlagen", res.FailCount)
		}
		ok, fail := 0, 0
		if res != nil {
			ok, fail = res.OKCount, res.FailCount
		}
		embed = discord.ComposeFailureEmbed(backupName, taskID, ok, fail, msg, now, loc)
	} else {
		if !s.settings.GetBool(settings.KeyDiscordNotifyOK) {
			return
		}
		ok, fail := 0, 0
		if res != nil {
			ok, fail = res.OKCount, res.FailCount
		}
		embed = discord.ComposeSuccessEmbed(backupName, taskID, ok, fail, now, loc)
	}
	if err := s.discord.SendEmbed(ctx, webhook, embed); err != nil {
		s.logger.Warn("compose discord notify failed", "error", err)
		_ = s.audit.Add(ctx, "error", "discord", "compose notification failed", map[string]any{"error": err.Error()})
		return
	}
	_ = s.audit.Add(ctx, "info", "discord", "compose notification sent", map[string]any{"task_id": taskID})
}

func (s *Service) cronExpr() string {
	v := strings.TrimSpace(s.settings.Get(settings.KeyComposeBackupCron))
	if v == "" {
		return DefaultCron
	}
	return v
}

func dirExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func loadTZ(name string) *time.Location {
	if name == "" {
		name = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}
