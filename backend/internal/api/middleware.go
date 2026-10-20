package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/app"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/auth"
)

type ctxKey string

const (
	ctxUser    ctxKey = "user"
	ctxSession ctxKey = "session"
)

func withUser(ctx context.Context, u *auth.User) context.Context {
	return context.WithValue(ctx, ctxUser, u)
}

func withSession(ctx context.Context, s *auth.Session) context.Context {
	return context.WithValue(ctx, ctxSession, s)
}

func userFrom(ctx context.Context) *auth.User {
	u, _ := ctx.Value(ctxUser).(*auth.User)
	return u
}

func sessionFrom(ctx context.Context) *auth.Session {
	s, _ := ctx.Value(ctxSession).(*auth.Session)
	return s
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(s.app.Config.SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sess, user, err := s.app.Auth.GetSession(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			csrf := r.Header.Get("X-CSRF-Token")
			if csrf == "" || csrf != sess.CSRFToken {
				writeError(w, http.StatusForbidden, "csrf_invalid")
				return
			}
		}
		next(w, r.WithContext(withSession(withUser(r.Context(), user), sess)))
	}
}

func (s *Server) requireSetupComplete(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		needs, err := s.app.Auth.NeedsSetup(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		if needs {
			writeError(w, http.StatusForbidden, "setup_required")
			return
		}
		next(w, r)
	}
}

func (s *Server) clientIP(r *http.Request) string {
	trustXFF := auth.IsTrustedProxy(r.RemoteAddr, s.app.Config.TrustedProxies)
	return auth.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), trustXFF)
}

func setSessionCookie(w http.ResponseWriter, a *app.App, sessionID string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.Config.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(auth.SessionTTL.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, a *app.App) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.Config.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")
}
