package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/api"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/app"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/version"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	application, err := app.New(cfg, logger)
	if err != nil {
		slog.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(application),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0, // SSE streams need unbounded write timeout
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server",
			"addr", cfg.ListenAddr,
			"version", version.Version,
			"data_dir", cfg.DataDir,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	application.Stop()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
