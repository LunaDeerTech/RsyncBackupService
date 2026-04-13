package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
)

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

type v2InstanceListItem struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	RemoteConfigID   *int64     `json:"remote_config_id,omitempty"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastBackupTime   *time.Time `json:"last_backup_time,omitempty"`
	LastBackupStatus *string    `json:"last_backup_status,omitempty"`
	BackupCount      int64      `json:"backup_count"`
	DRScore          *float64   `json:"dr_score,omitempty"`
	DRLevel          string     `json:"dr_level,omitempty"`
}

type v2BackupResponse struct {
	ID              int64      `json:"id"`
	InstanceID      int64      `json:"instance_id"`
	PolicyID        int64      `json:"policy_id"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	BackupSizeBytes int64      `json:"backup_size_bytes"`
	ActualSizeBytes int64      `json:"actual_size_bytes"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds int64      `json:"duration_seconds"`
	ErrorMessage    string     `json:"error_message,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type v2OverviewInstance struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	ExcludePatterns []string  `json:"exclude_patterns"`
	RemoteConfigID  *int64    `json:"remote_config_id,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type v2InstanceStats struct {
	BackupCount          int64                    `json:"backup_count"`
	SuccessBackupCount   int64                    `json:"success_backup_count"`
	FailureBackupCount   int64                    `json:"failure_backup_count"`
	TotalBackupSizeBytes int64                    `json:"total_backup_size_bytes"`
	TotalBackupDiskBytes int64                    `json:"total_backup_disk_bytes"`
	PolicyCount          int64                    `json:"policy_count"`
	LastBackup           *v2BackupResponse        `json:"last_backup,omitempty"`
	RecentTrend          []model.BackupTrendPoint `json:"recent_trend"`
}

type v2InstanceOverviewResponse struct {
	Instance   v2OverviewInstance `json:"instance"`
	Stats      v2InstanceStats    `json:"stats"`
	Permission string             `json:"permission,omitempty"`
}

func (h *Handler) ListV2Instances(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	claims := middleware.MustGetUser(r.Context())
	pagination := ParsePagination(r)
	offset := (pagination.Page - 1) * pagination.PageSize

	var (
		instances []model.Instance
		total     int64
		err       error
	)

	if claims.Role == "admin" {
		total, err = h.db.CountInstances()
		if err == nil {
			instances, err = h.db.ListInstancesPage(pagination.PageSize, offset)
		}
	} else {
		total, err = h.db.CountInstancesByUserPermission(claims.UserID)
		if err == nil {
			instances, err = h.db.ListInstancesByUserPermissionPage(claims.UserID, pagination.PageSize, offset)
		}
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list instances")
		return
	}

	items, err := h.buildV2InstanceListItems(instances)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load instance stats")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(total, pagination.PageSize),
	})
}

func (h *Handler) GetV2InstanceOverview(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	instance, err := h.db.GetInstanceByID(instanceID)
	if err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	stats, err := h.db.GetInstanceStats(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load instance stats")
		return
	}

	permission := ""
	claims := middleware.GetUser(r.Context())
	if claims != nil && claims.Role != "admin" {
		perm, err := h.db.GetInstancePermission(claims.UserID, instanceID)
		if err == nil {
			permission = perm.Permission
		}
	}

	JSON(w, http.StatusOK, buildV2InstanceOverviewResponse(*instance, *stats, permission))
}

func (h *Handler) GetV2InstanceCurrentTask(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	task, err := h.db.GetCurrentTaskByInstance(instanceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			JSON(w, http.StatusOK, map[string]any{"task": nil})
			return
		}
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query current task")
		return
	}

	instance, err := h.db.GetInstanceByID(instanceID)
	if err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"task": buildTaskResponse(*task, instance.Name)})
}

func (h *Handler) ListV2InstanceBackups(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	pagination := ParsePagination(r)
	total, err := h.db.CountBackupsByInstance(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count backups")
		return
	}

	backups, err := h.db.ListBackupsByInstance(instanceID, pagination.PageSize, (pagination.Page-1)*pagination.PageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list backups")
		return
	}

	items := make([]v2BackupResponse, 0, len(backups))
	for _, backup := range backups {
		items = append(items, buildV2BackupResponse(backup))
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      items,
		Total:      int64(total),
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(int64(total), pagination.PageSize),
	})
}

func (h *Handler) GetV2InstancePlan(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	if h.scheduler == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "scheduler unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	upcoming := h.scheduler.GetUpcomingTasks(v2UpcomingWindowFromRequest(r))
	items := make([]engine.UpcomingTask, 0, len(upcoming))
	for _, task := range upcoming {
		if task.InstanceID == instanceID {
			items = append(items, task)
		}
	}

	JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetV2DisasterRecoveryScore(w http.ResponseWriter, r *http.Request) {
	h.GetDisasterRecoveryScore(w, r)
}

func (h *Handler) buildV2InstanceListItems(instances []model.Instance) ([]v2InstanceListItem, error) {
	items := make([]v2InstanceListItem, 0, len(instances))
	for _, instance := range instances {
		stats, err := h.db.GetInstanceStats(instance.ID)
		if err != nil {
			return nil, err
		}

		item := v2InstanceListItem{
			ID:             instance.ID,
			Name:           instance.Name,
			RemoteConfigID: cloneOptionalInt64(instance.RemoteConfigID),
			Status:         instance.Status,
			CreatedAt:      instance.CreatedAt,
			UpdatedAt:      instance.UpdatedAt,
			BackupCount:    stats.BackupCount,
		}
		if stats.LastBackup != nil {
			lastBackupStatus := stats.LastBackup.Status
			item.LastBackupStatus = &lastBackupStatus
			item.LastBackupTime = backupOccurredAt(stats.LastBackup)
		}
		if h.disasterRecovery != nil {
			if score, err := h.disasterRecovery.GetScore(context.Background(), instance.ID); err == nil && score != nil {
				total := score.Total
				item.DRScore = &total
				item.DRLevel = score.Level
			}
		}
		items = append(items, item)
	}

	return items, nil
}

func buildV2InstanceOverviewResponse(instance model.Instance, stats model.InstanceStats, permission string) v2InstanceOverviewResponse {
	return v2InstanceOverviewResponse{
		Instance: v2OverviewInstance{
			ID:              instance.ID,
			Name:            instance.Name,
			ExcludePatterns: append([]string(nil), instance.ExcludePatterns...),
			RemoteConfigID:  cloneOptionalInt64(instance.RemoteConfigID),
			Status:          instance.Status,
			CreatedAt:       instance.CreatedAt,
			UpdatedAt:       instance.UpdatedAt,
		},
		Stats:      buildV2InstanceStats(stats),
		Permission: permission,
	}
}

func buildV2InstanceStats(stats model.InstanceStats) v2InstanceStats {
	response := v2InstanceStats{
		BackupCount:          stats.BackupCount,
		SuccessBackupCount:   stats.SuccessBackupCount,
		FailureBackupCount:   stats.FailureBackupCount,
		TotalBackupSizeBytes: stats.TotalBackupSizeBytes,
		TotalBackupDiskBytes: stats.TotalBackupDiskBytes,
		PolicyCount:          stats.PolicyCount,
		RecentTrend:          append([]model.BackupTrendPoint(nil), stats.RecentTrend...),
	}
	if stats.LastBackup != nil {
		lastBackup := buildV2BackupResponse(*stats.LastBackup)
		response.LastBackup = &lastBackup
	}

	return response
}

func buildV2BackupResponse(backup model.Backup) v2BackupResponse {
	return v2BackupResponse{
		ID:              backup.ID,
		InstanceID:      backup.InstanceID,
		PolicyID:        backup.PolicyID,
		Type:            backup.Type,
		Status:          backup.Status,
		BackupSizeBytes: backup.BackupSizeBytes,
		ActualSizeBytes: backup.ActualSizeBytes,
		StartedAt:       backup.StartedAt,
		CompletedAt:     backup.CompletedAt,
		DurationSeconds: backup.DurationSeconds,
		ErrorMessage:    backup.ErrorMessage,
		CreatedAt:       backup.CreatedAt,
	}
}

func v2UpcomingWindowFromRequest(r *http.Request) time.Duration {
	within := 720 * time.Hour
	if r == nil {
		return within
	}

	if value := r.URL.Query().Get("within_hours"); value != "" {
		hours, err := strconv.Atoi(value)
		if err == nil && hours > 0 && hours <= 720 {
			return time.Duration(hours) * time.Hour
		}
	}

	return within
}

func openAPISchemas() map[string]any {
	return map[string]any{
		"ApiEnvelope": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"code":    map[string]any{"type": "integer"},
				"message": map[string]any{"type": "string"},
				"data":    map[string]any{"nullable": true},
			},
		},
		"InstanceListItem": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                 map[string]any{"type": "integer"},
				"name":               map[string]any{"type": "string"},
				"remote_config_id":   map[string]any{"type": "integer", "nullable": true},
				"status":             map[string]any{"type": "string"},
				"created_at":         map[string]any{"type": "string", "format": "date-time"},
				"updated_at":         map[string]any{"type": "string", "format": "date-time"},
				"last_backup_time":   map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"last_backup_status": map[string]any{"type": "string", "nullable": true},
				"backup_count":       map[string]any{"type": "integer"},
				"dr_score":           map[string]any{"type": "number", "nullable": true},
				"dr_level":           map[string]any{"type": "string"},
			},
		},
		"PaginatedInstances": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/InstanceListItem"}},
				"total":       map[string]any{"type": "integer"},
				"page":        map[string]any{"type": "integer"},
				"page_size":   map[string]any{"type": "integer"},
				"total_pages": map[string]any{"type": "integer"},
			},
		},
		"Backup": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":                map[string]any{"type": "integer"},
				"instance_id":       map[string]any{"type": "integer"},
				"policy_id":         map[string]any{"type": "integer"},
				"type":              map[string]any{"type": "string"},
				"status":            map[string]any{"type": "string"},
				"backup_size_bytes": map[string]any{"type": "integer"},
				"actual_size_bytes": map[string]any{"type": "integer"},
				"started_at":        map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"completed_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"duration_seconds":  map[string]any{"type": "integer"},
				"error_message":     map[string]any{"type": "string"},
				"created_at":        map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"PaginatedBackups": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Backup"}},
				"total":       map[string]any{"type": "integer"},
				"page":        map[string]any{"type": "integer"},
				"page_size":   map[string]any{"type": "integer"},
				"total_pages": map[string]any{"type": "integer"},
			},
		},
		"Task": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":            map[string]any{"type": "integer"},
				"instance_id":   map[string]any{"type": "integer"},
				"instance_name": map[string]any{"type": "string"},
				"backup_id":     map[string]any{"type": "integer", "nullable": true},
				"type":          map[string]any{"type": "string"},
				"status":        map[string]any{"type": "string"},
				"progress":      map[string]any{"type": "integer"},
				"current_step":  map[string]any{"type": "string"},
				"started_at":    map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"completed_at":  map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"estimated_end": map[string]any{"type": "string", "format": "date-time", "nullable": true},
				"error_message": map[string]any{"type": "string"},
				"created_at":    map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"CurrentTaskResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{"allOf": []any{map[string]any{"$ref": "#/components/schemas/Task"}}, "nullable": true},
			},
		},
		"OverviewInstance": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":               map[string]any{"type": "integer"},
				"name":             map[string]any{"type": "string"},
				"exclude_patterns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"remote_config_id": map[string]any{"type": "integer", "nullable": true},
				"status":           map[string]any{"type": "string"},
				"created_at":       map[string]any{"type": "string", "format": "date-time"},
				"updated_at":       map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"BackupTrendPoint": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"date":          map[string]any{"type": "string"},
				"count":         map[string]any{"type": "integer"},
				"success_count": map[string]any{"type": "integer"},
				"failure_count": map[string]any{"type": "integer"},
			},
		},
		"InstanceStats": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"backup_count":            map[string]any{"type": "integer"},
				"success_backup_count":    map[string]any{"type": "integer"},
				"failure_backup_count":    map[string]any{"type": "integer"},
				"total_backup_size_bytes": map[string]any{"type": "integer"},
				"total_backup_disk_bytes": map[string]any{"type": "integer"},
				"policy_count":            map[string]any{"type": "integer"},
				"last_backup":             map[string]any{"allOf": []any{map[string]any{"$ref": "#/components/schemas/Backup"}}, "nullable": true},
				"recent_trend":            map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/BackupTrendPoint"}},
			},
		},
		"InstanceOverview": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"instance":   map[string]any{"$ref": "#/components/schemas/OverviewInstance"},
				"stats":      map[string]any{"$ref": "#/components/schemas/InstanceStats"},
				"permission": map[string]any{"type": "string"},
			},
		},
		"UpcomingTask": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"policy_id":     map[string]any{"type": "integer"},
				"policy_name":   map[string]any{"type": "string"},
				"instance_id":   map[string]any{"type": "integer"},
				"instance_name": map[string]any{"type": "string"},
				"type":          map[string]any{"type": "string"},
				"next_run_at":   map[string]any{"type": "string", "format": "date-time"},
			},
		},
		"UpcomingTaskListResponse": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/UpcomingTask"},
				},
			},
		},
		"DisasterRecoveryScore": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"total":           map[string]any{"type": "number"},
				"level":           map[string]any{"type": "string"},
				"freshness":       map[string]any{"type": "number"},
				"recovery_points": map[string]any{"type": "number"},
				"redundancy":      map[string]any{"type": "number"},
				"stability":       map[string]any{"type": "number"},
				"deductions":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"calculated_at":   map[string]any{"type": "string", "format": "date-time"},
			},
		},
	}
}
