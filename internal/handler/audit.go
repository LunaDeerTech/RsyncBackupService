package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/store"
)

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	query, err := buildAuditLogQuery(r, nil)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	items, total, err := h.db.ListAuditLogs(query)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list audit logs")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       query.Pagination.Page,
		PageSize:   query.Pagination.PageSize,
		TotalPages: totalPages(total, query.Pagination.PageSize),
	})
}

func (h *Handler) ListInstanceAuditLogs(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	instanceID, err := instanceIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid instance id")
		return
	}
	if _, err := h.db.GetInstanceByID(instanceID); err != nil {
		writeInstanceError(w, err, "failed to query instance")
		return
	}

	query, err := buildAuditLogQuery(r, &instanceID)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}

	items, total, err := h.db.ListAuditLogs(query)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list audit logs")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       query.Pagination.Page,
		PageSize:   query.Pagination.PageSize,
		TotalPages: totalPages(total, query.Pagination.PageSize),
	})
}

func (h *Handler) writeAuditLog(ctx context.Context, instanceID, userID int64, action string, detail any) {
	if h == nil || h.audit == nil {
		return
	}
	if err := h.audit.LogAction(ctx, instanceID, userID, action, detail); err != nil {
		slog.Error("write audit log failed", "action", action, "instance_id", instanceID, "user_id", userID, "error", err)
	}
}

func (h *Handler) writeCurrentUserAudit(r *http.Request, instanceID int64, action string, detail any) {
	if r == nil {
		return
	}
	userID := int64(0)
	if claims := middleware.GetUser(r.Context()); claims != nil {
		userID = claims.UserID
	}
	h.writeAuditLog(r.Context(), instanceID, userID, action, detail)
}

func buildAuditLogQuery(r *http.Request, instanceID *int64) (store.AuditLogQuery, error) {
	pagination := ParsePagination(r)
	query := store.AuditLogQuery{
		InstanceID: instanceID,
		Pagination: store.Pagination{Page: pagination.Page, PageSize: pagination.PageSize},
		Actions:    parseAuditActions(r.URL.Query().Get("action")),
	}

	startDate, err := parseAuditDate(r.URL.Query().Get("start_date"), false)
	if err != nil {
		return store.AuditLogQuery{}, fmt.Errorf("invalid start_date")
	}
	endDate, err := parseAuditDate(r.URL.Query().Get("end_date"), true)
	if err != nil {
		return store.AuditLogQuery{}, fmt.Errorf("invalid end_date")
	}
	if startDate != nil {
		query.StartDate = startDate
	}
	if endDate != nil {
		query.EndDate = endDate
	}
	if query.StartDate != nil && query.EndDate != nil && query.StartDate.After(*query.EndDate) {
		return store.AuditLogQuery{}, fmt.Errorf("start_date must be before or equal to end_date")
	}

	return query, nil
}

func parseAuditActions(raw string) []string {
	parts := strings.Split(raw, ",")
	actions := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		actions = append(actions, trimmed)
	}
	return actions
}

func parseAuditDate(raw string, endOfDay bool) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		utc := parsed.UTC()
		if endOfDay {
			end := utc.Add(24*time.Hour - time.Nanosecond)
			return &end, nil
		}
		return &utc, nil
	}

	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			utc := parsed.UTC()
			return &utc, nil
		}
	}

	return nil, fmt.Errorf("invalid date")
}
