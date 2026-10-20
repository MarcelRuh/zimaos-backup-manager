package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	SessionTTL          = 7 * 24 * time.Hour
	MaxFailedAttempts   = 8
	LockoutWindow       = 15 * time.Minute
	MinPasswordLength   = 10
	MinUsernameLength   = 3
	MaxUsernameLength   = 64
)

var (
	ErrSetupCompleted   = errors.New("setup already completed")
	ErrSetupRequired    = errors.New("setup required")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLockedOut        = errors.New("too many failed login attempts")
	ErrInvalidSession   = errors.New("invalid session")
	ErrWeakPassword     = errors.New("password too weak")
	ErrInvalidUsername  = errors.New("invalid username")
)

type Service struct {
	db *sql.DB
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Session struct {
	ID        string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Service) CompleteSetup(ctx context.Context, username, password string) (*Session, error) {
	needs, err := s.NeedsSetup(ctx)
	if err != nil {
		return nil, err
	}
	if !needs {
		return nil, ErrSetupCompleted
	}
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users(username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, username, hash, now, now)
	if err != nil {
		return nil, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.createSession(ctx, userID)
}

func (s *Service) Login(ctx context.Context, username, password, ip string) (*Session, error) {
	username = strings.TrimSpace(username)
	locked, err := s.isLockedOut(ctx, username, ip)
	if err != nil {
		return nil, err
	}
	if locked {
		_ = s.recordAttempt(ctx, username, ip, false)
		return nil, ErrLockedOut
	}

	var userID int64
	var hash string
	err = s.db.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username = ? COLLATE NOCASE`, username).
		Scan(&userID, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		_ = s.recordAttempt(ctx, username, ip, false)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	ok, err := VerifyPassword(hash, password)
	if err != nil || !ok {
		_ = s.recordAttempt(ctx, username, ip, false)
		return nil, ErrInvalidCredentials
	}
	_ = s.recordAttempt(ctx, username, ip, true)
	return s.createSession(ctx, userID)
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
	return err
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (*Session, *User, error) {
	if sessionID == "" {
		return nil, nil, ErrInvalidSession
	}
	var sess Session
	var user User
	var expires string
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.csrf_token, s.expires_at, u.username
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ?
	`, sessionID).Scan(&sess.ID, &sess.UserID, &sess.CSRFToken, &expires, &user.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrInvalidSession
	}
	if err != nil {
		return nil, nil, err
	}
	exp, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return nil, nil, ErrInvalidSession
	}
	if time.Now().UTC().After(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID)
		return nil, nil, ErrInvalidSession
	}
	sess.ExpiresAt = exp
	user.ID = sess.UserID
	_, _ = s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), sessionID)
	return &sess, &user, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	if err := validatePassword(next); err != nil {
		return err
	}
	var hash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return err
	}
	ok, err := VerifyPassword(hash, current)
	if err != nil || !ok {
		return ErrInvalidCredentials
	}
	newHash, err := HashPassword(next)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		newHash, time.Now().UTC().Format(time.RFC3339Nano), userID)
	return err
}

func (s *Service) createSession(ctx context.Context, userID int64) (*Session, error) {
	id, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	exp := now.Add(SessionTTL)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions(id, user_id, csrf_token, created_at, expires_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, userID, csrf, now.Format(time.RFC3339Nano), exp.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	return &Session{ID: id, UserID: userID, CSRFToken: csrf, ExpiresAt: exp}, nil
}

func (s *Service) isLockedOut(ctx context.Context, username, ip string) (bool, error) {
	since := time.Now().UTC().Add(-LockoutWindow).Format(time.RFC3339Nano)
	var fails int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE success = 0 AND created_at >= ?
		  AND (username = ? COLLATE NOCASE OR ip = ?)
	`, since, username, NormalizeIP(ip)).Scan(&fails)
	if err != nil {
		return false, err
	}
	return fails >= MaxFailedAttempts, nil
}

func (s *Service) recordAttempt(ctx context.Context, username, ip string, success bool) error {
	flag := 0
	if success {
		flag = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO login_attempts(username, ip, success, created_at)
		VALUES (?, ?, ?, ?)
	`, username, NormalizeIP(ip), flag, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func validateUsername(username string) error {
	if len(username) < MinUsernameLength || len(username) > MaxUsernameLength {
		return ErrInvalidUsername
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return ErrInvalidUsername
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrWeakPassword
	}
	return nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NormalizeIP(ip string) string {
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	return ip
}

// ClientIP returns the client address for rate limiting / lockout.
// X-Forwarded-For is only honored when trustXFF is true (caller verified a trusted proxy).
func ClientIP(remoteAddr string, xff string, trustXFF bool) string {
	if trustXFF {
		xff = strings.TrimSpace(xff)
		if xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				candidate := strings.TrimSpace(parts[0])
				if candidate != "" {
					return candidate
				}
			}
		}
	}
	return NormalizeIP(remoteAddr)
}

// IsTrustedProxy reports whether peerIP is in the allow-list.
func IsTrustedProxy(peerIP string, trusted []string) bool {
	peerIP = NormalizeIP(peerIP)
	for _, t := range trusted {
		if NormalizeIP(t) == peerIP {
			return true
		}
	}
	return false
}

func FormatError(err error) string {
	switch {
	case errors.Is(err, ErrSetupCompleted):
		return "setup_completed"
	case errors.Is(err, ErrSetupRequired):
		return "setup_required"
	case errors.Is(err, ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(err, ErrLockedOut):
		return "locked_out"
	case errors.Is(err, ErrInvalidSession):
		return "invalid_session"
	case errors.Is(err, ErrWeakPassword):
		return "weak_password"
	case errors.Is(err, ErrInvalidUsername):
		return "invalid_username"
	default:
		return fmt.Sprintf("error: %v", err)
	}
}
