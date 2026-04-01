package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type SSHKeyHandler struct {
	sshKeyService *service.SSHKeyService
}

type sshKeyResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

func NewSSHKeyHandler(sshKeyService *service.SSHKeyService) *SSHKeyHandler {
	return &SSHKeyHandler{sshKeyService: sshKeyService}
}

func (h *SSHKeyHandler) List(c *gin.Context) {
	sshKeys, err := h.sshKeyService.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list ssh keys failed")
		return
	}

	c.JSON(http.StatusOK, toSSHKeyResponses(sshKeys))
}

func (h *SSHKeyHandler) Create(c *gin.Context) {
	var request service.CreateSSHKeyRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "ssh_keys.create", ResourceType: "ssh_keys"})

	sshKey, err := h.sshKeyService.Create(c.Request.Context(), request)
	if err != nil {
		h.writeSSHKeyError(c, err, "register ssh key failed")
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "ssh_keys.create", ResourceType: "ssh_keys", ResourceID: sshKey.ID})
	c.JSON(http.StatusCreated, toSSHKeyResponse(sshKey))
}

func (h *SSHKeyHandler) Delete(c *gin.Context) {
	sshKeyID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "ssh_keys.delete", ResourceType: "ssh_keys", ResourceID: sshKeyID})

	if err := h.sshKeyService.Delete(c.Request.Context(), sshKeyID); err != nil {
		h.writeSSHKeyError(c, err, "delete ssh key failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SSHKeyHandler) TestConnection(c *gin.Context) {
	sshKeyID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request service.TestSSHKeyConnectionRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "ssh_keys.test", ResourceType: "ssh_keys", ResourceID: sshKeyID})

	if err := h.sshKeyService.TestConnection(c.Request.Context(), sshKeyID, request); err != nil {
		h.writeSSHKeyConnectionError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *SSHKeyHandler) writeSSHKeyError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrSSHKeyNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrResourceInUse):
		writeError(c, http.StatusConflict, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallbackMessage)
	}
}

func (h *SSHKeyHandler) writeSSHKeyConnectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSSHKeyNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusBadGateway, err.Error())
	}
}

func toSSHKeyResponse(sshKey model.SSHKey) sshKeyResponse {
	return sshKeyResponse{
		ID:          sshKey.ID,
		Name:        sshKey.Name,
		Fingerprint: sshKey.Fingerprint,
		CreatedAt:   sshKey.CreatedAt.UTC().Format(http.TimeFormat),
	}
}

func toSSHKeyResponses(sshKeys []model.SSHKey) []sshKeyResponse {
	responses := make([]sshKeyResponse, 0, len(sshKeys))
	for _, sshKey := range sshKeys {
		responses = append(responses, toSSHKeyResponse(sshKey))
	}

	return responses
}