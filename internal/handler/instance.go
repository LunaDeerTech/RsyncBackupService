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

const (
	instanceErrorNotFound = 40404
	instanceErrorConflict = 40904
)

type instanceRequest struct {
	Name           string `json:"name"`
	SourceType     string `json:"source_type"`
	SourcePath     string `json:"source_path"`
	RemoteConfigID *int64 `json:"remote_config_id"`
}

type instanceInput struct {
	Name           string
	SourceType     string
	SourcePath     string
	RemoteConfigID *int64
}

type instancePermissionAssignmentRequest struct {
	UserID     int64  `json:"user_id"`
	Permission string `json:"permission"`
}

type instancePermissionsRequest struct {
	Permissions []instancePermissionAssignmentRequest `json:"permissions"`
}

type instanceListItem struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	SourceType       string     `json:"source_type"`
	SourcePath       string     `json:"source_path"`
	RemoteConfigID   *int64     `json:"remote_config_id,omitempty"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	LastBackupTime   *time.Time `json:"last_backup_time,omitempty"`
	LastBackupStatus *string    `json:"last_backup_status,omitempty"`
	BackupCount      int64      `json:"backup_count"`
}

type instanceDetailResponse struct {
	Instance model.Instance      `json:"instance"`
	Stats    model.InstanceStats `json:"stats"`
}

func (h *Handler) ListInstances(w http.ResponseWriter, r *http.Request) {
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

	items, err := h.buildInstanceListItems(instances)
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

func (h *Handler) CreateInstance(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request instanceRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := normalizeInstanceInput(request)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.validateInstanceInput(input); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureInstanceNameAvailable(input.Name, 0); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	instance := &model.Instance{
		Name:           input.Name,
		SourceType:     input.SourceType,
		SourcePath:     input.SourcePath,
		RemoteConfigID: cloneOptionalInt64(input.RemoteConfigID),
		Status:         "idle",
	}
	if err := h.db.CreateInstance(instance); err != nil {
		writeInstanceError(w, err, "failed to create instance")
		return
	}

	JSON(w, http.StatusCreated, instance)
}

func (h *Handler) GetInstance(w http.ResponseWriter, r *http.Request) {
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

	JSON(w, http.StatusOK, instanceDetailResponse{Instance: *instance, Stats: *stats})
}

func (h *Handler) UpdateInstance(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}

	current, err := h.db.GetInstanceByID(instanceID)
	if err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}
	if current.Status != "idle" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only idle instances can be edited")
		return
	}

	var request instanceRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := normalizeInstanceInput(request)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.validateInstanceInput(input); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureInstanceNameAvailable(input.Name, current.ID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	current.Name = input.Name
	current.SourceType = input.SourceType
	current.SourcePath = input.SourcePath
	current.RemoteConfigID = cloneOptionalInt64(input.RemoteConfigID)
	if err := h.db.UpdateInstance(current); err != nil {
		writeInstanceError(w, err, "failed to update instance")
		return
	}

	JSON(w, http.StatusOK, current)
}

func (h *Handler) DeleteInstance(w http.ResponseWriter, r *http.Request) {
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
	if instance.Status != "idle" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "only idle instances can be deleted")
		return
	}

	if err := h.db.DeleteInstance(instanceID); err != nil {
		writeInstanceError(w, err, "failed to delete instance")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "instance deleted"})
}

func (h *Handler) GetInstanceStats(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
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

	stats, err := h.db.GetInstanceStats(instanceID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load instance stats")
		return
	}

	JSON(w, http.StatusOK, stats)
}

func (h *Handler) UpdateInstancePermissions(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
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

	var request instancePermissionsRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	permissions, err := h.normalizeInstancePermissions(request)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	if err := h.db.SetInstancePermissions(instanceID, permissions); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update instance permissions")
		return
	}

	JSON(w, http.StatusOK, map[string]any{"permissions": request.Permissions})
}

func normalizeInstanceInput(request instanceRequest) (instanceInput, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return instanceInput{}, fmt.Errorf("name is required")
	}

	sourceType := strings.ToLower(strings.TrimSpace(request.SourceType))
	switch sourceType {
	case "local", "ssh":
	default:
		return instanceInput{}, fmt.Errorf("source_type must be local or ssh")
	}

	sourcePath := strings.TrimSpace(request.SourcePath)
	if sourcePath == "" {
		return instanceInput{}, fmt.Errorf("source_path is required")
	}

	if request.RemoteConfigID != nil && *request.RemoteConfigID <= 0 {
		return instanceInput{}, fmt.Errorf("remote_config_id must be positive")
	}

	return instanceInput{
		Name:           name,
		SourceType:     sourceType,
		SourcePath:     sourcePath,
		RemoteConfigID: cloneOptionalInt64(request.RemoteConfigID),
	}, nil
}

func (h *Handler) validateInstanceInput(input instanceInput) error {
	if input.SourceType == "ssh" {
		if input.RemoteConfigID == nil {
			return fmt.Errorf("remote_config_id is required for ssh source")
		}
		remoteConfig, err := h.db.GetRemoteConfigByID(*input.RemoteConfigID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("remote_config_id is invalid")
			}
			return err
		}
		if remoteConfig.Type != "ssh" {
			return fmt.Errorf("remote_config_id must reference an ssh remote config")
		}
		return nil
	}

	if input.RemoteConfigID != nil {
		return fmt.Errorf("remote_config_id is only supported for ssh source")
	}

	return nil
}

func (h *Handler) ensureInstanceNameAvailable(name string, currentID int64) error {
	instance, err := h.db.GetInstanceByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if instance.ID != currentID {
		return errInstanceNameExists
	}

	return nil
}

func (h *Handler) normalizeInstancePermissions(request instancePermissionsRequest) ([]model.InstancePermission, error) {
	permissions := make([]model.InstancePermission, 0, len(request.Permissions))
	seenUserIDs := make(map[int64]struct{}, len(request.Permissions))

	for _, item := range request.Permissions {
		if item.UserID <= 0 {
			return nil, fmt.Errorf("user_id must be positive")
		}
		if _, exists := seenUserIDs[item.UserID]; exists {
			return nil, fmt.Errorf("duplicate user_id %d", item.UserID)
		}
		seenUserIDs[item.UserID] = struct{}{}

		permission := strings.ToLower(strings.TrimSpace(item.Permission))
		if permission != "readonly" {
			return nil, fmt.Errorf("permission must be readonly")
		}

		user, err := h.db.GetUserByID(item.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("user_id %d does not exist", item.UserID)
			}
			return nil, err
		}
		if user.Role != "viewer" {
			return nil, fmt.Errorf("user_id %d must belong to a viewer", item.UserID)
		}

		permissions = append(permissions, model.InstancePermission{
			UserID:     item.UserID,
			Permission: permission,
		})
	}

	return permissions, nil
}

func (h *Handler) buildInstanceListItems(instances []model.Instance) ([]instanceListItem, error) {
	items := make([]instanceListItem, 0, len(instances))
	for _, instance := range instances {
		stats, err := h.db.GetInstanceStats(instance.ID)
		if err != nil {
			return nil, err
		}
		item := instanceListItem{
			ID:             instance.ID,
			Name:           instance.Name,
			SourceType:     instance.SourceType,
			SourcePath:     instance.SourcePath,
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
		items = append(items, item)
	}

	return items, nil
}

func backupOccurredAt(backup *model.Backup) *time.Time {
	if backup == nil {
		return nil
	}
	if backup.CompletedAt != nil {
		occurredAt := *backup.CompletedAt
		return &occurredAt
	}
	if backup.StartedAt != nil {
		occurredAt := *backup.StartedAt
		return &occurredAt
	}
	occurredAt := backup.CreatedAt
	return &occurredAt
}

var errInstanceNameExists = errors.New("instance name already exists")

func writeInstanceError(w http.ResponseWriter, err error, defaultMessage string) {
	switch {
	case errors.Is(err, errInstanceNameExists):
		Error(w, http.StatusConflict, instanceErrorConflict, "instance name already exists")
	case errors.Is(err, sql.ErrNoRows):
		Error(w, http.StatusNotFound, instanceErrorNotFound, "instance not found")
	default:
		Error(w, http.StatusInternalServerError, authErrorInternal, defaultMessage)
	}
}

func instanceIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("instance id is required")
	}

	instanceID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse instance id %q: %w", rawID, err)
	}
	if instanceID <= 0 {
		return 0, fmt.Errorf("instance id must be positive")
	}

	return instanceID, nil
}