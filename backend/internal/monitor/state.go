package monitor

import (
	"strings"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/domain"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

type Result = domain.Result

const (
	ResultSuccess        = domain.ResultSuccess
	ResultFailed         = domain.ResultFailed
	ResultRunning        = domain.ResultRunning
	ResultUnknown        = domain.ResultUnknown
	ResultConnectionLost = domain.ResultConnectionLost
	ResultAmbiguous      = domain.ResultAmbiguous
)

type DurationQuality = domain.DurationQuality

const (
	DurationMeasured  = domain.DurationMeasured
	DurationEstimated = domain.DurationEstimated
)

// Observation is one poll snapshot used by the state machine.
type Observation struct {
	Task               zimaos.Task
	APIReachable       bool
	ObservedAtUnix     int64
	BaselineLastBackup int64
	SawRunning         bool
	StartedAtUnix      int64
}

// Transition decides the next run result from the latest observation.
func Transition(obs Observation) Result {
	if !obs.APIReachable {
		return ResultConnectionLost
	}
	task := obs.Task

	if task.EffectiveRunning() {
		return ResultRunning
	}
	if task.EffectiveFailed() {
		return ResultFailed
	}

	lastBackupAdvanced := task.LastBackupTime > 0 && task.LastBackupTime > obs.BaselineLastBackup
	completed := task.EffectiveCompleted()

	if completed && obs.SawRunning && lastBackupAdvanced {
		return ResultSuccess
	}
	if completed && obs.SawRunning && !lastBackupAdvanced {
		return ResultSuccess
	}
	if completed && !obs.SawRunning && lastBackupAdvanced {
		if obs.StartedAtUnix > 0 && task.LastBackupTime >= obs.StartedAtUnix-5 {
			return ResultSuccess
		}
		return ResultAmbiguous
	}
	if completed && !obs.SawRunning && !lastBackupAdvanced {
		return ResultUnknown
	}

	status := zimaos.NormalizeStatus(task.Status)
	if status == "" || zimaos.IsIdleStatus(status) {
		return ResultUnknown
	}

	if strings.Contains(status, "fail") || strings.Contains(status, "error") {
		return ResultFailed
	}
	if strings.Contains(status, "run") || strings.Contains(status, "progress") {
		return ResultRunning
	}
	return ResultUnknown
}

func IsTerminal(r Result) bool       { return domain.IsTerminal(r) }
func IsNotifySuccess(r Result) bool  { return domain.IsNotifySuccess(r) }
func IsNotifyFailure(r Result) bool  { return domain.IsNotifyFailure(r) }
