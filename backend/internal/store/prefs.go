package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const (
	ComposeHookOff       = "off"
	ComposeHookBefore    = "before"
	ComposeHookOnSuccess = "on_success"
	ComposeHookBoth      = "both"
)

type TaskPrefs struct {
	TaskID            string `json:"task_id"`
	Enabled           bool   `json:"enabled"`
	NotifyEnabled     bool   `json:"notify_enabled"`
	NotifySuccess     bool   `json:"notify_success"`
	NotifyFailure     bool   `json:"notify_failure"`
	ComposeBackupHook string `json:"compose_backup_hook"`
}

func NormalizeComposeHook(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case ComposeHookBefore:
		return ComposeHookBefore
	case ComposeHookBoth:
		return ComposeHookBoth
	case ComposeHookOnSuccess, "after", "success", "on_complete":
		return ComposeHookOnSuccess
	default:
		return ComposeHookOff
	}
}

func (p TaskPrefs) ComposeBefore() bool {
	h := NormalizeComposeHook(p.ComposeBackupHook)
	return h == ComposeHookBefore || h == ComposeHookBoth
}

func (p TaskPrefs) ComposeOnSuccess() bool {
	h := NormalizeComposeHook(p.ComposeBackupHook)
	return h == ComposeHookOnSuccess || h == ComposeHookBoth
}

type PrefsStore struct {
	db *sql.DB
}

func NewPrefsStore(db *sql.DB) *PrefsStore {
	return &PrefsStore{db: db}
}

func (s *PrefsStore) Get(ctx context.Context, taskID string) (TaskPrefs, error) {
	var p TaskPrefs
	var enabled, nEn, nOk, nFail int
	var hook sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT task_id, enabled, notify_enabled, notify_success, notify_failure, compose_backup_hook
		FROM task_prefs WHERE task_id = ?
	`, taskID).Scan(&p.TaskID, &enabled, &nEn, &nOk, &nFail, &hook)
	if err == sql.ErrNoRows {
		return TaskPrefs{
			TaskID:            taskID,
			Enabled:           true,
			NotifyEnabled:     true,
			NotifySuccess:     true,
			NotifyFailure:     true,
			ComposeBackupHook: ComposeHookOff,
		}, nil
	}
	if err != nil {
		return p, err
	}
	p.Enabled = enabled == 1
	p.NotifyEnabled = nEn == 1
	p.NotifySuccess = nOk == 1
	p.NotifyFailure = nFail == 1
	p.ComposeBackupHook = NormalizeComposeHook(hook.String)
	return p, nil
}

func (s *PrefsStore) Upsert(ctx context.Context, p TaskPrefs) error {
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	hook := NormalizeComposeHook(p.ComposeBackupHook)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_prefs(task_id, enabled, notify_enabled, notify_success, notify_failure, compose_backup_hook, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			enabled = excluded.enabled,
			notify_enabled = excluded.notify_enabled,
			notify_success = excluded.notify_success,
			notify_failure = excluded.notify_failure,
			compose_backup_hook = excluded.compose_backup_hook,
			updated_at = excluded.updated_at
	`, p.TaskID, b(p.Enabled), b(p.NotifyEnabled), b(p.NotifySuccess), b(p.NotifyFailure), hook,
		time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *PrefsStore) List(ctx context.Context) ([]TaskPrefs, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, enabled, notify_enabled, notify_success, notify_failure, compose_backup_hook FROM task_prefs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskPrefs
	for rows.Next() {
		var p TaskPrefs
		var enabled, nEn, nOk, nFail int
		var hook sql.NullString
		if err := rows.Scan(&p.TaskID, &enabled, &nEn, &nOk, &nFail, &hook); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		p.NotifyEnabled = nEn == 1
		p.NotifySuccess = nOk == 1
		p.NotifyFailure = nFail == 1
		p.ComposeBackupHook = NormalizeComposeHook(hook.String)
		out = append(out, p)
	}
	return out, rows.Err()
}
