package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type AuditLog struct {
	ID        int64          `json:"id"`
	Level     string         `json:"level"`
	Category  string         `json:"category"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type AuditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

func (s *AuditStore) Add(ctx context.Context, level, category, message string, details map[string]any) error {
	if details == nil {
		details = map[string]any{}
	}
	raw, _ := json.Marshal(details)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_logs(level, category, message, details_json, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, level, category, message, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *AuditStore) List(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, level, category, message, details_json, created_at
		FROM audit_logs ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditLog
	for rows.Next() {
		var (
			a       AuditLog
			details string
			created string
		)
		if err := rows.Scan(&a.ID, &a.Level, &a.Category, &a.Message, &details, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(details), &a.Details)
		t, _ := time.Parse(time.RFC3339Nano, created)
		a.CreatedAt = t
		out = append(out, a)
	}
	return out, rows.Err()
}
