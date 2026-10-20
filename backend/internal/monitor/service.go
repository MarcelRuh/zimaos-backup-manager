package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/discord"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/events"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

// ComposeAfterSuccess syncs Docker Compose files into AppData after a successful ZimaOS backup.
type ComposeAfterSuccess interface {
	RunAfterBackupSuccess(ctx context.Context, taskID, backupName string)
}

type Service struct {
	client   *zimaos.Client
	runs     *store.RunStore
	prefs    *store.PrefsStore
	audit    *store.AuditStore
	settings *settings.Store
	discord  *discord.Client
	compose  ComposeAfterSuccess
	hub      *events.Hub
	logger   *slog.Logger

	mu       sync.Mutex
	taskLock map[string]struct{}
	stopCh   chan struct{}
	wg       sync.WaitGroup

	cacheMu sync.RWMutex
	tasks   []zimaos.Task
}

func NewService(
	client *zimaos.Client,
	runs *store.RunStore,
	prefs *store.PrefsStore,
	audit *store.AuditStore,
	settingsStore *settings.Store,
	discordClient *discord.Client,
	hub *events.Hub,
	logger *slog.Logger,
	compose ComposeAfterSuccess,
) *Service {
	return &Service{
		client:   client,
		runs:     runs,
		prefs:    prefs,
		audit:    audit,
		settings: settingsStore,
		discord:  discordClient,
		compose:  compose,
		hub:      hub,
		logger:   logger,
		taskLock: map[string]struct{}{},
		stopCh:   make(chan struct{}),
	}
}

func (s *Service) Start() {
	s.wg.Add(1)
	go s.loop()
}

func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Service) CachedTasks() []zimaos.Task {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	out := make([]zimaos.Task, len(s.tasks))
	copy(out, s.tasks)
	return out
}

func (s *Service) TryLockTask(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.taskLock[taskID]; ok {
		return false
	}
	s.taskLock[taskID] = struct{}{}
	return true
}

func (s *Service) UnlockTask(taskID string) {
	s.mu.Lock()
	delete(s.taskLock, taskID)
	s.mu.Unlock()
}

func (s *Service) IsTaskLocked(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.taskLock[taskID]
	return ok
}

func (s *Service) loop() {
	defer s.wg.Done()
	for {
		interval := time.Duration(s.settings.GetInt(settings.KeyPollIntervalSec, 5)) * time.Second
		if interval < 2*time.Second {
			interval = 2 * time.Second
		}
		s.tick()
		select {
		case <-s.stopCh:
			return
		case <-time.After(interval):
		}
	}
}

func (s *Service) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	base := s.settings.Get(settings.KeyZimaOSAPIBaseURL)
	token := s.settings.Get(settings.KeyZimaOSAPIToken)
	username := s.settings.Get(settings.KeyZimaOSUsername)
	password := s.settings.Get(settings.KeyZimaOSPassword)
	s.client.Configure(base, token, username, password)

	tasks, err := s.client.ListTasks(ctx)
	if err != nil {
		s.logger.Warn("zimaos poll failed", "error", err)
		s.hub.Publish("api_status", map[string]any{"connected": false, "error": err.Error()})
		s.markConnectionLost(ctx)
		return
	}

	s.cacheMu.Lock()
	s.tasks = tasks
	s.cacheMu.Unlock()
	s.hub.Publish("tasks", tasks)
	s.hub.Publish("api_status", map[string]any{"connected": true})

	seen := map[string]struct{}{}
	for _, t := range tasks {
		seen[t.ID] = struct{}{}
		s.observeTask(ctx, t)
	}

	// Recover open runs whose task vanished from the API response.
	open, err := s.runs.ListOpen(ctx)
	if err != nil {
		return
	}
	for _, run := range open {
		if _, ok := seen[run.TaskID]; ok {
			continue
		}
		run.Result = ResultAmbiguous
		run.ErrorMessage = "Task nicht mehr in der ZimaOS-API gefunden; Abschluss unklar."
		end := time.Now().UTC()
		run.EndedAt = &end
		dur := end.Sub(run.StartedAt).Milliseconds()
		run.DurationMS = &dur
		run.DurationQuality = DurationEstimated
		_ = s.runs.Update(ctx, &run)
		s.hub.Publish("run", run)
	}
}

func (s *Service) markConnectionLost(ctx context.Context) {
	open, err := s.runs.ListOpen(ctx)
	if err != nil {
		return
	}
	for i := range open {
		if open[i].Result == ResultConnectionLost {
			continue
		}
		open[i].Result = ResultConnectionLost
		_ = s.runs.Update(ctx, &open[i])
		s.hub.Publish("run", open[i])
	}
}

func (s *Service) observeTask(ctx context.Context, task zimaos.Task) {
	open, err := s.runs.GetOpenByTask(ctx, task.ID)
	if err == nil && open != nil {
		s.advanceRun(ctx, open, task, true)
		return
	}
	if task.EffectiveRunning() {
		// Externally started backup
		_, _ = s.BeginObservedRun(ctx, task)
	}
}

func (s *Service) BeginObservedRun(ctx context.Context, task zimaos.Task) (*store.BackupRun, error) {
	now := time.Now().UTC()
	totalFiles, xferredFiles := task.AggregateFiles()
	totalSize, xferredSize := task.AggregateSize()
	run := &store.BackupRun{
		TaskID:             task.ID,
		BackupName:         task.Name,
		Source:             task.Source,
		Target:             task.PrimaryTarget(),
		StartedAt:          now,
		DurationQuality:    DurationEstimated,
		Result:             ResultRunning,
		TotalFiles:         totalFiles,
		XferredFiles:       xferredFiles,
		TotalSize:          totalSize,
		XferredSize:        xferredSize,
		DiscordStatus:      "none",
		Fingerprint:        store.MakeFingerprint(task.ID, now, task.LastBackupTime),
		BaselineLastBackup: task.LastBackupTime,
		SawRunning:         task.EffectiveRunning(),
	}
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, err
	}
	_ = s.audit.Add(ctx, "info", "backup", "backup run observed/started", map[string]any{
		"task_id": task.ID, "name": task.Name, "run_id": run.ID,
	})
	s.hub.Publish("run", run)
	return run, nil
}

