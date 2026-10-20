package control

import (
	"context"
	"errors"
	"fmt"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/monitor"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

var (
	ErrAPIDown          = errors.New("zimaos api not reachable")
	ErrTaskDisabled     = errors.New("backup task disabled in manager")
	ErrAlreadyRunning   = errors.New("backup already running")
	ErrTargetOffline    = errors.New("backup target not connected")
	ErrStartInProgress  = errors.New("start already in progress for this task")
)

type Service struct {
	client  *zimaos.Client
	monitor *monitor.Service
	runs    *store.RunStore
	prefs   *store.PrefsStore
	audit   *store.AuditStore
}

func NewService(client *zimaos.Client, mon *monitor.Service, runs *store.RunStore, prefs *store.PrefsStore, audit *store.AuditStore) *Service {
	return &Service{client: client, monitor: mon, runs: runs, prefs: prefs, audit: audit}
}

func (s *Service) StartBackup(ctx context.Context, taskID string) (*store.BackupRun, string, error) {
	if !s.monitor.TryLockTask(taskID) {
		return nil, "", ErrStartInProgress
	}
	defer s.monitor.UnlockTask(taskID)

	prefs, err := s.prefs.Get(ctx, taskID)
	if err != nil {
		return nil, "", err
	}
	if !prefs.Enabled {
		return nil, "", ErrTaskDisabled
	}

	task, err := s.client.GetTask(ctx, taskID)
	if err != nil {
		if errors.Is(err, zimaos.ErrUnavailable) {
			return nil, "", ErrAPIDown
		}
		return nil, "", err
	}

	if task.EffectiveRunning() {
		return nil, "", ErrAlreadyRunning
	}
	if open, err := s.runs.GetOpenByTask(ctx, taskID); err == nil && open != nil {
		return nil, "", ErrAlreadyRunning
	}
	if task.AnyTargetDisconnected() {
		return nil, "", ErrTargetOffline
	}

	msg, err := s.client.StartTask(ctx, taskID)
	if err != nil {
		_ = s.audit.Add(ctx, "error", "backup", "failed to start backup", map[string]any{
			"task_id": taskID, "error": err.Error(),
		})
		return nil, "", fmt.Errorf("start task: %w", err)
	}

	// Refresh task after start for baseline
	refreshed, err := s.client.GetTask(ctx, taskID)
	if err != nil {
		refreshed = task
	}
	run, err := s.monitor.BeginManagedRun(ctx, *refreshed)
	if err != nil {
		return nil, msg, err
	}
	_ = s.audit.Add(ctx, "info", "backup", "backup start requested", map[string]any{
		"task_id": taskID, "run_id": run.ID, "api_message": msg,
	})
	return run, msg, nil
}
