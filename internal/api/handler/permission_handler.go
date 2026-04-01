package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	permissionService *service.PermissionService
}

type upsertPermissionRequest struct {
	Role string `json:"role"`
}

func NewPermissionHandler(permissionService *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permissionService: permissionService}
}

func (h *PermissionHandler) List(c *gin.Context) {
	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	permissions, err := h.permissionService.ListInstancePermissions(c.Request.Context(), instanceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInstanceNotFound):
			writeError(c, http.StatusNotFound, "instance not found")
		default:
			writeError(c, http.StatusInternalServerError, "list instance permissions failed")
		}
		return
	}

	c.JSON(http.StatusOK, permissions)
}

func (h *PermissionHandler) Upsert(c *gin.Context) {
	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	userID, ok := parseIDParam(c, "userID")
	if !ok {
		return
	}

	var request upsertPermissionRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "instance_permissions.upsert",
		ResourceType: "instance_permissions",
		ResourceID:   instanceID,
		Detail: map[string]any{
			"user_id": userID,
			"role":    request.Role,
		},
	})

	permission, err := h.permissionService.SetInstanceRole(c.Request.Context(), instanceID, userID, request.Role)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRole):
			writeError(c, http.StatusBadRequest, "invalid instance role")
		case errors.Is(err, service.ErrInstanceNotFound):
			writeError(c, http.StatusNotFound, "instance not found")
		case errors.Is(err, service.ErrUserNotFound):
			writeError(c, http.StatusNotFound, "user not found")
		default:
			writeError(c, http.StatusInternalServerError, "set instance permission failed")
		}
		return
	}

	c.JSON(http.StatusOK, permission)
}