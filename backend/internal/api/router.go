package api

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/app"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/static"
)

const (
	setupRateLimitWindow = time.Minute
	setupRateLimitMax    = 10
)

type Server struct {
	app *app.App

	setupMu   sync.Mutex
	setupHits map[string][]time.Time
}

func NewRouter(application *app.App) http.Handler {
	s := &Server{app: application, setupHits: map[string][]time.Time{}}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/setup/status", s.handleSetupStatus)
	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.requireSetupComplete(s.handleLogin))
	mux.HandleFunc("POST /api/auth/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handleChangePassword))

	mux.HandleFunc("GET /api/dashboard", s.requireAuth(s.handleDashboard))
	mux.HandleFunc("GET /api/tasks", s.requireAuth(s.handleTasks))
	mux.HandleFunc("POST /api/tasks/{id}/start", s.requireAuth(s.handleStartTask))
	mux.HandleFunc("GET /api/tasks/{id}/prefs", s.requireAuth(s.handleGetPrefs))
	mux.HandleFunc("PUT /api/tasks/{id}/prefs", s.requireAuth(s.handlePutPrefs))

	mux.HandleFunc("GET /api/history", s.requireAuth(s.handleHistory))
	mux.HandleFunc("GET /api/schedules", s.requireAuth(s.handleListSchedules))
	mux.HandleFunc("POST /api/schedules", s.requireAuth(s.handleCreateSchedule))
	mux.HandleFunc("PUT /api/schedules/{id}", s.requireAuth(s.handleUpdateSchedule))
	mux.HandleFunc("DELETE /api/schedules/{id}", s.requireAuth(s.handleDeleteSchedule))

	mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleGetSettings))
	mux.HandleFunc("PUT /api/settings", s.requireAuth(s.handlePutSettings))
	mux.HandleFunc("POST /api/discord/test", s.requireAuth(s.handleDiscordTest))
	mux.HandleFunc("GET /api/compose-backup", s.requireAuth(s.handleComposeBackupStatus))
	mux.HandleFunc("POST /api/compose-backup/run", s.requireAuth(s.handleComposeBackupRun))
	mux.HandleFunc("GET /api/system", s.requireAuth(s.handleSystem))
	mux.HandleFunc("GET /api/logs", s.requireAuth(s.handleLogs))
	mux.HandleFunc("GET /api/events", s.requireAuth(s.handleSSE))

	frontend, err := fs.Sub(static.FS, "dist")
	if err != nil {
		// Fallback empty: health/API still work during development without built UI
		frontend = static.Empty
	}
	fileServer := http.FileServer(http.FS(frontend))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(frontend, path); err != nil {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) allowSetupAttempt(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-setupRateLimitWindow)
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	hits := s.setupHits[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= setupRateLimitMax {
		s.setupHits[ip] = kept
		return false
	}
	s.setupHits[ip] = append(kept, now)
	return true
}
