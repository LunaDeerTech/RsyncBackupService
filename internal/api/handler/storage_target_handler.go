package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type StorageTargetHandler struct {
	storageTargetService *service.StorageTargetService
}

type storageTargetResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port"`
	User      string `json:"user,omitempty"`
	SSHKeyID  *uint  `json:"ssh_key_id,omitempty"`
	BasePath  string `json:"base_path"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NewStorageTargetHandler(storageTargetService *service.StorageTargetService) *StorageTargetHandler {
	return &StorageTargetHandler{storageTargetService: storageTargetService}
}

func (h *StorageTargetHandler) List(c *gin.Context) {
	storageTargets, err := h.storageTargetService.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list storage targets failed")
		return
	}

	c.JSON(http.StatusOK, toStorageTargetResponses(storageTargets))
}

func (h *StorageTargetHandler) Create(c *gin.Context) {
	var request service.CreateStorageTargetRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "storage_targets.create", ResourceType: "storage_targets"})

	storageTarget, err := h.storageTargetService.Create(c.Request.Context(), request)
	if err != nil {
		h.writeStorageTargetError(c, err, "create storage target failed")
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "storage_targets.create", ResourceType: "storage_targets", ResourceID: storageTarget.ID})
	c.JSON(http.StatusCreated, toStorageTargetResponse(storageTarget))
}

func (h *StorageTargetHandler) Update(c *gin.Context) {
	storageTargetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request service.UpdateStorageTargetRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "storage_targets.update", ResourceType: "storage_targets", ResourceID: storageTargetID})

	storageTarget, err := h.storageTargetService.Update(c.Request.Context(), storageTargetID, request)
	if err != nil {
		h.writeStorageTargetError(c, err, "update storage target failed")
		return
	}

	c.JSON(http.StatusOK, toStorageTargetResponse(storageTarget))
}

func (h *StorageTargetHandler) Delete(c *gin.Context) {
	storageTargetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "storage_targets.delete", ResourceType: "storage_targets", ResourceID: storageTargetID})

	if err := h.storageTargetService.Delete(c.Request.Context(), storageTargetID); err != nil {
		h.writeStorageTargetError(c, err, "delete storage target failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *StorageTargetHandler) TestConnection(c *gin.Context) {
	storageTargetID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "storage_targets.test", ResourceType: "storage_targets", ResourceID: storageTargetID})

	if err := h.storageTargetService.TestConnection(c.Request.Context(), storageTargetID); err != nil {
		h.writeStorageTargetConnectionError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *StorageTargetHandler) writeStorageTargetError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrStorageTargetNotFound), errors.Is(err, service.ErrSSHKeyNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrResourceInUse):
		writeError(c, http.StatusConflict, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallbackMessage)
	}
}

func (h *StorageTargetHandler) writeStorageTargetConnectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrStorageTargetNotFound), errors.Is(err, service.ErrSSHKeyNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusBadGateway, err.Error())
	}
}

func toStorageTargetResponse(storageTarget model.StorageTarget) storageTargetResponse {
	return storageTargetResponse{
		ID:        storageTarget.ID,
		Name:      storageTarget.Name,
		Type:      storageTarget.Type,
		Host:      storageTarget.Host,
		Port:      storageTarget.Port,
		User:      storageTarget.User,
		SSHKeyID:  storageTarget.SSHKeyID,
		BasePath:  storageTarget.BasePath,
		CreatedAt: storageTarget.CreatedAt.UTC().Format(http.TimeFormat),
		UpdatedAt: storageTarget.UpdatedAt.UTC().Format(http.TimeFormat),
	}
}

func toStorageTargetResponses(storageTargets []model.StorageTarget) []storageTargetResponse {
	responses := make([]storageTargetResponse, 0, len(storageTargets))
	for _, storageTarget := range storageTargets {
		responses = append(responses, toStorageTargetResponse(storageTarget))
	}

	return responses
}