package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/auth"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/composebackup"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/control"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/discord"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/scheduler"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/version"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version.Version,
	})
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	needs, err := s.app.Auth.NeedsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"needs_setup": needs})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !s.allowSetupAttempt(s.clientIP(r)) {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	sess, err := s.app.Auth.CompleteSetup(r.Context(), body.Username, body.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrSetupCompleted):
			writeError(w, http.StatusConflict, "setup_completed")
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, "weak_password")
		case errors.Is(err, auth.ErrInvalidUsername):
			writeError(w, http.StatusBadRequest, "invalid_username")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	_ = s.app.Settings.Set(r.Context(), settings.KeySetupCompleted, "true")
	setSessionCookie(w, s.app, sess.ID, s.app.Config.CookieSecure || r.TLS != nil)
	writeJSON(w, http.StatusCreated, map[string]any{
		"csrf_token": sess.CSRFToken,
		"user":       map[string]any{"username": body.Username},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	sess, err := s.app.Auth.Login(r.Context(), body.Username, body.Password, s.clientIP(r))
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrLockedOut):
			writeError(w, http.StatusTooManyRequests, "locked_out")
		default:
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
		}
		return
	}
	setSessionCookie(w, s.app, sess.ID, s.app.Config.CookieSecure || r.TLS != nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"csrf_token": sess.CSRFToken,
		"user":       map[string]any{"username": body.Username},
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess := sessionFrom(r.Context()); sess != nil {
		_ = s.app.Auth.Logout(r.Context(), sess.ID)
	}
	clearSessionCookie(w, s.app)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	sess := sessionFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"csrf_token": sess.CSRFToken,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	user := userFrom(r.Context())
	if err := s.app.Auth.ChangePassword(r.Context(), user.ID, body.Current, body.Next); err != nil {
		switch {
		case errors.Is(err, auth.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, "weak_password")
		case errors.Is(err, auth.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
		default:
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tasks := s.app.Monitor.CachedTasks()
	connected, lastOK, lastErr := s.app.ZimaOS.Status()
	var running, completed, failed, idle int
	for _, t := range tasks {
		switch {
		case t.EffectiveRunning():
			running++
		case t.EffectiveFailed():
			failed++
		case t.EffectiveCompleted():
			completed++
		default:
			idle++
		}
	}
	lastSuccess, _ := s.app.Runs.LastSuccessful(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"total_tasks":    len(tasks),
		"running":        running,
		"completed":      completed,
		"failed":         failed,
		"idle":           idle,
		"last_success":   lastSuccess,
		"api_connected":  connected,
		"api_last_ok":    lastOK,
		"api_last_error": lastErr,
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	tasks := s.app.Monitor.CachedTasks()
	if len(tasks) == 0 {
		ctxTasks, err := s.app.ZimaOS.ListTasks(r.Context())
		if err == nil {
			tasks = ctxTasks
		}
	}
	type view struct {
		zimaos.Task
		Prefs store.TaskPrefs `json:"prefs"`
	}
	out := make([]view, 0, len(tasks))
	for _, t := range tasks {
		prefs, _ := s.app.Prefs.Get(r.Context(), t.ID)
		out = append(out, view{Task: t, Prefs: prefs})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleStartTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, msg, err := s.app.Control.StartBackup(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, control.ErrAlreadyRunning), errors.Is(err, control.ErrStartInProgress):
			writeError(w, http.StatusConflict, "already_running")
		case errors.Is(err, control.ErrTargetOffline):
			writeError(w, http.StatusConflict, "target_offline")
		case errors.Is(err, control.ErrTaskDisabled):
			writeError(w, http.StatusForbidden, "task_disabled")
		case errors.Is(err, control.ErrComposeBeforeFailed):
			writeError(w, http.StatusFailedDependency, "compose_before_failed")
		case errors.Is(err, control.ErrAPIDown), errors.Is(err, zimaos.ErrUnavailable):
			writeError(w, http.StatusBadGateway, "api_unavailable")
		case errors.Is(err, zimaos.ErrNotFound):
			writeError(w, http.StatusNotFound, "task_not_found")
		default:
			writeError(w, http.StatusBadGateway, "start_failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "message": msg})
}

func (s *Server) handleGetPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.app.Prefs.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handlePutPrefs(w http.ResponseWriter, r *http.Request) {
	var body store.TaskPrefs
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	body.TaskID = r.PathValue("id")
	body.ComposeBackupHook = store.NormalizeComposeHook(body.ComposeBackupHook)
	if err := s.app.Prefs.Upsert(r.Context(), body); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := store.RunFilter{
		TaskID: q.Get("task_id"),
		Result: q.Get("result"),
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("offset"); v != "" {
		f.Offset, _ = strconv.Atoi(v)
	}
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	items, total, err := s.app.Runs.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.Schedules.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TaskID   string `json:"task_id"`
		Name     string `json:"name"`
		CronExpr string `json:"cron_expr"`
		Preset   string `json:"preset"`
		Timezone string `json:"timezone"`
		Enabled  bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	cronExpr := body.CronExpr
	if body.Preset != "" {
		p, err := scheduler.PresetCron(body.Preset)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_preset")
			return
		}
		cronExpr = p
	}
	if err := scheduler.ValidateCron(cronExpr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_cron")
		return
	}
	tz := body.Timezone
	if tz == "" {
		tz = s.app.Settings.Get(settings.KeyTimezone)
	}
	sch := &store.Schedule{
		TaskID:   body.TaskID,
		Name:     body.Name,
		CronExpr: cronExpr,
		Timezone: tz,
		Enabled:  body.Enabled,
	}
	if err := s.app.Schedules.Create(r.Context(), sch); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	_ = s.app.Scheduler.Reload(r.Context())
	writeJSON(w, http.StatusCreated, sch)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	existing, err := s.app.Schedules.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var body struct {
		TaskID   string `json:"task_id"`
		Name     string `json:"name"`
		CronExpr string `json:"cron_expr"`
		Timezone string `json:"timezone"`
		Enabled  bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if err := scheduler.ValidateCron(body.CronExpr); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_cron")
		return
	}
	existing.TaskID = body.TaskID
	existing.Name = body.Name
	existing.CronExpr = body.CronExpr
	existing.Timezone = body.Timezone
	existing.Enabled = body.Enabled
	if err := s.app.Schedules.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	_ = s.app.Scheduler.Reload(r.Context())
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return
	}
	if _, err := s.app.Schedules.Get(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err := s.app.Schedules.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	_ = s.app.Scheduler.Reload(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Settings.PublicView())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	updates := map[string]string{}
	setStr := func(key, settingKey string) {
		if v, ok := body[key]; ok {
			if s, ok := v.(string); ok {
				updates[settingKey] = s
			}
		}
	}
	setBool := func(key, settingKey string) {
		if v, ok := body[key]; ok {
			if b, ok := v.(bool); ok {
				if b {
					updates[settingKey] = "true"
				} else {
					updates[settingKey] = "false"
				}
			}
		}
	}
	setStr("language", settings.KeyLanguage)
	setStr("theme", settings.KeyTheme)
	setStr("timezone", settings.KeyTimezone)
	setStr("zimaos_api_base_url", settings.KeyZimaOSAPIBaseURL)
	setBool("discord_enabled", settings.KeyDiscordEnabled)
	setBool("discord_notify_success", settings.KeyDiscordNotifyOK)
	setBool("discord_notify_failure", settings.KeyDiscordNotifyFail)
	setBool("compose_backup_enabled", settings.KeyComposeBackupEnabled)
	setStr("compose_backup_cron", settings.KeyComposeBackupCron)

	if v, ok := body["poll_interval_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			if n >= 2 && n <= 300 {
				updates[settings.KeyPollIntervalSec] = strconv.Itoa(int(n))
			}
		}
	}
	if v, ok := body["zimaos_api_token"]; ok {
		if token, ok := v.(string); ok {
			updates[settings.KeyZimaOSAPIToken] = token
		}
	}
	setStr("zimaos_username", settings.KeyZimaOSUsername)
	if v, ok := body["zimaos_password"]; ok {
		if password, ok := v.(string); ok && password != "" {
			updates[settings.KeyZimaOSPassword] = password
		}
	}
	if v, ok := body["discord_webhook_url"]; ok {
		if url, ok := v.(string); ok {
			updates[settings.KeyDiscordWebhookURL] = url
		}
	}
	if v, ok := body["compose_backup_keep"]; ok {
		switch n := v.(type) {
		case float64:
			if n >= 1 && n <= 90 {
				updates[settings.KeyComposeBackupKeep] = strconv.Itoa(int(n))
			}
		}
	}
	setStr("compose_backup_appdata_dir", settings.KeyComposeAppDataDir)
	setStr("compose_backup_appdata_host_root", settings.KeyComposeAppDataHostRoot)
	setStr("compose_backup_compose_dir", settings.KeyComposeMountDir)
	setStr("compose_backup_compose_host_root", settings.KeyComposeHostRoot)
	setStr("compose_backup_docker_sock", settings.KeyComposeDockerSock)

	if err := s.app.Settings.SetMany(r.Context(), updates); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	s.app.ZimaOS.Configure(
		s.app.Settings.Get(settings.KeyZimaOSAPIBaseURL),
		s.app.Settings.Get(settings.KeyZimaOSAPIToken),
		s.app.Settings.Get(settings.KeyZimaOSUsername),
		s.app.Settings.Get(settings.KeyZimaOSPassword),
	)
	_ = s.app.Scheduler.Reload(r.Context())
	_ = s.app.ComposeBackup.Reload(r.Context())
	writeJSON(w, http.StatusOK, s.app.Settings.PublicView())
}

func (s *Server) handleComposeBackupStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.ComposeBackup.Status())
}

func (s *Server) handleComposeBackupRun(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.ComposeBackup.Run(r.Context(), "", "manual")
	if err != nil {
		switch {
		case errors.Is(err, composebackup.ErrDisabled):
			writeError(w, http.StatusConflict, "compose_backup_disabled")
		case errors.Is(err, composebackup.ErrAppDataMissing):
			writeError(w, http.StatusFailedDependency, "compose_backup_appdata_missing")
		case errors.Is(err, composebackup.ErrDockerMissing):
			writeError(w, http.StatusFailedDependency, "compose_backup_docker_missing")
		case errors.Is(err, composebackup.ErrBusy):
			writeError(w, http.StatusConflict, "compose_backup_busy")
		default:
			writeError(w, http.StatusInternalServerError, "compose_backup_failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok_count":   res.OKCount,
		"fail_count": res.FailCount,
		"projects":   res.Projects,
		"created_at": res.CreatedAt.UTC().Format(time.RFC3339),
		"status":     s.app.ComposeBackup.Status(),
	})
}

func (s *Server) handleDiscordTest(w http.ResponseWriter, r *http.Request) {
	webhook := s.app.Settings.Get(settings.KeyDiscordWebhookURL)
	if webhook == "" {
		writeError(w, http.StatusBadRequest, "webhook_missing")
		return
	}
	loc, err := time.LoadLocation(s.app.Settings.Get(settings.KeyTimezone))
	if err != nil {
		loc = time.Local
	}
	if err := s.app.Discord.SendEmbed(r.Context(), webhook, discord.TestEmbed(loc)); err != nil {
		writeError(w, http.StatusBadGateway, "discord_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	connected, lastOK, lastErr := s.app.ZimaOS.Status()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        version.Version,
		"api_connected":  connected,
		"api_last_ok":    lastOK,
		"api_last_error": lastErr,
		"database_path":  s.app.Config.DatabasePath,
		"data_dir":       s.app.Config.DataDir,
	})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.app.Audit.List(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.app.Hub.Subscribe()
	defer s.app.Hub.Unsubscribe(ch)

	fmt.Fprintf(w, "event: connected\ndata: {\"ok\":true}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
