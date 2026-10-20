package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/domain"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

type BackupRun struct {
	ID                 int64                  `json:"id"`
	TaskID             string                 `json:"task_id"`
	BackupName         string                 `json:"backup_name"`
	Source             zimaos.Device          `json:"source"`
	Target             zimaos.Device          `json:"target"`
	StartedAt          time.Time              `json:"started_at"`
	EndedAt            *time.Time             `json:"ended_at,omitempty"`
	DurationMS         *int64                 `json:"duration_ms,omitempty"`
	DurationQuality    domain.DurationQuality `json:"duration_quality"`
	Result             domain.Result          `json:"result"`
	TotalFiles         int64                  `json:"total_files"`
	XferredFiles       int64                  `json:"xferred_files"`
	TotalSize          int64                  `json:"total_size"`
	XferredSize        int64                  `json:"xferred_size"`
	ErrorMessage       string                 `json:"error_message"`
	DiscordStatus      string                 `json:"discord_status"`
	Fingerprint        string                 `json:"fingerprint"`
	BaselineLastBackup int64                  `json:"baseline_last_backup_time"`
	SawRunning         bool                   `json:"saw_running"`
	NotificationSent   bool                   `json:"notification_sent"`
}

type RunFilter struct {
	TaskID    string
	Result    string
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

type RunStore struct {
	db *sql.DB
}

func NewRunStore(db *sql.DB) *RunStore {
	return &RunStore{db: db}
}

func (s *RunStore) Create(ctx context.Context, run *BackupRun) error {
	sourceJSON, _ := json.Marshal(run.Source)
	targetJSON, _ := json.Marshal(run.Target)
	saw := 0
	if run.SawRunning {
		saw = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO backup_runs(
			task_id, backup_name, source_json, target_json, started_at, ended_at,
			duration_ms, duration_quality, result, total_files, xferred_files,
			total_size, xferred_size, error_message, discord_status, fingerprint,
			baseline_last_backup_time, saw_running, notification_sent, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`,
		run.TaskID, run.BackupName, string(sourceJSON), string(targetJSON),
		run.StartedAt.UTC().Format(time.RFC3339Nano),
		run.DurationQuality, run.Result, run.TotalFiles, run.XferredFiles,
		run.TotalSize, run.XferredSize, run.ErrorMessage, run.DiscordStatus, run.Fingerprint,
		run.BaselineLastBackup, saw, now, now,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	run.ID = id
	return nil
}

func (s *RunStore) Update(ctx context.Context, run *BackupRun) error {
	sourceJSON, _ := json.Marshal(run.Source)
	targetJSON, _ := json.Marshal(run.Target)
	saw := 0
	if run.SawRunning {
		saw = 1
	}
	notif := 0
	if run.NotificationSent {
		notif = 1
	}
	var ended any
	var duration any
	if run.EndedAt != nil {
		ended = run.EndedAt.UTC().Format(time.RFC3339Nano)
	}
	if run.DurationMS != nil {
		duration = *run.DurationMS
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE backup_runs SET
			backup_name = ?, source_json = ?, target_json = ?, ended_at = ?, duration_ms = ?,
			duration_quality = ?, result = ?, total_files = ?, xferred_files = ?,
			total_size = ?, xferred_size = ?, error_message = ?, discord_status = ?,
			saw_running = ?, notification_sent = ?, updated_at = ?
		WHERE id = ?
	`,
		run.BackupName, string(sourceJSON), string(targetJSON), ended, duration,
		run.DurationQuality, run.Result, run.TotalFiles, run.XferredFiles,
		run.TotalSize, run.XferredSize, run.ErrorMessage, run.DiscordStatus,
		saw, notif, time.Now().UTC().Format(time.RFC3339Nano), run.ID,
	)
	return err
}

func (s *RunStore) GetOpenByTask(ctx context.Context, taskID string) (*BackupRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, backup_name, source_json, target_json, started_at, ended_at,
			duration_ms, duration_quality, result, total_files, xferred_files, total_size,
			xferred_size, error_message, discord_status, fingerprint, baseline_last_backup_time,
			saw_running, notification_sent
		FROM backup_runs
		WHERE task_id = ? AND result IN ('running', 'connection_lost')
		ORDER BY id DESC LIMIT 1
	`, taskID)
	return scanRun(row)
}

func (s *RunStore) ListOpen(ctx context.Context) ([]BackupRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, backup_name, source_json, target_json, started_at, ended_at,
			duration_ms, duration_quality, result, total_files, xferred_files, total_size,
			xferred_size, error_message, discord_status, fingerprint, baseline_last_backup_time,
			saw_running, notification_sent
		FROM backup_runs
		WHERE result IN ('running', 'connection_lost')
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *run)
	}
	return out, rows.Err()
}

func (s *RunStore) List(ctx context.Context, f RunFilter) ([]BackupRun, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.TaskID != "" {
		where = append(where, "task_id = ?")
		args = append(args, f.TaskID)
	}
	if f.Result != "" {
		where = append(where, "result = ?")
		args = append(args, f.Result)
	}
	if f.From != nil {
		where = append(where, "started_at >= ?")
		args = append(args, f.From.UTC().Format(time.RFC3339Nano))
	}
	if f.To != nil {
		where = append(where, "started_at <= ?")
		args = append(args, f.To.UTC().Format(time.RFC3339Nano))
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, backup_name, source_json, target_json, started_at, ended_at,
			duration_ms, duration_quality, result, total_files, xferred_files, total_size,
			xferred_size, error_message, discord_status, fingerprint, baseline_last_backup_time,
			saw_running, notification_sent
		FROM backup_runs
		WHERE `+clause+`
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []BackupRun
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *run)
	}
	return out, total, rows.Err()
}

func (s *RunStore) LastSuccessful(ctx context.Context) (*BackupRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, backup_name, source_json, target_json, started_at, ended_at,
			duration_ms, duration_quality, result, total_files, xferred_files, total_size,
			xferred_size, error_message, discord_status, fingerprint, baseline_last_backup_time,
			saw_running, notification_sent
		FROM backup_runs WHERE result = 'success' ORDER BY COALESCE(ended_at, started_at) DESC LIMIT 1
	`)
	run, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return run, err
}

func (s *RunStore) FingerprintExists(ctx context.Context, fp string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backup_runs WHERE fingerprint = ?`, fp).Scan(&n)
	return n > 0, err
}

func MakeFingerprint(taskID string, startedAt time.Time, baselineLastBackup int64) string {
	return fmt.Sprintf("%s|%d|%d", taskID, startedAt.UTC().Unix(), baselineLastBackup)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (*BackupRun, error) {
	var (
		run       BackupRun
		sourceRaw string
		targetRaw string
		started   string
		ended     sql.NullString
		duration  sql.NullInt64
		quality   string
		result    string
		saw       int
		notif     int
	)
	err := row.Scan(
		&run.ID, &run.TaskID, &run.BackupName, &sourceRaw, &targetRaw, &started, &ended,
		&duration, &quality, &result, &run.TotalFiles, &run.XferredFiles, &run.TotalSize,
		&run.XferredSize, &run.ErrorMessage, &run.DiscordStatus, &run.Fingerprint,
		&run.BaselineLastBackup, &saw, &notif,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(sourceRaw), &run.Source)
	_ = json.Unmarshal([]byte(targetRaw), &run.Target)
	t, err := time.Parse(time.RFC3339Nano, started)
	if err != nil {
		return nil, err
	}
	run.StartedAt = t
	if ended.Valid {
		et, err := time.Parse(time.RFC3339Nano, ended.String)
		if err == nil {
			run.EndedAt = &et
		}
	}
	if duration.Valid {
		v := duration.Int64
		run.DurationMS = &v
	}
	run.DurationQuality = domain.DurationQuality(quality)
	run.Result = domain.Result(result)
	run.SawRunning = saw == 1
	run.NotificationSent = notif == 1
	return &run, nil
}

func scanRunRows(rows *sql.Rows) (*BackupRun, error) {
	return scanRun(rows)
}
