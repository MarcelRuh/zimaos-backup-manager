package store

import (
	"context"
	"database/sql"
	"time"
)

type TaskPrefs struct {
	TaskID         string `json:"task_id"`
	Enabled        bool   `json:"enabled"`
	NotifyEnabled  bool   `json:"notify_enabled"`
	NotifySuccess  bool   `json:"notify_success"`
	NotifyFailure  bool   `json:"notify_failure"`
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
	err := s.db.QueryRowContext(ctx, `
		SELECT task_id, enabled, notify_enabled, notify_success, notify_failure
		FROM task_prefs WHERE task_id = ?
	`, taskID).Scan(&p.TaskID, &enabled, &nEn, &nOk, &nFail)
	if err == sql.ErrNoRows {
		return TaskPrefs{
			TaskID:        taskID,
			Enabled:       true,
			NotifyEnabled: true,
			NotifySuccess: true,
			NotifyFailure: true,
		}, nil
	}
	if err != nil {
		return p, err
	}
	p.Enabled = enabled == 1
	p.NotifyEnabled = nEn == 1
	p.NotifySuccess = nOk == 1
	p.NotifyFailure = nFail == 1
	return p, nil
}

func (s *PrefsStore) Upsert(ctx context.Context, p TaskPrefs) error {
	b := func(v bool) int {
		if v {
			return 1
		}
		return 0
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_prefs(task_id, enabled, notify_enabled, notify_success, notify_failure, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			enabled = excluded.enabled,
			notify_enabled = excluded.notify_enabled,
			notify_success = excluded.notify_success,
			notify_failure = excluded.notify_failure,
			updated_at = excluded.updated_at
	`, p.TaskID, b(p.Enabled), b(p.NotifyEnabled), b(p.NotifySuccess), b(p.NotifyFailure),
		time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *PrefsStore) List(ctx context.Context) ([]TaskPrefs, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, enabled, notify_enabled, notify_success, notify_failure FROM task_prefs
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskPrefs
	for rows.Next() {
		var p TaskPrefs
		var enabled, nEn, nOk, nFail int
		if err := rows.Scan(&p.TaskID, &enabled, &nEn, &nOk, &nFail); err != nil {
			return nil, err
		}
		p.Enabled = enabled == 1
		p.NotifyEnabled = nEn == 1
		p.NotifySuccess = nOk == 1
		p.NotifyFailure = nFail == 1
		out = append(out, p)
	}
	return out, rows.Err()
}
