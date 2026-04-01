package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type InstanceHandler struct {
	instanceService *service.InstanceService
}

type instanceResponse struct {
	ID              uint     `json:"id"`
	Name            string   `json:"name"`
	SourceType      string   `json:"source_type"`
	SourceHost      string   `json:"source_host,omitempty"`
	SourcePort      int      `json:"source_port"`
	SourceUser      string   `json:"source_user,omitempty"`
	SourceSSHKeyID  *uint    `json:"source_ssh_key_id,omitempty"`
	SourcePath      string   `json:"source_path"`
	ExcludePatterns []string `json:"exclude_patterns"`
	Enabled         bool     `json:"enabled"`
	CreatedBy       uint     `json:"created_by"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

func NewInstanceHandler(instanceService *service.InstanceService) *InstanceHandler {
	return &InstanceHandler{instanceService: instanceService}
}

func (h *InstanceHandler) List(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instances, err := h.instanceService.List(c.Request.Context(), user)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list instances failed")
		return
	}

	c.JSON(http.StatusOK, toInstanceResponses(instances))
}

func (h *InstanceHandler) Create(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	var request service.CreateInstanceRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.create", ResourceType: "backup_instances"})

	instance, err := h.instanceService.Create(c.Request.Context(), user, request)
	if err != nil {
		h.writeInstanceError(c, err, "create instance failed")
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.create", ResourceType: "backup_instances", ResourceID: instance.ID})
	c.JSON(http.StatusCreated, toInstanceResponse(instance))
}

func (h *InstanceHandler) Get(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	instance, err := h.instanceService.Get(c.Request.Context(), user, instanceID)
	if err != nil {
		h.writeInstanceError(c, err, "load instance failed")
		return
	}

	c.JSON(http.StatusOK, toInstanceResponse(instance))
}

func (h *InstanceHandler) Update(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request service.UpdateInstanceRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.update", ResourceType: "backup_instances", ResourceID: instanceID})

	instance, err := h.instanceService.Update(c.Request.Context(), user, instanceID, request)
	if err != nil {
		h.writeInstanceError(c, err, "update instance failed")
		return
	}

	c.JSON(http.StatusOK, toInstanceResponse(instance))
}

func (h *InstanceHandler) Delete(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "instances.delete", ResourceType: "backup_instances", ResourceID: instanceID})

	if err := h.instanceService.Delete(c.Request.Context(), user, instanceID); err != nil {
		h.writeInstanceError(c, err, "delete instance failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *InstanceHandler) writeInstanceError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrInstanceNotFound), errors.Is(err, service.ErrSSHKeyNotFound):
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

func toInstanceResponse(instance model.BackupInstance) instanceResponse {
	return instanceResponse{
		ID:              instance.ID,
		Name:            instance.Name,
		SourceType:      instance.SourceType,
		SourceHost:      instance.SourceHost,
		SourcePort:      instance.SourcePort,
		SourceUser:      instance.SourceUser,
		SourceSSHKeyID:  instance.SourceSSHKeyID,
		SourcePath:      instance.SourcePath,
		ExcludePatterns: decodeExcludePatterns(instance.ExcludePatterns),
		Enabled:         instance.Enabled,
		CreatedBy:       instance.CreatedBy,
		CreatedAt:       instance.CreatedAt.UTC().Format(http.TimeFormat),
		UpdatedAt:       instance.UpdatedAt.UTC().Format(http.TimeFormat),
	}
}

func toInstanceResponses(instances []model.BackupInstance) []instanceResponse {
	responses := make([]instanceResponse, 0, len(instances))
	for _, instance := range instances {
		responses = append(responses, toInstanceResponse(instance))
	}

	return responses
}