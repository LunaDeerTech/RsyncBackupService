package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type BackupHandler struct {
	executorService *service.ExecutorService
}

type backupRecordResponse struct {
	ID               uint    `json:"id"`
	InstanceID       uint    `json:"instance_id"`
	StorageTargetID  uint    `json:"storage_target_id"`
	StrategyID       *uint   `json:"strategy_id,omitempty"`
	BackupType       string  `json:"backup_type"`
	Status           string  `json:"status"`
	SnapshotPath     string  `json:"snapshot_path"`
	BytesTransferred int64   `json:"bytes_transferred"`
	FilesTransferred int64   `json:"files_transferred"`
	TotalSize        int64   `json:"total_size"`
	VolumeCount      int     `json:"volume_count"`
	StartedAt        string  `json:"started_at"`
	FinishedAt       *string `json:"finished_at,omitempty"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

func NewBackupHandler(executorService *service.ExecutorService) *BackupHandler {
	return &BackupHandler{executorService: executorService}
}

func (h *BackupHandler) List(c *gin.Context) {
	if h.executorService == nil {
		writeError(c, http.StatusInternalServerError, "backup service unavailable")
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	request, err := buildListBackupRecordsRequest(c, instanceID)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	records, err := h.executorService.ListBackupRecords(c.Request.Context(), request)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list backups failed")
		return
	}

	c.JSON(http.StatusOK, toBackupRecordResponses(records))
}

func (h *BackupHandler) ListSnapshots(c *gin.Context) {
	if h.executorService == nil {
		writeError(c, http.StatusInternalServerError, "backup service unavailable")
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	request, err := buildListBackupRecordsRequest(c, instanceID)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	records, err := h.executorService.ListRestorableBackups(c.Request.Context(), request)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list restorable backups failed")
		return
	}

	c.JSON(http.StatusOK, toBackupRecordResponses(records))
}

func buildListBackupRecordsRequest(c *gin.Context, instanceID uint) (service.ListBackupRecordsRequest, error) {
	request := service.ListBackupRecordsRequest{
		InstanceID: instanceID,
		BackupType: strings.TrimSpace(c.Query("backup_type")),
		Status:     strings.TrimSpace(c.Query("status")),
	}

	strategyIDValue := strings.TrimSpace(c.Query("strategy_id"))
	if strategyIDValue != "" {
		parsedStrategyID, err := strconv.ParseUint(strategyIDValue, 10, 64)
		if err != nil {
			return service.ListBackupRecordsRequest{}, fmt.Errorf("invalid strategy id")
		}
		strategyID := uint(parsedStrategyID)
		request.StrategyID = &strategyID
	}

	return request, nil
}

func toBackupRecordResponse(record model.BackupRecord) backupRecordResponse {
	return backupRecordResponse{
		ID:               record.ID,
		InstanceID:       record.InstanceID,
		StorageTargetID:  record.StorageTargetID,
		StrategyID:       record.StrategyID,
		BackupType:       record.BackupType,
		Status:           record.Status,
		SnapshotPath:     record.SnapshotPath,
		BytesTransferred: record.BytesTransferred,
		FilesTransferred: record.FilesTransferred,
		TotalSize:        record.TotalSize,
		VolumeCount:      record.VolumeCount,
		StartedAt:        record.StartedAt.UTC().Format(http.TimeFormat),
		FinishedAt:       formatOptionalHTTPTime(record.FinishedAt),
		ErrorMessage:     record.ErrorMessage,
	}
}

func toBackupRecordResponses(records []model.BackupRecord) []backupRecordResponse {
	responses := make([]backupRecordResponse, 0, len(records))
	for _, record := range records {
		responses = append(responses, toBackupRecordResponse(record))
	}

	return responses
}