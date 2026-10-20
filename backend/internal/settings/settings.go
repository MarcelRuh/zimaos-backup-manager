package settings

import (
	"context"
	"database/sql"
	"strconv"
	"sync"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
)

const (
	KeyLanguage          = "language"
	KeyTheme             = "theme"
	KeyTimezone          = "timezone"
	KeyZimaOSAPIBaseURL  = "zimaos_api_base_url"
	KeyZimaOSAPIToken    = "zimaos_api_token"
	KeyZimaOSUsername    = "zimaos_username"
	KeyZimaOSPassword    = "zimaos_password"
	KeyPollIntervalSec   = "poll_interval_seconds"
	KeyDiscordWebhookURL = "discord_webhook_url"
	KeyDiscordEnabled    = "discord_enabled"
	KeyDiscordNotifyOK   = "discord_notify_success"
	KeyDiscordNotifyFail = "discord_notify_failure"
	KeySetupCompleted    = "setup_completed"
)

type Store struct {
	db  *sql.DB
	mu  sync.RWMutex
	mem map[string]string
}

func NewStore(db *sql.DB, cfg config.Config) (*Store, error) {
	s := &Store{db: db, mem: map[string]string{}}
	defaults := map[string]string{
		KeyLanguage:          config.DefaultLanguage,
		KeyTheme:             config.DefaultTheme,
		KeyTimezone:          cfg.Timezone,
		KeyZimaOSAPIBaseURL:  cfg.ZimaOSAPIBaseURL,
		KeyZimaOSAPIToken:    cfg.ZimaOSAPIToken,
		KeyZimaOSUsername:    cfg.ZimaOSUsername,
		KeyZimaOSPassword:    cfg.ZimaOSPassword,
		KeyPollIntervalSec:   "5",
		KeyDiscordWebhookURL: "",
		KeyDiscordEnabled:    "false",
		KeyDiscordNotifyOK:   "true",
		KeyDiscordNotifyFail: "true",
		KeySetupCompleted:    "false",
	}
	ctx := context.Background()
	for k, v := range defaults {
		if err := s.ensureDefault(ctx, k, v); err != nil {
			return nil, err
		}
	}
	if err := s.reload(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureDefault(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO NOTHING
	`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) reload(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return err
	}
	defer rows.Close()

	next := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		next[k] = v
	}
	s.mu.Lock()
	s.mem = next
	s.mu.Unlock()
	return rows.Err()
}

func (s *Store) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mem[key]
}

func (s *Store) GetBool(key string) bool {
	v := s.Get(key)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (s *Store) GetInt(key string, fallback int) int {
	n, err := strconv.Atoi(s.Get(key))
	if err != nil {
		return fallback
	}
	return n
}

func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.mem[key] = value
	s.mu.Unlock()
	return nil
}

func (s *Store) SetMany(ctx context.Context, values map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	for k, v := range values {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings(key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		`, k, v, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	for k, v := range values {
		s.mem[k] = v
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) PublicView() map[string]any {
	return map[string]any{
		"language":               s.Get(KeyLanguage),
		"theme":                  s.Get(KeyTheme),
		"timezone":               s.Get(KeyTimezone),
		"zimaos_api_base_url":    s.Get(KeyZimaOSAPIBaseURL),
		"zimaos_api_token_set":   s.Get(KeyZimaOSAPIToken) != "",
		"zimaos_username":        s.Get(KeyZimaOSUsername),
		"zimaos_password_set":    s.Get(KeyZimaOSPassword) != "",
		"poll_interval_seconds":  s.GetInt(KeyPollIntervalSec, 5),
		"discord_enabled":        s.GetBool(KeyDiscordEnabled),
		"discord_webhook_set":    s.Get(KeyDiscordWebhookURL) != "",
		"discord_notify_success": s.GetBool(KeyDiscordNotifyOK),
		"discord_notify_failure": s.GetBool(KeyDiscordNotifyFail),
		"setup_completed":        s.GetBool(KeySetupCompleted),
	}
}
