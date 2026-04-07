package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
)

type taskResponse struct {
	ID           int64       `json:"id"`
	InstanceID   int64       `json:"instance_id"`
	InstanceName string      `json:"instance_name"`
	BackupID     *int64      `json:"backup_id,omitempty"`
	Type         string      `json:"type"`
	Status       string      `json:"status"`
	Progress     int         `json:"progress"`
	CurrentStep  string      `json:"current_step"`
	StartedAt    *time.Time  `json:"started_at,omitempty"`
	CompletedAt  *time.Time  `json:"completed_at,omitempty"`
	EstimatedEnd *time.Time  `json:"estimated_end,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	tasks, err := h.db.ListActiveTasks()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list tasks")
		return
	}

	responses := make([]taskResponse, 0, len(tasks))
	instanceNames := make(map[int64]string, len(tasks))
	for _, task := range tasks {
		instanceName, err := h.instanceName(task.InstanceID, instanceNames)
		if err != nil {
			Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query instance")
			return
		}
		responses = append(responses, buildTaskResponse(task, instanceName))
	}

	JSON(w, http.StatusOK, map[string]any{"items": responses})
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	taskID, err := taskIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid task id")
		return
	}

	task, err := h.db.GetTaskByID(taskID)
	if err != nil {
		writeTaskError(w, err)
		return
	}

	if err := h.ensureTaskAccess(r, task.InstanceID); err != nil {
		writeTaskAccessError(w, err)
		return
	}

	instance, err := h.db.GetInstanceByID(task.InstanceID)
	if err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	JSON(w, http.StatusOK, buildTaskResponse(*task, instance.Name))
}

func (h *Handler) CancelTask(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	if h.taskQueue == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "task queue unavailable")
		return
	}

	taskID, err := taskIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid task id")
		return
	}

	if err := h.taskQueue.Cancel(taskID); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	task, err := h.db.GetTaskByID(taskID)
	if err != nil {
		writeTaskError(w, err)
		return
	}

	instance, err := h.db.GetInstanceByID(task.InstanceID)
	if err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"message": "task cancellation requested",
		"task":    buildTaskResponse(*task, instance.Name),
	})
}

func (h *Handler) ensureTaskAccess(r *http.Request, instanceID int64) error {
	claims := middleware.MustGetUser(r.Context())
	if claims.Role == "admin" {
		return nil
	}

	_, err := h.db.GetInstancePermission(claims.UserID, instanceID)
	return err
}

func (h *Handler) instanceName(instanceID int64, cache map[int64]string) (string, error) {
	if name, ok := cache[instanceID]; ok {
		return name, nil
	}

	instance, err := h.db.GetInstanceByID(instanceID)
	if err != nil {
		return "", err
	}
	cache[instanceID] = instance.Name
	return instance.Name, nil
}

func buildTaskResponse(task model.Task, instanceName string) taskResponse {
	return taskResponse{
		ID:           task.ID,
		InstanceID:   task.InstanceID,
		InstanceName: instanceName,
		BackupID:     task.BackupID,
		Type:         task.Type,
		Status:       task.Status,
		Progress:     task.Progress,
		CurrentStep:  task.CurrentStep,
		StartedAt:    task.StartedAt,
		CompletedAt:  task.CompletedAt,
		EstimatedEnd: task.EstimatedEnd,
		ErrorMessage: task.ErrorMessage,
		CreatedAt:    task.CreatedAt,
	}
}

func writeTaskError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusNotFound, 40406, "task not found")
		return
	}

	Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query task")
}

func writeTaskAccessError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusForbidden, 40301, "forbidden")
		return
	}

	Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query instance permission")
}

func taskIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("task id is required")
	}

	taskID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse task id %q: %w", rawID, err)
	}
	if taskID <= 0 {
		return 0, fmt.Errorf("task id must be positive")
	}

	return taskID, nil
}