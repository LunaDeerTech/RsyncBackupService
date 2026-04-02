package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

const defaultAuditLogsPageSize = 20

type AuditHandler struct {
	auditService *service.AuditService
}

type auditLogResponse struct {
	ID           uint            `json:"id"`
	UserID       uint            `json:"user_id"`
	Username     string          `json:"username"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uint            `json:"resource_id"`
	Detail       json.RawMessage `json:"detail"`
	IPAddress    string          `json:"ip_address"`
	CreatedAt    string          `json:"created_at"`
}

type listAuditLogsResponse struct {
	Items    []auditLogResponse `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func (h *AuditHandler) List(c *gin.Context) {
	request, err := buildListAuditLogsRequest(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	logs, total, err := h.auditService.List(c.Request.Context(), request)
	if err != nil {
		if service.IsValidationError(err) {
			writeError(c, http.StatusBadRequest, err.Error())
			return
		}
		writeError(c, http.StatusInternalServerError, "list audit logs failed")
		return
	}

	responses := make([]auditLogResponse, 0, len(logs))
	for _, log := range logs {
		responses = append(responses, auditLogResponse{
			ID:           log.ID,
			UserID:       log.UserID,
			Username:     log.User.Username,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			ResourceID:   log.ResourceID,
			Detail:       toAuditLogDetail(log.Detail),
			IPAddress:    log.IPAddress,
			CreatedAt:    log.CreatedAt.UTC().Format(http.TimeFormat),
		})
	}

	c.JSON(http.StatusOK, listAuditLogsResponse{
		Items:    responses,
		Total:    total,
		Page:     request.Page,
		PageSize: request.PageSize,
	})
}

func buildListAuditLogsRequest(c *gin.Context) (service.ListAuditLogsRequest, error) {
	request := service.ListAuditLogsRequest{
		Action:       strings.TrimSpace(c.Query("action")),
		ResourceType: strings.TrimSpace(c.Query("resource_type")),
		Page:         1,
		PageSize:     defaultAuditLogsPageSize,
	}

	if userIDValue := strings.TrimSpace(c.Query("user_id")); userIDValue != "" {
		parsedUserID, err := strconv.ParseUint(userIDValue, 10, 64)
		if err != nil {
			return service.ListAuditLogsRequest{}, service.ErrAuditLogQueryInvalid
		}
		userID := uint(parsedUserID)
		request.UserID = &userID
	}

	if startTimeValue := strings.TrimSpace(c.Query("start_time")); startTimeValue != "" {
		parsedStartTime, err := time.Parse(time.RFC3339, startTimeValue)
		if err != nil {
			return service.ListAuditLogsRequest{}, service.ErrAuditLogQueryInvalid
		}
		request.StartTime = &parsedStartTime
	}
	if endTimeValue := strings.TrimSpace(c.Query("end_time")); endTimeValue != "" {
		parsedEndTime, err := time.Parse(time.RFC3339, endTimeValue)
		if err != nil {
			return service.ListAuditLogsRequest{}, service.ErrAuditLogQueryInvalid
		}
		request.EndTime = &parsedEndTime
	}
	if pageValue := strings.TrimSpace(c.Query("page")); pageValue != "" {
		parsedPage, err := strconv.Atoi(pageValue)
		if err != nil {
			return service.ListAuditLogsRequest{}, service.ErrAuditLogQueryInvalid
		}
		request.Page = parsedPage
	}
	if pageSizeValue := strings.TrimSpace(c.Query("page_size")); pageSizeValue != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeValue)
		if err != nil {
			return service.ListAuditLogsRequest{}, service.ErrAuditLogQueryInvalid
		}
		request.PageSize = parsedPageSize
	}

	return request, nil
}

func toAuditLogDetail(encodedDetail string) json.RawMessage {
	if strings.TrimSpace(encodedDetail) == "" {
		return json.RawMessage(`{}`)
	}

	return json.RawMessage(encodedDetail)
}