func (s *Service) BeginManagedRun(ctx context.Context, task zimaos.Task) (*store.BackupRun, error) {
	return s.BeginObservedRun(ctx, task)
}

func (s *Service) advanceRun(ctx context.Context, run *store.BackupRun, task zimaos.Task, apiOK bool) {
	if task.EffectiveRunning() {
		run.SawRunning = true
	}
	totalFiles, xferredFiles := task.AggregateFiles()
	totalSize, xferredSize := task.AggregateSize()
	run.BackupName = task.Name
	run.Source = task.Source
	run.Target = task.PrimaryTarget()
	run.TotalFiles = totalFiles
	run.XferredFiles = xferredFiles
	run.TotalSize = totalSize
	run.XferredSize = xferredSize
	if errMsg := task.CombinedError(); errMsg != "" {
		run.ErrorMessage = errMsg
	}

	obs := Observation{
		Task:               task,
		APIReachable:       apiOK,
		ObservedAtUnix:     time.Now().Unix(),
		BaselineLastBackup: run.BaselineLastBackup,
		SawRunning:         run.SawRunning,
		StartedAtUnix:      run.StartedAt.Unix(),
	}
	next := Transition(obs)
	prev := run.Result
	run.Result = next

	if IsTerminal(next) && (prev == ResultRunning || prev == ResultConnectionLost) {
		end := time.Now().UTC()
		run.EndedAt = &end
		dur := end.Sub(run.StartedAt).Milliseconds()
		run.DurationMS = &dur
		if run.SawRunning {
			run.DurationQuality = DurationMeasured
		} else {
			run.DurationQuality = DurationEstimated
		}
		if next == ResultFailed && run.ErrorMessage == "" {
			run.ErrorMessage = "Backup als fehlgeschlagen gemeldet (ZimaOS-Status)."
		}
		if next == ResultUnknown {
			run.ErrorMessage = "Abschluss nicht eindeutig bestätigbar; kein Erfolg gemeldet."
		}
		if next == ResultAmbiguous {
			run.ErrorMessage = "Backup-Durchlauf nicht eindeutig zuordenbar."
		}
	}

	_ = s.runs.Update(ctx, run)
	s.hub.Publish("run", run)

	if IsTerminal(next) && (prev == ResultRunning || prev == ResultConnectionLost) {
		if IsNotifySuccess(next) {
			s.maybeComposeAfterSuccess(ctx, run)
		}
		if !run.NotificationSent {
			s.maybeNotify(ctx, run)
		}
	}
}

func (s *Service) maybeComposeAfterSuccess(ctx context.Context, run *store.BackupRun) {
	if s.compose == nil {
		return
	}
	prefs, err := s.prefs.Get(ctx, run.TaskID)
	if err != nil || !prefs.ComposeOnSuccess() {
		return
	}
	// Run async so Discord/Zima notify and poll loop stay responsive.
	go func() {
		cctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		s.compose.RunAfterBackupSuccess(cctx, run.TaskID, run.BackupName)
	}()
}

func (s *Service) maybeNotify(ctx context.Context, run *store.BackupRun) {
	if !s.settings.GetBool(settings.KeyDiscordEnabled) {
		return
	}
	prefs, _ := s.prefs.Get(ctx, run.TaskID)
	if !prefs.NotifyEnabled {
		return
	}
	webhook := s.settings.Get(settings.KeyDiscordWebhookURL)
	if webhook == "" {
		return
	}

	var embed discord.Embed
	end := time.Now().UTC()
	if run.EndedAt != nil {
		end = *run.EndedAt
	}
	duration := discord.FormatDurationHHMMSS(run.DurationMS)
	files := discord.FormatFiles(run.XferredFiles, run.TotalFiles)
	dataAmount := discord.FormatDataAmount(run.XferredSize, run.TotalSize)
	loc := loadTimezone(s.settings.Get(settings.KeyTimezone))

	switch {
	case IsNotifySuccess(run.Result) && prefs.NotifySuccess && s.settings.GetBool(settings.KeyDiscordNotifyOK):
		embed = discord.SuccessEmbed(run.BackupName, run.TaskID, duration, files, dataAmount, end, loc)
	case IsNotifyFailure(run.Result) && prefs.NotifyFailure && s.settings.GetBool(settings.KeyDiscordNotifyFail):
		embed = discord.FailureEmbed(run.BackupName, run.TaskID, duration, files, dataAmount, run.ErrorMessage, end, loc)
	default:
		return
	}

	if err := s.discord.SendEmbed(ctx, webhook, embed); err != nil {
		run.DiscordStatus = "failed"
		_ = s.runs.Update(ctx, run)
		_ = s.audit.Add(ctx, "error", "discord", "notification failed", map[string]any{
			"run_id": run.ID, "error": err.Error(),
		})
		return
	}
	run.DiscordStatus = "sent"
	run.NotificationSent = true
	_ = s.runs.Update(ctx, run)
	_ = s.audit.Add(ctx, "info", "discord", "notification sent", map[string]any{"run_id": run.ID})
}

func loadTimezone(name string) *time.Location {
	if name == "" {
		name = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.Local
	}
	return loc
}
