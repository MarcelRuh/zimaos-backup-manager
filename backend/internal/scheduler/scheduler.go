package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/control"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/settings"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/store"
)

// Scheduler runs cron expressions without catch-up of missed executions.
// robfig/cron v3 with default options does not backfill missed jobs after downtime.
type Scheduler struct {
	schedules *store.ScheduleStore
	control   *control.Service
	settings  *settings.Store
	audit     *store.AuditStore
	logger    *slog.Logger

	mu     sync.Mutex
	cron   *cron.Cron
	entryIDs map[int64]cron.EntryID
}

func New(schedules *store.ScheduleStore, ctrl *control.Service, settingsStore *settings.Store, audit *store.AuditStore, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		schedules: schedules,
		control:   ctrl,
		settings:  settingsStore,
		audit:     audit,
		logger:    logger,
		entryIDs:  map[int64]cron.EntryID{},
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	return s.Reload(ctx)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
	s.entryIDs = map[int64]cron.EntryID{}
}

func (s *Scheduler) Reload(ctx context.Context) error {
	list, err := s.schedules.List(ctx)
	if err != nil {
		return err
	}

	tzName := s.settings.Get(settings.KeyTimezone)
	if tzName == "" {
		tzName = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		loc = time.Local
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
	}
	s.cron = cron.New(cron.WithLocation(loc), cron.WithParser(cron.NewParser(
		cron.Minute|cron.Hour|cron.Dom|cron.Month|cron.Dow|cron.Descriptor,
	)))
	s.entryIDs = map[int64]cron.EntryID{}

	for _, sch := range list {
		if !sch.Enabled {
			continue
		}
		sch := sch
		id, err := s.cron.AddFunc(sch.CronExpr, func() {
			s.trigger(sch)
		})
		if err != nil {
			s.logger.Error("invalid schedule", "id", sch.ID, "cron", sch.CronExpr, "error", err)
			_ = s.audit.Add(context.Background(), "error", "scheduler", "invalid cron expression", map[string]any{
				"schedule_id": sch.ID, "cron": sch.CronExpr, "error": err.Error(),
			})
			continue
		}
		s.entryIDs[sch.ID] = id
	}
	s.cron.Start()
	return nil
}

func (s *Scheduler) trigger(sch store.Schedule) {
	// Allow compose→AppData before backup (can take a couple of minutes).
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	now := time.Now().UTC()
	_ = s.schedules.MarkTriggered(ctx, sch.ID, now)
	_, msg, err := s.control.StartBackup(ctx, sch.TaskID)
	if err != nil {
		s.logger.Warn("scheduled start failed", "schedule_id", sch.ID, "task_id", sch.TaskID, "error", err)
		_ = s.audit.Add(ctx, "warn", "scheduler", "scheduled backup start failed", map[string]any{
			"schedule_id": sch.ID, "task_id": sch.TaskID, "error": err.Error(),
		})
		return
	}
	_ = s.audit.Add(ctx, "info", "scheduler", "scheduled backup started", map[string]any{
		"schedule_id": sch.ID, "task_id": sch.TaskID, "message": msg,
	})
}

func PresetCron(preset string) (string, error) {
	switch preset {
	case "daily":
		return "0 3 * * *", nil
	case "weekly":
		return "0 3 * * 1", nil
	case "monthly":
		return "0 3 1 * *", nil
	default:
		return "", fmt.Errorf("unknown preset %q", preset)
	}
}

func ValidateCron(expr string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(expr)
	return err
}
