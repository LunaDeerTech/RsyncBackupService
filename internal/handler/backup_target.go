package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/store"
	"rsync-backup-service/internal/util"
)

const (
	backupTargetErrorNotFound = 40403
	backupTargetErrorConflict = 40903
)

type backupTargetRequest struct {
	Name           string `json:"name"`
	BackupType     string `json:"backup_type"`
	StorageType    string `json:"storage_type"`
	StoragePath    string `json:"storage_path"`
	RemoteConfigID *int64 `json:"remote_config_id"`
}

type backupTargetInput struct {
	Name           string
	BackupType     string
	StorageType    string
	StoragePath    string
	RemoteConfigID *int64
}

func (h *Handler) ListBackupTargets(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	pagination := ParsePagination(r)
	total, err := h.db.CountBackupTargets()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count backup targets")
		return
	}

	targets, err := h.db.ListBackupTargetsPage(pagination.PageSize, (pagination.Page-1)*pagination.PageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list backup targets")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      targets,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(total, pagination.PageSize),
	})
}

func (h *Handler) CreateBackupTarget(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request backupTargetRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := normalizeBackupTargetInput(request)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.validateBackupTargetInput(input); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureBackupTargetNameAvailable(input.Name, 0); err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	target := &model.BackupTarget{
		Name:           input.Name,
		BackupType:     input.BackupType,
		StorageType:    input.StorageType,
		StoragePath:    input.StoragePath,
		RemoteConfigID: cloneOptionalInt64(input.RemoteConfigID),
		HealthStatus:   "degraded",
		HealthMessage:  "health check pending",
	}
	if err := h.db.CreateBackupTarget(target); err != nil {
		writeBackupTargetError(w, err, "failed to create backup target")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionTargetCreate, map[string]any{
		"target_id":        target.ID,
		"name":             target.Name,
		"backup_type":      target.BackupType,
		"storage_type":     target.StorageType,
		"storage_path":     target.StoragePath,
		"remote_config_id": target.RemoteConfigID,
	})

	JSON(w, http.StatusCreated, target)
}

func (h *Handler) UpdateBackupTarget(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	targetID, err := backupTargetIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid backup target id")
		return
	}

	current, err := h.db.GetBackupTargetByID(targetID)
	if err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	var request backupTargetRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	input, err := normalizeBackupTargetInput(request)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.validateBackupTargetInput(input); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	if err := h.ensureBackupTargetNameAvailable(input.Name, current.ID); err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	current.Name = input.Name
	current.BackupType = input.BackupType
	current.StorageType = input.StorageType
	current.StoragePath = input.StoragePath
	current.RemoteConfigID = cloneOptionalInt64(input.RemoteConfigID)
	if err := h.db.UpdateBackupTarget(current); err != nil {
		writeBackupTargetError(w, err, "failed to update backup target")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionTargetUpdate, map[string]any{
		"target_id":        current.ID,
		"name":             current.Name,
		"backup_type":      current.BackupType,
		"storage_type":     current.StorageType,
		"storage_path":     current.StoragePath,
		"remote_config_id": current.RemoteConfigID,
	})

	JSON(w, http.StatusOK, current)
}

func (h *Handler) DeleteBackupTarget(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	targetID, err := backupTargetIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid backup target id")
		return
	}

	usage, err := h.db.GetBackupTargetUsage(targetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query backup target usage")
		return
	}
	if usage.InUse() {
		ErrorWithData(w, http.StatusBadRequest, authErrorInvalidRequest, "backup target is in use", copyBackupTargetUsage(usage))
		return
	}
	target, err := h.db.GetBackupTargetByID(targetID)
	if err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	if err := h.db.DeleteBackupTarget(targetID); err != nil {
		writeBackupTargetError(w, err, "failed to delete backup target")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionTargetDelete, map[string]any{
		"deleted_target_id": target.ID,
		"name":              target.Name,
		"backup_type":       target.BackupType,
		"storage_type":      target.StorageType,
	})

	JSON(w, http.StatusOK, map[string]string{"message": "backup target deleted"})
}

func (h *Handler) CheckBackupTargetHealth(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	targetID, err := backupTargetIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid backup target id")
		return
	}

	target, err := h.db.GetBackupTargetByID(targetID)
	if err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	healthChecker := engine.NewHealthChecker(h.db)
	status, message, total, used, err := healthChecker.CheckTarget(target)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to run backup target health check")
		return
	}
	if err := h.db.UpdateHealthStatus(targetID, status, message, total, used); err != nil {
		writeBackupTargetError(w, err, "failed to persist backup target health check")
		return
	}
	riskDetector := engine.NewRiskDetector(h.db, nil, h.audit)
	if h.disasterRecovery != nil {
		riskDetector = engine.NewRiskDetector(h.db, h.disasterRecovery.Cache(), h.audit)
	}
	if err := riskDetector.OnHealthCheckComplete(r.Context(), targetID, status); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update backup target risk state")
		return
	}
	if h.disasterRecovery != nil && engine.TargetHealthChanged(target, status, message, total, used) {
		h.disasterRecovery.InvalidateByTarget(targetID)
	}

	updated, err := h.db.GetBackupTargetByID(targetID)
	if err != nil {
		writeBackupTargetError(w, err, "failed to query backup target")
		return
	}

	JSON(w, http.StatusOK, updated)
}

