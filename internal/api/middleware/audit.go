package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/gin-gonic/gin"
)

const auditMetadataKey = "audit-metadata"

type AuditMetadata struct {
	Action       string
	ResourceType string
	ResourceID   uint
	Detail       map[string]any
}

func WithAuditMetadata(metadata AuditMetadata) gin.HandlerFunc {
	return func(c *gin.Context) {
		SetAuditMetadata(c, metadata)
		c.Next()
	}
}

func SetAuditMetadata(c *gin.Context, metadata AuditMetadata) {
	c.Set(auditMetadataKey, metadata)
}

func AuditLogger(repo repository.AuditLogRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if repo == nil || !shouldAudit(c) {
			return
		}

		user, ok := CurrentUser(c)
		if !ok {
			return
		}

		metadata := auditMetadataFromContext(c)
		if metadata.Action == "" {
			metadata.Action = defaultAuditAction(c)
		}
		if metadata.ResourceType == "" {
			metadata.ResourceType = defaultResourceType(c)
		}
		if metadata.ResourceID == 0 {
			if resourceID, err := parseUintParam(c.Param("id")); err == nil {
				metadata.ResourceID = resourceID
			}
		}

		detail := metadata.Detail
		if detail == nil {
			detail = make(map[string]any)
		}
		detail["method"] = c.Request.Method
		detail["path"] = c.FullPath()
		detail["status"] = c.Writer.Status()

		encodedDetail, err := json.Marshal(detail)
		if err != nil {
			encodedDetail = []byte(`{"error":"marshal audit detail"}`)
		}

		_ = repo.Create(c.Request.Context(), &model.AuditLog{
			UserID:       user.UserID,
			Action:       metadata.Action,
			ResourceType: metadata.ResourceType,
			ResourceID:   metadata.ResourceID,
			Detail:       string(encodedDetail),
			IPAddress:    c.ClientIP(),
		})
	}
}

func shouldAudit(c *gin.Context) bool {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodOptions {
		return false
	}

	return strings.HasPrefix(c.FullPath(), "/api/")
}

func auditMetadataFromContext(c *gin.Context) AuditMetadata {
	value, exists := c.Get(auditMetadataKey)
	if !exists {
		return AuditMetadata{}
	}

	metadata, ok := value.(AuditMetadata)
	if !ok {
		return AuditMetadata{}
	}

	return metadata
}

func defaultAuditAction(c *gin.Context) string {
	resourceType := defaultResourceType(c)

	switch c.Request.Method {
	case http.MethodPost:
		return resourceType + ".create"
	case http.MethodPut, http.MethodPatch:
		return resourceType + ".update"
	case http.MethodDelete:
		return resourceType + ".delete"
	default:
		return strings.ToLower(c.Request.Method) + "." + resourceType
	}
}

func defaultResourceType(c *gin.Context) string {
	segments := strings.Split(strings.Trim(c.FullPath(), "/"), "/")
	if len(segments) < 2 {
		return "api"
	}
	if len(segments) >= 4 && segments[1] == "instances" && segments[3] == "permissions" {
		return "instance_permissions"
	}

	return segments[1]
}