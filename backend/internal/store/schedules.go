package store

import (
	"context"
	"database/sql"
	"time"
)

type Schedule struct {
	ID              int64      `json:"id"`
	TaskID          string     `json:"task_id"`
	Name            string     `json:"name"`
	CronExpr        string     `json:"cron_expr"`
	Timezone        string     `json:"timezone"`
	Enabled         bool       `json:"enabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
}

type ScheduleStore struct {
	db *sql.DB
}

func NewScheduleStore(db *sql.DB) *ScheduleStore {
	return &ScheduleStore{db: db}
}

func (s *ScheduleStore) List(ctx context.Context) ([]Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, name, cron_expr, timezone, enabled, created_at, updated_at, last_triggered_at
		FROM schedules ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Schedule
	for rows.Next() {
		sch, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sch)
	}
	return out, rows.Err()
}

func (s *ScheduleStore) Get(ctx context.Context, id int64) (*Schedule, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, task_id, name, cron_expr, timezone, enabled, created_at, updated_at, last_triggered_at
		FROM schedules WHERE id = ?
	`, id)
	return scanSchedule(row)
}

func (s *ScheduleStore) Create(ctx context.Context, sch *Schedule) error {
	now := time.Now().UTC()
	en := 0
	if sch.Enabled {
		en = 1
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO schedules(task_id, name, cron_expr, timezone, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, sch.TaskID, sch.Name, sch.CronExpr, sch.Timezone, en,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	sch.ID = id
	sch.CreatedAt = now
	sch.UpdatedAt = now
	return nil
}

func (s *ScheduleStore) Update(ctx context.Context, sch *Schedule) error {
	en := 0
	if sch.Enabled {
		en = 1
	}
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE schedules SET task_id = ?, name = ?, cron_expr = ?, timezone = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`, sch.TaskID, sch.Name, sch.CronExpr, sch.Timezone, en, now.Format(time.RFC3339Nano), sch.ID)
	sch.UpdatedAt = now
	return err
}

func (s *ScheduleStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	return err
}

func (s *ScheduleStore) MarkTriggered(ctx context.Context, id int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE schedules SET last_triggered_at = ?, updated_at = ? WHERE id = ?
	`, at.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func scanSchedule(row scanner) (*Schedule, error) {
	var (
		sch     Schedule
		enabled int
		created string
		updated string
		last    sql.NullString
	)
	if err := row.Scan(&sch.ID, &sch.TaskID, &sch.Name, &sch.CronExpr, &sch.Timezone, &enabled, &created, &updated, &last); err != nil {
		return nil, err
	}
	sch.Enabled = enabled == 1
	var err error
	sch.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, err
	}
	sch.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return nil, err
	}
	if last.Valid {
		t, err := time.Parse(time.RFC3339Nano, last.String)
		if err == nil {
			sch.LastTriggeredAt = &t
		}
	}
	return &sch, nil
}
