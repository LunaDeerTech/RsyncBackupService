package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type StrategyHandler struct {
	strategyService *service.StrategyService
}

type strategyResponse struct {
	ID                  uint    `json:"id"`
	InstanceID          uint    `json:"instance_id"`
	Name                string  `json:"name"`
	BackupType          string  `json:"backup_type"`
	CronExpr            *string `json:"cron_expr,omitempty"`
	IntervalSeconds     int     `json:"interval_seconds"`
	RetentionDays       int     `json:"retention_days"`
	RetentionCount      int     `json:"retention_count"`
	ColdVolumeSize      *string `json:"cold_volume_size,omitempty"`
	MaxExecutionSeconds int     `json:"max_execution_seconds"`
	StorageTargetIDs    []uint  `json:"storage_target_ids"`
	Enabled             bool    `json:"enabled"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func NewStrategyHandler(strategyService *service.StrategyService) *StrategyHandler {
	return &StrategyHandler{strategyService: strategyService}
}

func (h *StrategyHandler) ListByInstance(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	strategies, err := h.strategyService.ListByInstance(c.Request.Context(), user, instanceID)
	if err != nil {
		h.writeStrategyError(c, err, "list strategies failed")
		return
	}

	c.JSON(http.StatusOK, toStrategyResponses(strategies))
}

func (h *StrategyHandler) Create(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request service.CreateStrategyRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "strategies.create", ResourceType: "strategies", Detail: map[string]any{"instance_id": instanceID}})

	strategy, err := h.strategyService.Create(c.Request.Context(), user, instanceID, request)
	if err != nil {
		h.writeStrategyError(c, err, "create strategy failed")
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "strategies.create", ResourceType: "strategies", ResourceID: strategy.ID, Detail: map[string]any{"instance_id": instanceID}})
	c.JSON(http.StatusCreated, toStrategyResponse(strategy))
}

func (h *StrategyHandler) Update(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	strategyID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request service.UpdateStrategyRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "strategies.update", ResourceType: "strategies", ResourceID: strategyID})

	strategy, err := h.strategyService.Update(c.Request.Context(), user, strategyID, request)
	if err != nil {
		h.writeStrategyError(c, err, "update strategy failed")
		return
	}

	c.JSON(http.StatusOK, toStrategyResponse(strategy))
}

func (h *StrategyHandler) Delete(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	strategyID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "strategies.delete", ResourceType: "strategies", ResourceID: strategyID})

	if err := h.strategyService.Delete(c.Request.Context(), user, strategyID); err != nil {
		h.writeStrategyError(c, err, "delete strategy failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *StrategyHandler) writeStrategyError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrInstanceNotFound), errors.Is(err, service.ErrStrategyNotFound), errors.Is(err, service.ErrStorageTargetNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrPermissionDenied):
		writeError(c, http.StatusForbidden, "insufficient instance permission")
	case errors.Is(err, service.ErrResourceInUse):
		writeError(c, http.StatusConflict, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallbackMessage)
	}
}

func toStrategyResponse(strategy model.Strategy) strategyResponse {
	storageTargetIDs := make([]uint, 0, len(strategy.StorageTargets))
	for _, storageTarget := range strategy.StorageTargets {
		storageTargetIDs = append(storageTargetIDs, storageTarget.ID)
	}

	return strategyResponse{
		ID:                  strategy.ID,
		InstanceID:          strategy.InstanceID,
		Name:                strategy.Name,
		BackupType:          strategy.BackupType,
		CronExpr:            strategy.CronExpr,
		IntervalSeconds:     strategy.IntervalSeconds,
		RetentionDays:       strategy.RetentionDays,
		RetentionCount:      strategy.RetentionCount,
		ColdVolumeSize:      strategy.ColdVolumeSize,
		MaxExecutionSeconds: strategy.MaxExecutionSeconds,
		StorageTargetIDs:    storageTargetIDs,
		Enabled:             strategy.Enabled,
		CreatedAt:           strategy.CreatedAt.UTC().Format(http.TimeFormat),
		UpdatedAt:           strategy.UpdatedAt.UTC().Format(http.TimeFormat),
	}
}

func toStrategyResponses(strategies []model.Strategy) []strategyResponse {
	responses := make([]strategyResponse, 0, len(strategies))
	for _, strategy := range strategies {
		responses = append(responses, toStrategyResponse(strategy))
	}

	return responses
}