package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	notificationService *service.NotificationService
}

type notificationChannelResponse struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Enabled   bool            `json:"enabled"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
}

type notificationChannelSummary struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type notificationChannelOptionResponse struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type notificationSubscriptionResponse struct {
	ID            uint                       `json:"id"`
	UserID        uint                       `json:"user_id"`
	InstanceID    uint                       `json:"instance_id"`
	ChannelID     uint                       `json:"channel_id"`
	Channel       notificationChannelSummary `json:"channel"`
	Events        []string                   `json:"events"`
	ChannelConfig json.RawMessage            `json:"channel_config"`
	Enabled       bool                       `json:"enabled"`
	CreatedAt     string                     `json:"created_at"`
}

func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

func (h *NotificationHandler) ListChannels(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}

	channels, err := h.notificationService.ListChannels(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list notification channels failed")
		return
	}
	if !user.IsAdmin {
		responses := make([]notificationChannelOptionResponse, 0, len(channels))
		for _, channel := range channels {
			if !channel.Enabled {
				continue
			}
			responses = append(responses, notificationChannelOptionResponse{
				ID:      channel.ID,
				Name:    channel.Name,
				Type:    channel.Type,
				Enabled: channel.Enabled,
			})
		}
		c.JSON(http.StatusOK, responses)
		return
	}

	responses := make([]notificationChannelResponse, 0, len(channels))
	for _, channel := range channels {
		response, err := toNotificationChannelResponse(channel)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "sanitize notification channel failed")
			return
		}
		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *NotificationHandler) CreateChannel(c *gin.Context) {
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_channels.create", ResourceType: "notification_channels"})

	var request service.CreateNotificationChannelRequest
	if !bindJSON(c, &request) {
		return
	}

	channel, err := h.notificationService.CreateChannel(c.Request.Context(), request)
	if err != nil {
		h.writeNotificationError(c, err, "create notification channel failed")
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_channels.create", ResourceType: "notification_channels", ResourceID: channel.ID})

	response, err := toNotificationChannelResponse(channel)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "sanitize notification channel failed")
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h *NotificationHandler) UpdateChannel(c *gin.Context) {
	channelID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_channels.update", ResourceType: "notification_channels", ResourceID: channelID})

	var request service.UpdateNotificationChannelRequest
	if !bindJSON(c, &request) {
		return
	}

	channel, err := h.notificationService.UpdateChannel(c.Request.Context(), channelID, request)
	if err != nil {
		h.writeNotificationError(c, err, "update notification channel failed")
		return
	}

	response, err := toNotificationChannelResponse(channel)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "sanitize notification channel failed")
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *NotificationHandler) DeleteChannel(c *gin.Context) {
	channelID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_channels.delete", ResourceType: "notification_channels", ResourceID: channelID})

	if err := h.notificationService.DeleteChannel(c.Request.Context(), channelID); err != nil {
		h.writeNotificationError(c, err, "delete notification channel failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) TestChannel(c *gin.Context) {
	channelID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_channels.test", ResourceType: "notification_channels", ResourceID: channelID})

	var request service.TestNotificationChannelRequest
	if !bindJSON(c, &request) {
		return
	}

	if err := h.notificationService.TestChannel(c.Request.Context(), channelID, request); err != nil {
		h.writeNotificationError(c, err, "test notification channel failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) ListSubscriptions(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}
	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	subscriptions, err := h.notificationService.ListSubscriptions(c.Request.Context(), user, instanceID)
	if err != nil {
		h.writeNotificationError(c, err, "list notification subscriptions failed")
		return
	}

	responses := make([]notificationSubscriptionResponse, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		response, err := toNotificationSubscriptionResponse(subscription)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "decode notification subscription failed")
			return
		}
		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *NotificationHandler) UpsertSubscription(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}
	instanceID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_subscriptions.upsert", ResourceType: "notification_subscriptions", ResourceID: instanceID})

	var request service.UpsertNotificationSubscriptionRequest
	if !bindJSON(c, &request) {
		return
	}

	subscription, created, err := h.notificationService.UpsertSubscription(c.Request.Context(), user, instanceID, request)
	if err != nil {
		h.writeNotificationError(c, err, "upsert notification subscription failed")
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_subscriptions.upsert", ResourceType: "notification_subscriptions", ResourceID: subscription.ID, Detail: map[string]any{"instance_id": instanceID, "channel_id": subscription.ChannelID}})

	response, err := toNotificationSubscriptionResponse(subscription)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "decode notification subscription failed")
		return
	}

	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}
	c.JSON(statusCode, response)
}

func (h *NotificationHandler) DeleteSubscription(c *gin.Context) {
	user, ok := currentAuthUser(c)
	if !ok {
		return
	}
	subscriptionID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{Action: "notification_subscriptions.delete", ResourceType: "notification_subscriptions", ResourceID: subscriptionID})

	if err := h.notificationService.DeleteSubscription(c.Request.Context(), user, subscriptionID); err != nil {
		h.writeNotificationError(c, err, "delete notification subscription failed")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *NotificationHandler) writeNotificationError(c *gin.Context, err error, fallbackMessage string) {
	switch {
	case errors.Is(err, service.ErrNotificationChannelNotFound), errors.Is(err, service.ErrNotificationSubscriptionNotFound), errors.Is(err, service.ErrInstanceNotFound):
		writeError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrPermissionDenied):
		writeError(c, http.StatusForbidden, err.Error())
	case service.IsValidationError(err):
		writeError(c, http.StatusBadRequest, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, fallbackMessage)
	}
}

func toNotificationChannelResponse(channel model.NotificationChannel) (notificationChannelResponse, error) {
	config, err := service.RedactNotificationChannelConfig(channel.Type, channel.Config)
	if err != nil {
		return notificationChannelResponse{}, err
	}

	return notificationChannelResponse{
		ID:        channel.ID,
		Name:      channel.Name,
		Type:      channel.Type,
		Config:    config,
		Enabled:   channel.Enabled,
		CreatedAt: channel.CreatedAt.UTC().Format(http.TimeFormat),
		UpdatedAt: channel.UpdatedAt.UTC().Format(http.TimeFormat),
	}, nil
}

func toNotificationSubscriptionResponse(subscription model.NotificationSubscription) (notificationSubscriptionResponse, error) {
	var events []string
	if strings := subscription.Events; strings != "" {
		if err := json.Unmarshal([]byte(strings), &events); err != nil {
			return notificationSubscriptionResponse{}, err
		}
	}

	channelConfig := json.RawMessage(`{}`)
	if subscription.ChannelConfig != "" {
		channelConfig = json.RawMessage(subscription.ChannelConfig)
	}

	return notificationSubscriptionResponse{
		ID:         subscription.ID,
		UserID:     subscription.UserID,
		InstanceID: subscription.InstanceID,
		ChannelID:  subscription.ChannelID,
		Channel: notificationChannelSummary{
			ID:      subscription.Channel.ID,
			Name:    subscription.Channel.Name,
			Type:    subscription.Channel.Type,
			Enabled: subscription.Channel.Enabled,
		},
		Events:        events,
		ChannelConfig: channelConfig,
		Enabled:       subscription.Enabled,
		CreatedAt:     subscription.CreatedAt.UTC().Format(http.TimeFormat),
	}, nil
}