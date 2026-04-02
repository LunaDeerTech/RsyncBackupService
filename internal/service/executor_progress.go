package service

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

type ProgressEvent struct {
	TaskID        string  `json:"task_id"`
	InstanceID    uint    `json:"instance_id"`
	Percentage    float64 `json:"percentage"`
	SpeedText     string  `json:"speed_text"`
	RemainingText string  `json:"remaining_text"`
	Status        string  `json:"status"`
}

type RunningTaskStatus struct {
	TaskID          string  `json:"task_id"`
	InstanceID      uint    `json:"instance_id"`
	StorageTargetID uint    `json:"storage_target_id"`
	StartedAt       string  `json:"started_at"`
	Percentage      float64 `json:"percentage"`
	SpeedText       string  `json:"speed_text"`
	RemainingText   string  `json:"remaining_text"`
	Status          string  `json:"status"`
}

func (s *ExecutorService) initProgressBus() {
	if s == nil {
		return
	}
	if s.progressSubscribers == nil {
		s.progressSubscribers = make(map[int]func(ProgressEvent))
	}
	if s.progressState == nil {
		s.progressState = make(map[string]ProgressEvent)
	}
}

func (s *ExecutorService) SubscribeProgress(handler func(ProgressEvent)) func() {
	if s == nil || handler == nil {
		return func() {}
	}
	s.initProgressBus()

	s.progressMu.Lock()
	s.nextProgressSubscriberID++
	subscriberID := s.nextProgressSubscriberID
	s.progressSubscribers[subscriberID] = handler
	s.progressMu.Unlock()

	return func() {
		s.progressMu.Lock()
		delete(s.progressSubscribers, subscriberID)
		s.progressMu.Unlock()
	}
}

func (s *ExecutorService) PublishProgress(event ProgressEvent) {
	if s == nil {
		return
	}
	s.initProgressBus()

	event = normalizeProgressEvent(event)
	if event.TaskID == "" {
		return
	}

	s.progressMu.Lock()
	if isTerminalProgressStatus(event.Status) {
		delete(s.progressState, event.TaskID)
	} else {
		s.progressState[event.TaskID] = event
	}
	subscribers := make([]func(ProgressEvent), 0, len(s.progressSubscribers))
	for _, subscriber := range s.progressSubscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.progressMu.Unlock()

	for _, subscriber := range subscribers {
		subscriber(event)
	}
}

func (s *ExecutorService) ListRunningTasks() []RunningTaskStatus {
	if s == nil || s.taskManager == nil {
		return nil
	}

	tasks := s.taskManager.List()
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()

	results := make([]RunningTaskStatus, 0, len(tasks))
	for _, task := range tasks {
		progress, exists := s.progressState[task.ID]
		if !exists {
			progress = ProgressEvent{
				TaskID:     task.ID,
				InstanceID: task.InstanceID,
				Status:     model.BackupStatusRunning,
			}
		}
		if progress.InstanceID == 0 {
			progress.InstanceID = task.InstanceID
		}

		results = append(results, RunningTaskStatus{
			TaskID:          task.ID,
			InstanceID:      task.InstanceID,
			StorageTargetID: task.StorageTargetID,
			StartedAt:       task.StartedAt.UTC().Format(http.TimeFormat),
			Percentage:      progress.Percentage,
			SpeedText:       progress.SpeedText,
			RemainingText:   progress.RemainingText,
			Status:          progress.Status,
		})
	}

	return results
}

func (s *ExecutorService) CancelTask(taskID string) error {
	if s == nil || s.taskManager == nil {
		return executorpkg.ErrTaskNotFound
	}

	return s.taskManager.Cancel(strings.TrimSpace(taskID))
}

func progressEventFromSnapshot(taskID string, instanceID uint, snapshot executorpkg.ProgressSnapshot, status string) ProgressEvent {
	speed := snapshot.AverageBytesPerSecond
	if speed <= 0 {
		speed = snapshot.BytesPerSecond
	}

	return normalizeProgressEvent(ProgressEvent{
		TaskID:        taskID,
		InstanceID:    instanceID,
		Percentage:    float64(snapshot.Percentage),
		SpeedText:     formatSpeedText(speed),
		RemainingText: formatRemainingText(snapshot.EstimatedRemaining),
		Status:        status,
	})
}

func normalizeProgressEvent(event ProgressEvent) ProgressEvent {
	event.TaskID = strings.TrimSpace(event.TaskID)
	event.SpeedText = strings.TrimSpace(event.SpeedText)
	event.RemainingText = strings.TrimSpace(event.RemainingText)
	event.Status = strings.TrimSpace(event.Status)
	if event.Status == "" {
		event.Status = model.BackupStatusRunning
	}
	if event.Status == model.BackupStatusSuccess {
		event.Percentage = 100
	}
	if event.Percentage < 0 {
		event.Percentage = 0
	}
	if event.Percentage > 100 {
		event.Percentage = 100
	}

	return event
}

func isTerminalProgressStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case model.BackupStatusSuccess, model.BackupStatusFailed, model.BackupStatusCancelled:
		return true
	default:
		return false
	}
}

func formatSpeedText(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 || math.IsNaN(bytesPerSecond) || math.IsInf(bytesPerSecond, 0) {
		return ""
	}

	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	value := bytesPerSecond
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}

	format := "%.2f %s"
	if unitIndex == 0 || value >= 100 {
		format = "%.0f %s"
	} else if value >= 10 {
		format = "%.1f %s"
	}

	return fmt.Sprintf(format, value, units[unitIndex])
}

func formatRemainingText(remaining time.Duration) string {
	if remaining <= 0 {
		return ""
	}

	remaining = remaining.Round(time.Second)
	hours := int(remaining / time.Hour)
	minutes := int((remaining % time.Hour) / time.Minute)
	seconds := int((remaining % time.Minute) / time.Second)

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