func normalizeBackupTargetInput(request backupTargetRequest) (backupTargetInput, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return backupTargetInput{}, fmt.Errorf("name is required")
	}

	backupType := strings.ToLower(strings.TrimSpace(request.BackupType))
	switch backupType {
	case "rolling", "cold":
	default:
		return backupTargetInput{}, fmt.Errorf("backup_type must be rolling or cold")
	}

	storageType := strings.ToLower(strings.TrimSpace(request.StorageType))
	switch storageType {
	case "local", "ssh", "openlist":
	default:
		return backupTargetInput{}, fmt.Errorf("storage_type must be local, ssh, or openlist")
	}

	storagePath := strings.TrimSpace(request.StoragePath)
	if storagePath == "" {
		return backupTargetInput{}, fmt.Errorf("storage_path is required")
	}
	if err := util.ValidatePath(storagePath); err != nil {
		return backupTargetInput{}, fmt.Errorf("storage_path: %w", err)
	}

	if request.RemoteConfigID != nil && *request.RemoteConfigID <= 0 {
		return backupTargetInput{}, fmt.Errorf("remote_config_id must be positive")
	}

	return backupTargetInput{
		Name:           name,
		BackupType:     backupType,
		StorageType:    storageType,
		StoragePath:    storagePath,
		RemoteConfigID: cloneOptionalInt64(request.RemoteConfigID),
	}, nil
}

func (h *Handler) validateBackupTargetInput(input backupTargetInput) error {
	switch input.BackupType {
	case "rolling":
		if input.StorageType != "local" && input.StorageType != "ssh" {
			return fmt.Errorf("rolling backups only support local or ssh storage")
		}
	case "cold":
		if input.StorageType != "local" && input.StorageType != "ssh" && input.StorageType != "openlist" {
			return fmt.Errorf("cold backups only support local, ssh, or openlist storage")
		}
	}

	if input.StorageType == "ssh" || input.StorageType == "openlist" {
		if input.RemoteConfigID == nil {
			return fmt.Errorf("remote_config_id is required for %s storage", input.StorageType)
		}
		remoteConfig, err := h.db.GetRemoteConfigByID(*input.RemoteConfigID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("remote_config_id is invalid")
			}
			return err
		}
		if input.StorageType == "ssh" && remoteConfig.Type != "ssh" {
			return fmt.Errorf("remote_config_id must reference an ssh remote config")
		}
		if input.StorageType == "openlist" && !openlist.IsRemoteConfig(*remoteConfig) {
			return fmt.Errorf("remote_config_id must reference an openlist remote config")
		}
		return nil
	}

	if input.RemoteConfigID != nil {
		return fmt.Errorf("remote_config_id is only supported for ssh or openlist storage")
	}

	return nil
}

func (h *Handler) ensureBackupTargetNameAvailable(name string, currentID int64) error {
	target, err := h.db.GetBackupTargetByName(name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if target.ID != currentID {
		return errBackupTargetNameExists
	}

	return nil
}

var errBackupTargetNameExists = errors.New("backup target name already exists")

func writeBackupTargetError(w http.ResponseWriter, err error, defaultMessage string) {
	switch {
	case errors.Is(err, errBackupTargetNameExists):
		Error(w, http.StatusConflict, backupTargetErrorConflict, "backup target name already exists")
	case errors.Is(err, sql.ErrNoRows):
		Error(w, http.StatusNotFound, backupTargetErrorNotFound, "backup target not found")
	default:
		Error(w, http.StatusInternalServerError, authErrorInternal, defaultMessage)
	}
}

func backupTargetIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("backup target id is required")
	}

	targetID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse backup target id %q: %w", rawID, err)
	}
	if targetID <= 0 {
		return 0, fmt.Errorf("backup target id must be positive")
	}

	return targetID, nil
}

func cloneOptionalInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func copyBackupTargetUsage(usage store.BackupTargetUsage) store.BackupTargetUsage {
	cloned := store.BackupTargetUsage{}
	if len(usage.Policies) > 0 {
		cloned.Policies = append([]string(nil), usage.Policies...)
	}
	return cloned
}
