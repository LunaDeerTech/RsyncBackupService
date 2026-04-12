package handler

import (
	"net/http"
	"sort"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
)

type registrationStatusResponse struct {
	Enabled bool `json:"enabled"`
}

type registrationUpdateRequest struct {
	Enabled bool `json:"enabled"`
}

type smtpTestRequest struct {
	To string `json:"to"`
}

type subscriptionsRequest struct {
	Subscriptions []subscriptionUpdateItem `json:"subscriptions"`
}

type subscriptionUpdateItem struct {
	InstanceID int64 `json:"instance_id"`
	Enabled    bool  `json:"enabled"`
}

type subscriptionsResponse struct {
	Subscriptions []model.NotificationSubscription `json:"subscriptions"`
}

func (h *Handler) GetRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	enabled, err := h.registrationEnabled()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query registration status")
		return
	}
	JSON(w, http.StatusOK, registrationStatusResponse{Enabled: enabled})
}

func (h *Handler) UpdateRegistrationStatus(w http.ResponseWriter, r *http.Request) {
	if h.systemConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "system config service unavailable")
		return
	}
	var request registrationUpdateRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}
	if err := h.systemConfigs.UpdateRegistrationEnabled(request.Enabled); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update registration status")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionSystemConfigUpdate, map[string]any{"registration_enabled": request.Enabled})
	JSON(w, http.StatusOK, registrationStatusResponse{Enabled: request.Enabled})
}

func (h *Handler) GetSMTPConfig(w http.ResponseWriter, r *http.Request) {
	if h.systemConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "system config service unavailable")
		return
	}
	config, err := h.systemConfigs.GetSMTPConfig()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load smtp config")
		return
	}
	JSON(w, http.StatusOK, config)
}

func (h *Handler) UpdateSMTPConfig(w http.ResponseWriter, r *http.Request) {
	if h.systemConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "system config service unavailable")
		return
	}
	var request service.SMTPConfig
	if !decodeRequestBody(w, r, &request) {
		return
	}
	if err := h.systemConfigs.UpdateSMTPConfig(&request); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionSystemConfigUpdate, map[string]any{
		"smtp_host":                request.Host,
		"smtp_port":                request.Port,
		"smtp_username":            request.Username,
		"smtp_from":                request.From,
		"smtp_password_configured": request.Password != "",
	})
	config, err := h.systemConfigs.GetSMTPConfig()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load smtp config")
		return
	}
	JSON(w, http.StatusOK, config)
}

func (h *Handler) TestSMTP(w http.ResponseWriter, r *http.Request) {
	if h.systemConfigs == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "system config service unavailable")
		return
	}
	var request smtpTestRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}
	config, err := h.systemConfigs.GetSMTPConfigWithPassword()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to load smtp config")
		return
	}
	if err := h.systemConfigs.TestSMTP(config, request.To); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]string{"message": "test email sent"})
}

func (h *Handler) GetCurrentUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	user, err := h.currentUser(r)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}
	instances, err := h.visibleInstancesForCurrentUser(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list instances")
		return
	}
	current, err := h.db.ListSubscriptionsByUser(user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list subscriptions")
		return
	}
	JSON(w, http.StatusOK, subscriptionsResponse{Subscriptions: mergeVisibleSubscriptions(instances, current, user.ID)})
}

func (h *Handler) UpdateCurrentUserSubscriptions(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}
	user, err := h.currentUser(r)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}
	instances, err := h.visibleInstancesForCurrentUser(r)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list instances")
		return
	}
	allowed := make(map[int64]model.Instance, len(instances))
	for _, instance := range instances {
		allowed[instance.ID] = instance
	}

	var request subscriptionsRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}
	current, err := h.db.ListSubscriptionsByUser(user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list subscriptions")
		return
	}

	merged := make(map[int64]bool, len(current)+len(request.Subscriptions))
	for _, item := range current {
		merged[item.InstanceID] = item.Enabled
	}
	for _, item := range request.Subscriptions {
		if _, ok := allowed[item.InstanceID]; !ok {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "subscription instance is not accessible")
			return
		}
		merged[item.InstanceID] = item.Enabled
	}
	subscriptions := make([]model.NotificationSubscription, 0, len(merged))
	for instanceID, enabled := range merged {
		subscriptions = append(subscriptions, model.NotificationSubscription{InstanceID: instanceID, Enabled: enabled})
	}
	if err := h.db.UpdateSubscriptions(user.ID, subscriptions); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update subscriptions")
		return
	}
	current, err = h.db.ListSubscriptionsByUser(user.ID)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list subscriptions")
		return
	}
	JSON(w, http.StatusOK, subscriptionsResponse{Subscriptions: mergeVisibleSubscriptions(instances, current, user.ID)})
}

func (h *Handler) visibleInstancesForCurrentUser(r *http.Request) ([]model.Instance, error) {
	claims := middleware.MustGetUser(r.Context())
	if claims.Role == "admin" {
		return h.db.ListInstances()
	}
	return h.db.ListInstancesByUserPermission(claims.UserID)
}

func mergeVisibleSubscriptions(instances []model.Instance, current []model.NotificationSubscription, userID int64) []model.NotificationSubscription {
	currentByInstance := make(map[int64]model.NotificationSubscription, len(current))
	for _, subscription := range current {
		currentByInstance[subscription.InstanceID] = subscription
	}
	merged := make([]model.NotificationSubscription, 0, len(instances))
	for _, instance := range instances {
		subscription := model.NotificationSubscription{
			UserID:       userID,
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			Enabled:      false,
		}
		if currentSub, ok := currentByInstance[instance.ID]; ok {
			subscription.ID = currentSub.ID
			subscription.Enabled = currentSub.Enabled
			subscription.CreatedAt = currentSub.CreatedAt
		}
		merged = append(merged, subscription)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].InstanceID < merged[j].InstanceID
	})
	return merged
}