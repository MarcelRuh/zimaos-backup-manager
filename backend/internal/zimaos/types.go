package zimaos

import "strings"

// Device describes a source or target storage location returned by ZimaOS.
type Device struct {
	DeviceType    string `json:"device_type"`
	DeviceSubtype string `json:"device_subtype"`
	IsConnected   bool   `json:"is_connected"`
	Name          string `json:"name"`
	Path          string `json:"path"`
}

// Subtask is a backup destination/run slice under a task.
type Subtask struct {
	ID            string  `json:"id"`
	ScheduleType  string  `json:"schedule_type"`
	Status        string  `json:"status"`
	Progress      float64 `json:"progress"`
	Speed         float64 `json:"speed"`
	TotalFiles    int64   `json:"total_files"`
	XferredFiles  int64   `json:"xferred_files"`
	TotalSize     int64   `json:"total_size"`
	XferredSize   int64   `json:"xferred_size"`
	UpdatedAt     int64   `json:"updated_at"`
	ErrorMessage  string  `json:"error_message"`
	Target        Device  `json:"target"`
}

// Task is a ZimaOS backup task.
type Task struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	LastBackupTime int64     `json:"last_backup_time"`
	Source         Device    `json:"source"`
	Subtasks       []Subtask `json:"subtasks"`
}

type tasksResponse struct {
	Data    []Task `json:"data"`
	Message string `json:"message"`
}

type startResponse struct {
	Message string `json:"message"`
}

func NormalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func IsRunningStatus(status string) bool {
	switch NormalizeStatus(status) {
	case "running", "in_progress", "in-progress", "progressing", "active", "backing_up", "syncing":
		return true
	default:
		return false
	}
}

func IsCompletedStatus(status string) bool {
	switch NormalizeStatus(status) {
	case "completed", "complete", "success", "succeeded", "done", "finished":
		return true
	default:
		return false
	}
}

func IsFailedStatus(status string) bool {
	switch NormalizeStatus(status) {
	case "failed", "error", "errored", "cancelled", "canceled", "aborted":
		return true
	default:
		return false
	}
}

func IsIdleStatus(status string) bool {
	switch NormalizeStatus(status) {
	case "", "idle", "pending", "waiting", "manual", "none", "not_started", "never":
		return true
	default:
		return false
	}
}

func (t Task) AggregateProgress() float64 {
	if len(t.Subtasks) == 0 {
		return 0
	}
	var sum float64
	for _, st := range t.Subtasks {
		sum += st.Progress
	}
	return sum / float64(len(t.Subtasks))
}

func (t Task) AggregateFiles() (total, xferred int64) {
	for _, st := range t.Subtasks {
		total += st.TotalFiles
		xferred += st.XferredFiles
	}
	return
}

func (t Task) AggregateSize() (total, xferred int64) {
	for _, st := range t.Subtasks {
		total += st.TotalSize
		xferred += st.XferredSize
	}
	return
}

func (t Task) PrimaryTarget() Device {
	if len(t.Subtasks) == 0 {
		return Device{}
	}
	return t.Subtasks[0].Target
}

func (t Task) AnyTargetDisconnected() bool {
	for _, st := range t.Subtasks {
		if !st.Target.IsConnected {
			return true
		}
	}
	return false
}

func (t Task) CombinedError() string {
	var parts []string
	for _, st := range t.Subtasks {
		if msg := strings.TrimSpace(st.ErrorMessage); msg != "" {
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "; ")
}

func (t Task) EffectiveRunning() bool {
	if IsRunningStatus(t.Status) {
		return true
	}
	for _, st := range t.Subtasks {
		if IsRunningStatus(st.Status) {
			return true
		}
	}
	return false
}

func (t Task) EffectiveFailed() bool {
	if IsFailedStatus(t.Status) {
		return true
	}
	for _, st := range t.Subtasks {
		if IsFailedStatus(st.Status) {
			return true
		}
	}
	return false
}

func (t Task) EffectiveCompleted() bool {
	if t.EffectiveRunning() || t.EffectiveFailed() {
		return false
	}
	if IsCompletedStatus(t.Status) {
		return true
	}
	if len(t.Subtasks) == 0 {
		return false
	}
	for _, st := range t.Subtasks {
		if !IsCompletedStatus(st.Status) {
			return false
		}
	}
	return true
}
