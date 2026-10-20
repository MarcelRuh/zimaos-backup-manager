package settings

import (
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/secretbox"
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

	encryptionKeyFile = ".encryption-key"
)

type Store struct {
	db  *sql.DB
	box *secretbox.Box
	mu  sync.RWMutex
	mem map[string]string
}

func NewStore(db *sql.DB, cfg config.Config) (*Store, error) {
	box, err := secretbox.OpenOrCreate(filepath.Join(cfg.DataDir, encryptionKeyFile))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, box: box, mem: map[string]string{}}
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
	if err := s.migratePlainSecrets(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureDefault(ctx context.Context, key, value string) error {
	stored, err := s.sealIfSecret(key, value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO NOTHING
	`, key, stored, time.Now().UTC().Format(time.RFC3339Nano))
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
		plain, err := s.openIfSecret(k, v)
		if err != nil {
			return err
		}
		next[k] = plain
	}
	s.mu.Lock()
	s.mem = next
	s.mu.Unlock()
	return rows.Err()
}

// migratePlainSecrets re-seals legacy plaintext password rows.
func (s *Store) migratePlainSecrets(ctx context.Context) error {
	raw, err := s.rawValue(ctx, KeyZimaOSPassword)
	if err != nil || raw == "" || secretbox.IsSealed(raw) {
		return err
	}
	return s.Set(ctx, KeyZimaOSPassword, raw)
}

func (s *Store) rawValue(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
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
	stored, err := s.sealIfSecret(key, value)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, stored, time.Now().UTC().Format(time.RFC3339Nano))
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
	plainMem := make(map[string]string, len(values))
	for k, v := range values {
		stored, err := s.sealIfSecret(k, v)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings(key, value, updated_at) VALUES (?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
		`, k, stored, now); err != nil {
			return err
		}
		plainMem[k] = v
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	for k, v := range plainMem {
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

func (s *Store) sealIfSecret(key, value string) (string, error) {
	if key != KeyZimaOSPassword || value == "" {
		return value, nil
	}
	if secretbox.IsSealed(value) {
		return value, nil
	}
	return s.box.Seal(value)
}

func (s *Store) openIfSecret(key, value string) (string, error) {
	if key != KeyZimaOSPassword {
		return value, nil
	}
	return s.box.Open(value)
}
