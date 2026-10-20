package control

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/db"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/discord"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/events"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/monitor"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
)

func TestStartBackupAndRejectDoubleStart(t *testing.T) {
	status := "idle"
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []zimaos.Task{{
				ID: "4", Name: "AppData", Status: status, LastBackupTime: 10,
				Source: DeviceConnected("src"),
				Subtasks: []zimaos.Subtask{{
					ID: "4", Status: status, Progress: 0,
					Target: DeviceConnected("dst"),
				}},
			}},
		})
	})
	mux.HandleFunc("/v2/backup/tasks/4/start", func(w http.ResponseWriter, r *http.Request) {
		status = "running"
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "task start completed successfully: 1/1 subtasks started"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	cfg := config.Config{DataDir: dir, Timezone: "Europe/Berlin", ZimaOSAPIBaseURL: srv.URL}
	settingsStore, err := settings.NewStore(database.SQL, cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = settingsStore.Set(context.Background(), settings.KeyZimaOSAPIBaseURL, srv.URL)

	client := zimaos.NewClient(srv.URL, "")
	runs := store.NewRunStore(database.SQL)
	prefs := store.NewPrefsStore(database.SQL)
	audit := store.NewAuditStore(database.SQL)
	hub := events.NewHub()
	mon := monitor.NewService(client, runs, prefs, audit, settingsStore, discord.NewClient(), hub, slog.Default())
	ctrl := NewService(client, mon, runs, prefs, audit)

	run, msg, err := ctrl.StartBackup(context.Background(), "4")
	if err != nil || run == nil || msg == "" {
		t.Fatalf("start failed: %v", err)
	}
	_, _, err = ctrl.StartBackup(context.Background(), "4")
	if err != ErrAlreadyRunning {
		t.Fatalf("expected already running, got %v", err)
	}
}

func DeviceConnected(path string) zimaos.Device {
	return zimaos.Device{Name: "Zima", Path: path, IsConnected: true, DeviceType: "Zima"}
}
