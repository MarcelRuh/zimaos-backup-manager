package monitor

import (
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/zimaos"
)

func TestTransitionStaleCompletedNotSuccess(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:         "completed",
			LastBackupTime: 100,
			Subtasks:       []zimaos.Subtask{{Status: "completed", Progress: 100}},
		},
		APIReachable:       true,
		BaselineLastBackup: 100,
		SawRunning:         false,
		StartedAtUnix:      200,
	}
	if got := Transition(obs); got != ResultUnknown {
		t.Fatalf("expected unknown for stale completed, got %s", got)
	}
}

func TestTransitionSuccessAfterRunning(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:         "completed",
			LastBackupTime: 300,
			Subtasks:       []zimaos.Subtask{{Status: "completed", Progress: 100}},
		},
		APIReachable:       true,
		BaselineLastBackup: 100,
		SawRunning:         true,
		StartedAtUnix:      200,
	}
	if got := Transition(obs); got != ResultSuccess {
		t.Fatalf("expected success, got %s", got)
	}
}

func TestTransitionRunning(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:   "running",
			Subtasks: []zimaos.Subtask{{Status: "running", Progress: 40}},
		},
		APIReachable: true,
		SawRunning:   true,
	}
	if got := Transition(obs); got != ResultRunning {
		t.Fatalf("expected running, got %s", got)
	}
}

func TestTransitionFailed(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:   "failed",
			Subtasks: []zimaos.Subtask{{Status: "failed", ErrorMessage: "disk full"}},
		},
		APIReachable: true,
		SawRunning:   true,
	}
	if got := Transition(obs); got != ResultFailed {
		t.Fatalf("expected failed, got %s", got)
	}
}

func TestTransitionConnectionLost(t *testing.T) {
	obs := Observation{APIReachable: false}
	if got := Transition(obs); got != ResultConnectionLost {
		t.Fatalf("expected connection_lost, got %s", got)
	}
}

func TestTransitionUnknownStatusSafe(t *testing.T) {
	obs := Observation{
		Task:         zimaos.Task{Status: "weird-new-status-xyz"},
		APIReachable: true,
	}
	if got := Transition(obs); got != ResultUnknown {
		t.Fatalf("expected unknown, got %s", got)
	}
}

func TestShortBackupWithoutSawRunningButAdvancedTimestamp(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:         "completed",
			LastBackupTime: 250,
			Subtasks:       []zimaos.Subtask{{Status: "completed"}},
		},
		APIReachable:       true,
		BaselineLastBackup: 100,
		SawRunning:         false,
		StartedAtUnix:      240,
	}
	if got := Transition(obs); got != ResultSuccess {
		t.Fatalf("expected success for short run, got %s", got)
	}
}

func TestTransitionAmbiguousWithoutSawRunning(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status:         "completed",
			LastBackupTime: 50,
			Subtasks:       []zimaos.Subtask{{Status: "completed"}},
		},
		APIReachable:       true,
		BaselineLastBackup: 10,
		SawRunning:         false,
		StartedAtUnix:      200,
	}
	if got := Transition(obs); got != ResultAmbiguous {
		t.Fatalf("expected ambiguous, got %s", got)
	}
}

func TestTransitionMultipleSubtasksOneRunning(t *testing.T) {
	obs := Observation{
		Task: zimaos.Task{
			Status: "running",
			Subtasks: []zimaos.Subtask{
				{Status: "completed", Progress: 100},
				{Status: "running", Progress: 40},
			},
		},
		APIReachable: true,
		SawRunning:   true,
	}
	if got := Transition(obs); got != ResultRunning {
		t.Fatalf("expected running with mixed subtasks, got %s", got)
	}
}
