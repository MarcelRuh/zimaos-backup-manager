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
	ErrAPIDown             = errors.New("zimaos api not reachable")
	ErrTaskDisabled        = errors.New("backup task disabled in manager")
	ErrAlreadyRunning      = errors.New("backup already running")
	ErrTargetOffline       = errors.New("backup target not connected")
	ErrStartInProgress     = errors.New("start already in progress for this task")
	ErrComposeBeforeFailed = errors.New("compose sync before backup failed")
)

// ComposeBeforeBackup syncs compose files into AppData before a ZimaOS backup starts.
type ComposeBeforeBackup interface {
	RunBeforeBackup(ctx context.Context, taskID, backupName string) error
}

type Service struct {
	client  *zimaos.Client
	monitor *monitor.Service
	runs    *store.RunStore
	prefs   *store.PrefsStore
	audit   *store.AuditStore
	compose ComposeBeforeBackup
}

func NewService(
	client *zimaos.Client,
	mon *monitor.Service,
	runs *store.RunStore,
	prefs *store.PrefsStore,
	audit *store.AuditStore,
	compose ComposeBeforeBackup,
) *Service {
	return &Service{client: client, monitor: mon, runs: runs, prefs: prefs, audit: audit, compose: compose}
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

	if prefs.ComposeBefore() && s.compose != nil {
		if err := s.compose.RunBeforeBackup(ctx, taskID, task.Name); err != nil {
			_ = s.audit.Add(ctx, "error", "compose_backup", "compose sync before backup failed", map[string]any{
				"task_id": taskID, "error": err.Error(),
			})
			return nil, "", fmt.Errorf("%w: %v", ErrComposeBeforeFailed, err)
		}
	}

	msg, err := s.client.StartTask(ctx, taskID)
	if err != nil {
		_ = s.audit.Add(ctx, "error", "backup", "failed to start backup", map[string]any{
			"task_id": taskID, "error": err.Error(),
		})
		return nil, "", fmt.Errorf("start task: %w", err)
	}

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
