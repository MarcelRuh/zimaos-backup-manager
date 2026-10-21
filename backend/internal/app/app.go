package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/auth"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/composebackup"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/control"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/db"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/discord"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/events"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/monitor"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/scheduler"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

type App struct {
	Config        config.Config
	Logger        *slog.Logger
	DB            *db.DB
	Settings      *settings.Store
	Auth          *auth.Service
	ZimaOS        *zimaos.Client
	Runs          *store.RunStore
	Prefs         *store.PrefsStore
	Schedules     *store.ScheduleStore
	Audit         *store.AuditStore
	Monitor       *monitor.Service
	Control       *control.Service
	Scheduler     *scheduler.Scheduler
	ComposeBackup *composebackup.Service
	Discord       *discord.Client
	Hub           *events.Hub
}

func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	settingsStore, err := settings.NewStore(database.SQL, cfg)
	if err != nil {
		_ = database.Close()
		return nil, err
	}

	hub := events.NewHub()
	zClient := zimaos.NewClient(settingsStore.Get(settings.KeyZimaOSAPIBaseURL), settingsStore.Get(settings.KeyZimaOSAPIToken))
	zClient.Configure(
		settingsStore.Get(settings.KeyZimaOSAPIBaseURL),
		settingsStore.Get(settings.KeyZimaOSAPIToken),
		settingsStore.Get(settings.KeyZimaOSUsername),
		settingsStore.Get(settings.KeyZimaOSPassword),
	)
	zClient.SetTokenPersister(func(ctx context.Context, token string) error {
		return settingsStore.Set(ctx, settings.KeyZimaOSAPIToken, token)
	})
	runs := store.NewRunStore(database.SQL)
	prefs := store.NewPrefsStore(database.SQL)
	schedules := store.NewScheduleStore(database.SQL)
	audit := store.NewAuditStore(database.SQL)
	discordClient := discord.NewClient()
	authSvc := auth.NewService(database.SQL)

	composeBackup := composebackup.New(composebackup.Paths{
		DockerSock:      cfg.DockerSock,
		AppDataDir:      cfg.AppDataDir,
		AppDataHostRoot: cfg.AppDataHostRoot,
		ComposeDir:      cfg.ComposeDir,
		ComposeHostRoot: cfg.ComposeHostRoot,
	}, settingsStore, audit, discordClient, logger)
	mon := monitor.NewService(zClient, runs, prefs, audit, settingsStore, discordClient, hub, logger, composeBackup)
	ctrl := control.NewService(zClient, mon, runs, prefs, audit, composeBackup)
	sched := scheduler.New(schedules, ctrl, settingsStore, audit, logger)

	a := &App{
		Config:        cfg,
		Logger:        logger,
		DB:            database,
		Settings:      settingsStore,
		Auth:          authSvc,
		ZimaOS:        zClient,
		Runs:          runs,
		Prefs:         prefs,
		Schedules:     schedules,
		Audit:         audit,
		Monitor:       mon,
		Control:       ctrl,
		Scheduler:     sched,
		ComposeBackup: composeBackup,
		Discord:       discordClient,
		Hub:           hub,
	}

	mon.Start()
	if err := sched.Start(context.Background()); err != nil {
		logger.Warn("scheduler start failed", "error", err)
	}
	if err := composeBackup.Start(context.Background()); err != nil {
		logger.Warn("compose backup scheduler start failed", "error", err)
	}
	_ = audit.Add(context.Background(), "info", "system", "application started", map[string]any{})
	return a, nil
}

func (a *App) Stop() {
	a.ComposeBackup.Stop()
	a.Scheduler.Stop()
	a.Monitor.Stop()
}

func (a *App) Close() {
	a.Stop()
	_ = a.DB.Close()
}
