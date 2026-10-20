package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultListenAddr        = ":8787"
	DefaultDataDir           = "./data"
	DefaultZimaOSAPIBaseURL  = "http://host.docker.internal"
	DefaultTimezone          = "Europe/Berlin"
	DefaultPollInterval      = 5 * time.Second
	DefaultSessionCookieName = "zbm_session"
	DefaultLanguage          = "de"
	DefaultTheme             = "dark"
	DefaultCasaOSAppsDir     = "/host/casaos/apps"
)

type Config struct {
	ListenAddr        string
	DataDir           string
	DatabasePath      string
	ZimaOSAPIBaseURL  string
	ZimaOSAPIToken    string
	ZimaOSUsername    string
	ZimaOSPassword    string
	Timezone          string
	PollInterval      time.Duration
	SessionCookieName string
	CookieSecure      bool
	AppPublicURL      string
	// TrustedProxies lists peer IPs allowed to set X-Forwarded-For.
	// Empty (default) means XFF is ignored and RemoteAddr is used.
	TrustedProxies []string
	// CasaOSAppsDir is the host apps compose directory (bind-mounted read-only).
	CasaOSAppsDir string
}

func LoadFromEnv() (Config, error) {
	dataDir := envOr("DATA_DIR", DefaultDataDir)
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data dir: %w", err)
	}

	pollSeconds := envIntOr("POLL_INTERVAL_SECONDS", int(DefaultPollInterval.Seconds()))
	if pollSeconds < 2 {
		pollSeconds = 2
	}

	cfg := Config{
		ListenAddr:        envOr("LISTEN_ADDR", DefaultListenAddr),
		DataDir:           absDataDir,
		DatabasePath:      filepath.Join(absDataDir, "backup-manager.db"),
		ZimaOSAPIBaseURL:  strings.TrimRight(envOr("ZIMAOS_API_BASE_URL", DefaultZimaOSAPIBaseURL), "/"),
		ZimaOSAPIToken:    strings.TrimSpace(os.Getenv("ZIMAOS_API_TOKEN")),
		ZimaOSUsername:    strings.TrimSpace(os.Getenv("ZIMAOS_API_USERNAME")),
		ZimaOSPassword:    os.Getenv("ZIMAOS_API_PASSWORD"),
		Timezone:          envOr("TZ", DefaultTimezone),
		PollInterval:      time.Duration(pollSeconds) * time.Second,
		SessionCookieName: envOr("SESSION_COOKIE_NAME", DefaultSessionCookieName),
		CookieSecure:      envBoolOr("COOKIE_SECURE", false),
		AppPublicURL:      strings.TrimRight(os.Getenv("APP_PUBLIC_URL"), "/"),
		TrustedProxies:    splitCSV(os.Getenv("TRUSTED_PROXIES")),
		CasaOSAppsDir:     envOr("CASAOS_APPS_DIR", DefaultCasaOSAppsDir),
	}

	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return Config{}, fmt.Errorf("create data directory %q: %w (check volume permissions)", cfg.DataDir, err)
	}

	testFile := filepath.Join(cfg.DataDir, ".write-test")
	if err := os.WriteFile(testFile, []byte("ok"), 0o600); err != nil {
		return Config{}, fmt.Errorf("data directory %q is not writable: %w", cfg.DataDir, err)
	}
	_ = os.Remove(testFile)

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBoolOr(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func splitCSV(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
