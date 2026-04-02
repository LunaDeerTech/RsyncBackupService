package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type RestoreHandler struct {
	restoreService *service.RestoreService
}

type createRestoreRequest struct {
	BackupRecordID    uint   `json:"backup_record_id"`
	RestoreTargetPath string `json:"restore_target_path"`
	Overwrite         bool   `json:"overwrite"`
}

type restoreRecordResponse struct {
	ID                uint    `json:"id"`
	InstanceID        uint    `json:"instance_id"`
	BackupRecordID    uint    `json:"backup_record_id"`
	RestoreTargetPath string  `json:"restore_target_path"`
	Overwrite         bool    `json:"overwrite"`
	Status            string  `json:"status"`
	StartedAt         string  `json:"started_at"`
	FinishedAt        *string `json:"finished_at,omitempty"`
	ErrorMessage      string  `json:"error_message,omitempty"`
	TriggeredBy       uint    `json:"triggered_by"`
}

func NewRestoreHandler(restoreService *service.RestoreService) *RestoreHandler {
	return &RestoreHandler{restoreService: restoreService}
}

func (h *RestoreHandler) Create(c *gin.Context) {
	if h.restoreService == nil {
		writeError(c, http.StatusInternalServerError, "restore service unavailable")
		return
	}

	user, ok := currentAuthUser(c)
	if !ok {
		return
	}
	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var requestBody createRestoreRequest
	if !bindJSON(c, &requestBody) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.restore", ResourceType: "restore_records"})

	restoreRecord, err := h.restoreService.Start(c.Request.Context(), service.RestoreRequest{
		InstanceID:        instanceID,
		BackupRecordID:    requestBody.BackupRecordID,
		RestoreTargetPath: requestBody.RestoreTargetPath,
		Overwrite:         requestBody.Overwrite,
		VerifyToken:       c.GetHeader("X-Verify-Token"),
		TriggeredBy:       user.UserID,
	})
	if err != nil {
		h.writeRestoreError(c, err, "start restore failed")
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.restore", ResourceType: "restore_records", ResourceID: restoreRecord.ID})
	c.JSON(http.StatusCreated, toRestoreRecordResponse(*restoreRecord))
}

func (h *RestoreHandler) List(c *gin.Context) {
	if h.restoreService == nil {
		writeError(c, http.StatusInternalServerError, "restore service unavailable")
		return
	}

	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	request := service.ListRestoreRecordsRequest{Actor: user}
	instanceIDValue := strings.TrimSpace(c.Query("instance_id"))
	if instanceIDValue != "" {
		parsedInstanceID, err := strconv.ParseUint(instanceIDValue, 10, 64)
		if err != nil {
			writeError(c, http.StatusBadRequest, "invalid instance id")
			return
		}
		instanceID := uint(parsedInstanceID)
		request.InstanceID = &instanceID
	}

	records, err := h.restoreService.List(c.Request.Context(), request)
	if err != nil {
		if service.IsValidationError(err) {
			writeError(c, http.StatusBadRequest, err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "list restore records failed")
		return
	}

	c.JSON(http.StatusOK, toRestoreRecordResponses(records))
}

func (h *RestoreHandler) writeRestoreError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrInstanceNotFound), errors.Is(err, service.ErrBackupRecordNotFound), errors.Is(err, service.ErrStorageTargetNotFound), errors.Is(err, service.ErrSSHKeyNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrVerifyTokenRequired), errors.Is(err, service.ErrVerifyTokenInvalid), errors.Is(err, service.ErrVerifyTokenExpired):
		writeError(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrRestoreTargetExists):
		writeError(c, http.StatusConflict, err.Error())
	case service.IsValidationError(err), errors.Is(err, service.ErrBackupRecordNotRestorable):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallbackMessage)
	}
}

func toRestoreRecordResponse(record model.RestoreRecord) restoreRecordResponse {
	return restoreRecordResponse{
		ID:                record.ID,
		InstanceID:        record.InstanceID,
		BackupRecordID:    record.BackupRecordID,
		RestoreTargetPath: record.RestoreTargetPath,
		Overwrite:         record.Overwrite,
		Status:            record.Status,
		StartedAt:         record.StartedAt.UTC().Format(http.TimeFormat),
		FinishedAt:        formatOptionalHTTPTime(record.FinishedAt),
		ErrorMessage:      record.ErrorMessage,
		TriggeredBy:       record.TriggeredBy,
	}
}

func toRestoreRecordResponses(records []model.RestoreRecord) []restoreRecordResponse {
	responses := make([]restoreRecordResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, toRestoreRecordResponse(record))
	}

	return responses
}