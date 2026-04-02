package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/notify"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const notificationEventTest = "notification_test"

var supportedNotificationEvents = map[string]struct{}{
	"backup_success":   {},
	"backup_failed":    {},
	"restore_complete": {},
	"restore_failed":   {},
}

type notificationNotifierBuilder func(channel model.NotificationChannel, subscription model.NotificationSubscription) (notify.Notifier, error)

type CreateNotificationChannelRequest struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled bool            `json:"enabled"`
}

type UpdateNotificationChannelRequest = CreateNotificationChannelRequest

type TestNotificationChannelRequest struct {
	ChannelConfig json.RawMessage `json:"channel_config"`
}

type UpsertNotificationSubscriptionRequest struct {
	ChannelID     uint            `json:"channel_id"`
	Events        []string        `json:"events"`
	ChannelConfig json.RawMessage `json:"channel_config"`
	Enabled       bool            `json:"enabled"`
}

type NotificationService struct {
	db                *gorm.DB
	permissionService *PermissionService
	clock             func() time.Time
	buildNotifier     notificationNotifierBuilder
}

func NewNotificationService(db *gorm.DB) *NotificationService {
	service := &NotificationService{
		db:                db,
		permissionService: NewPermissionService(db),
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
	service.buildNotifier = service.defaultBuildNotifier

	return service
}

func (s *NotificationService) ListChannels(ctx context.Context) ([]model.NotificationChannel, error) {
	var channels []model.NotificationChannel
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("list notification channels: %w", err)
	}

	return channels, nil
}

func (s *NotificationService) CreateChannel(ctx context.Context, req CreateNotificationChannelRequest) (model.NotificationChannel, error) {
	channel, err := s.buildNotificationChannelModel(nil, req)
	if err != nil {
		return model.NotificationChannel{}, err
	}

	if err := s.db.WithContext(ctx).Create(&channel).Error; err != nil {
		return model.NotificationChannel{}, fmt.Errorf("create notification channel: %w", err)
	}

	return channel, nil
}

func (s *NotificationService) UpdateChannel(ctx context.Context, id uint, req UpdateNotificationChannelRequest) (model.NotificationChannel, error) {
	channel, err := s.findChannel(ctx, id)
	if err != nil {
		return model.NotificationChannel{}, err
	}

	updatedChannel, err := s.buildNotificationChannelModel(&channel, req)
	if err != nil {
		return model.NotificationChannel{}, err
	}

	channel.Name = updatedChannel.Name
	channel.Type = updatedChannel.Type
	channel.Config = updatedChannel.Config
	channel.Enabled = updatedChannel.Enabled

	if err := s.db.WithContext(ctx).Save(&channel).Error; err != nil {
		return model.NotificationChannel{}, fmt.Errorf("update notification channel: %w", err)
	}

	return channel, nil
}

func (s *NotificationService) DeleteChannel(ctx context.Context, id uint) error {
	if _, err := s.findChannel(ctx, id); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", id).Delete(&model.NotificationSubscription{}).Error; err != nil {
			return fmt.Errorf("delete notification subscriptions by channel: %w", err)
		}
		if err := tx.Delete(&model.NotificationChannel{}, id).Error; err != nil {
			return fmt.Errorf("delete notification channel: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *NotificationService) TestChannel(ctx context.Context, id uint, req TestNotificationChannelRequest) error {
	channel, err := s.findChannel(ctx, id)
	if err != nil {
		return err
	}

	notifierInstance, err := s.buildNotifier(channel, model.NotificationSubscription{ChannelConfig: string(req.ChannelConfig), Enabled: true})
	if err != nil {
		return err
	}

	return notifierInstance.Send(ctx, notify.NotifyEvent{
		Type:       notificationEventTest,
		Instance:   "system",
		Message:    "Notification channel test",
		OccurredAt: s.clock(),
	})
}

func (s *NotificationService) ListSubscriptions(ctx context.Context, actor AuthIdentity, instanceID uint) ([]model.NotificationSubscription, error) {
	if err := s.requireInstanceRole(ctx, actor, instanceID, RoleViewer); err != nil {
		return nil, err
	}

	var subscriptions []model.NotificationSubscription
	if err := s.db.WithContext(ctx).
		Preload("Channel").
		Where("instance_id = ? AND user_id = ?", instanceID, actor.UserID).
		Order("id ASC").
		Find(&subscriptions).Error; err != nil {
		return nil, fmt.Errorf("list notification subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (s *NotificationService) UpsertSubscription(ctx context.Context, actor AuthIdentity, instanceID uint, req UpsertNotificationSubscriptionRequest) (model.NotificationSubscription, bool, error) {
	if err := s.requireInstanceRole(ctx, actor, instanceID, RoleViewer); err != nil {
		return model.NotificationSubscription{}, false, err
	}

	channel, err := s.findChannel(ctx, req.ChannelID)
	if err != nil {
		return model.NotificationSubscription{}, false, err
	}
	if _, err := s.buildNotifier(channel, model.NotificationSubscription{ChannelConfig: string(req.ChannelConfig), Enabled: req.Enabled}); err != nil {
		return model.NotificationSubscription{}, false, err
	}

	encodedEvents, err := encodeNotificationEvents(req.Events)
	if err != nil {
		return model.NotificationSubscription{}, false, err
	}
	encodedChannelConfig, err := encodeNotificationSubscriptionConfig(channel.Type, req.ChannelConfig)
	if err != nil {
		return model.NotificationSubscription{}, false, err
	}

	subscription := model.NotificationSubscription{
		UserID:        actor.UserID,
		InstanceID:    instanceID,
		ChannelID:     channel.ID,
		Events:        encodedEvents,
		ChannelConfig: encodedChannelConfig,
		Enabled:       req.Enabled,
	}

	created := false
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing model.NotificationSubscription
		err := tx.Where("user_id = ? AND instance_id = ? AND channel_id = ?", actor.UserID, instanceID, channel.ID).First(&existing).Error
		if err == nil {
			subscription.ID = existing.ID
			created = false
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			created = true
		} else {
			return fmt.Errorf("find existing notification subscription: %w", err)
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "instance_id"}, {Name: "channel_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"events", "channel_config", "enabled"}),
		}).Create(&subscription).Error; err != nil {
			return fmt.Errorf("upsert notification subscription: %w", err)
		}

		return nil
	}); err != nil {
		return model.NotificationSubscription{}, false, err
	}

	storedSubscription, err := s.findSubscription(ctx, subscription.ID, actor)
	if err != nil {
		return model.NotificationSubscription{}, false, err
	}

	return storedSubscription, created, nil
}

func (s *NotificationService) DeleteSubscription(ctx context.Context, actor AuthIdentity, subscriptionID uint) error {
	subscription, err := s.findSubscription(ctx, subscriptionID, actor)
	if err != nil {
		return err
	}

	if !actor.IsAdmin && subscription.UserID != actor.UserID {
		return ErrPermissionDenied
	}

	if err := s.db.WithContext(ctx).Delete(&model.NotificationSubscription{}, subscription.ID).Error; err != nil {
		return fmt.Errorf("delete notification subscription: %w", err)
	}

	return nil
}

func (s *NotificationService) Notify(ctx context.Context, event notify.NotifyEvent) error {
	if _, ok := supportedNotificationEvents[strings.TrimSpace(event.Type)]; !ok {
		return fmt.Errorf("%w: unsupported event type %q", ErrInvalidNotificationEvent, event.Type)
	}

	instanceID, err := s.resolveEventInstanceID(ctx, event)
	if err != nil {
		return err
	}

	instance, err := s.findInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(event.Instance) == "" {
		event.Instance = instance.Name
	}

	var subscriptions []model.NotificationSubscription
	if err := s.db.WithContext(ctx).
		Preload("Channel").
		Preload("User").
		Where("instance_id = ? AND enabled = ?", instanceID, true).
		Order("id ASC").
		Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("list notification subscriptions by instance: %w", err)
	}

	var notifyErrors []error
	for _, subscription := range subscriptions {
		if !subscription.Enabled || !subscription.Channel.Enabled {
			continue
		}
		if !subscriptionSupportsEvent(subscription.Events, event.Type) {
			continue
		}

		allowed, err := s.subscriptionUserHasViewerAccess(ctx, subscription, instanceID)
		if err != nil {
			notifyErrors = append(notifyErrors, fmt.Errorf("check subscription permission %d: %w", subscription.ID, err))
			continue
		}
		if !allowed {
			continue
		}

		notifierInstance, err := s.buildNotifier(subscription.Channel, subscription)
		if err != nil {
			notifyErrors = append(notifyErrors, fmt.Errorf("build notifier for subscription %d: %w", subscription.ID, err))
			continue
		}
		if err := notifierInstance.Send(ctx, event); err != nil {
			notifyErrors = append(notifyErrors, fmt.Errorf("send notification for subscription %d: %w", subscription.ID, err))
		}
	}

	if len(notifyErrors) > 0 {
		return errors.Join(notifyErrors...)
	}

	return nil
}

func (s *NotificationService) subscriptionUserHasViewerAccess(ctx context.Context, subscription model.NotificationSubscription, instanceID uint) (bool, error) {
	if subscription.User.IsAdmin {
		return true, nil
	}

	var permissionCount int64
	if err := s.db.WithContext(ctx).
		Model(&model.InstancePermission{}).
		Where("user_id = ? AND instance_id = ? AND role IN ?", subscription.UserID, instanceID, []string{RoleViewer, RoleAdmin}).
		Count(&permissionCount).Error; err != nil {
		return false, fmt.Errorf("count instance permissions: %w", err)
	}

	return permissionCount > 0, nil
}

func RedactNotificationChannelConfig(channelType, rawConfig string) (json.RawMessage, error) {
	switch strings.TrimSpace(channelType) {
	case notify.TypeSMTP:
		return notify.RedactSMTPConfig(json.RawMessage(rawConfig))
	default:
		return nil, fmt.Errorf("%w: unsupported channel type %q", ErrInvalidNotificationChannelType, channelType)
	}
}

func decodeNotificationEvents(encodedEvents string) []string {
	if strings.TrimSpace(encodedEvents) == "" {
		return []string{}
	}

	var events []string
	if err := json.Unmarshal([]byte(encodedEvents), &events); err != nil {
		return []string{}
	}

	return events
}

func decodeNotificationChannelConfig(encodedConfig string) json.RawMessage {
	if strings.TrimSpace(encodedConfig) == "" {
		return json.RawMessage(`{}`)
	}

	return json.RawMessage(encodedConfig)
}

func (s *NotificationService) defaultBuildNotifier(channel model.NotificationChannel, subscription model.NotificationSubscription) (notify.Notifier, error) {
	switch strings.TrimSpace(channel.Type) {
	case notify.TypeSMTP:
		notifierInstance, err := notify.NewSMTPNotifier(json.RawMessage(channel.Config), json.RawMessage(subscription.ChannelConfig))
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidNotificationChannelConfig, err)
		}
		return notifierInstance, nil
	default:
		return nil, fmt.Errorf("%w: unsupported channel type %q", ErrInvalidNotificationChannelType, channel.Type)
	}
}

func (s *NotificationService) buildNotificationChannelModel(existing *model.NotificationChannel, req CreateNotificationChannelRequest) (model.NotificationChannel, error) {
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return model.NotificationChannel{}, ErrNameRequired
	}
	trimmedType := strings.TrimSpace(req.Type)
	if trimmedType != notify.TypeSMTP {
		return model.NotificationChannel{}, fmt.Errorf("%w: unsupported channel type %q", ErrInvalidNotificationChannelType, req.Type)
	}

	var encodedConfig []byte
	switch trimmedType {
	case notify.TypeSMTP:
		var currentConfig json.RawMessage
		if existing != nil {
			currentConfig = json.RawMessage(existing.Config)
		}
		mergedConfig, err := notify.MergeSMTPConfig(currentConfig, req.Config)
		if err != nil {
			return model.NotificationChannel{}, fmt.Errorf("%w: %w", ErrInvalidNotificationChannelConfig, err)
		}
		encodedConfig, err = json.Marshal(mergedConfig)
		if err != nil {
			return model.NotificationChannel{}, fmt.Errorf("encode smtp config: %w", err)
		}
	default:
		return model.NotificationChannel{}, fmt.Errorf("%w: unsupported channel type %q", ErrInvalidNotificationChannelType, req.Type)
	}

	return model.NotificationChannel{
		Name:    trimmedName,
		Type:    trimmedType,
		Config:  string(encodedConfig),
		Enabled: req.Enabled,
	}, nil
}

func encodeNotificationEvents(events []string) (string, error) {
	normalizedEvents := make([]string, 0, len(events))
	seenEvents := make(map[string]struct{}, len(events))
	for _, event := range events {
		trimmedEvent := strings.TrimSpace(event)
		if trimmedEvent == "" {
			continue
		}
		if _, ok := supportedNotificationEvents[trimmedEvent]; !ok {
			return "", fmt.Errorf("%w: unsupported event %q", ErrInvalidNotificationEvent, event)
		}
		if _, exists := seenEvents[trimmedEvent]; exists {
			continue
		}
		seenEvents[trimmedEvent] = struct{}{}
		normalizedEvents = append(normalizedEvents, trimmedEvent)
	}
	if len(normalizedEvents) == 0 {
		return "", fmt.Errorf("%w: at least one event is required", ErrInvalidNotificationEvent)
	}
	sort.Strings(normalizedEvents)

	encodedEvents, err := json.Marshal(normalizedEvents)
	if err != nil {
		return "", fmt.Errorf("encode notification events: %w", err)
	}

	return string(encodedEvents), nil
}

func encodeNotificationSubscriptionConfig(channelType string, rawConfig json.RawMessage) (string, error) {
	switch strings.TrimSpace(channelType) {
	case notify.TypeSMTP:
		config, err := notify.ParseSMTPRecipientConfig(rawConfig)
		if err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalidNotificationSubscriptionConfig, err)
		}
		encodedConfig, err := json.Marshal(config)
		if err != nil {
			return "", fmt.Errorf("encode smtp recipient config: %w", err)
		}
		return string(encodedConfig), nil
	default:
		return "", fmt.Errorf("%w: unsupported channel type %q", ErrInvalidNotificationChannelType, channelType)
	}
}

func subscriptionSupportsEvent(encodedEvents, eventType string) bool {
	for _, event := range decodeNotificationEvents(encodedEvents) {
		if event == strings.TrimSpace(eventType) {
			return true
		}
	}

	return false
}

func (s *NotificationService) resolveEventInstanceID(ctx context.Context, event notify.NotifyEvent) (uint, error) {
	type instanceDetail struct {
		InstanceID uint `json:"instance_id"`
	}

	if event.Detail != nil {
		encodedDetail, err := json.Marshal(event.Detail)
		if err == nil {
			var detail instanceDetail
			if err := json.Unmarshal(encodedDetail, &detail); err == nil && detail.InstanceID != 0 {
				return detail.InstanceID, nil
			}
		}
	}

	trimmedInstance := strings.TrimSpace(event.Instance)
	if trimmedInstance == "" {
		return 0, ErrInstanceNotFound
	}

	var instances []model.BackupInstance
	if err := s.db.WithContext(ctx).Where("name = ?", trimmedInstance).Find(&instances).Error; err != nil {
		return 0, fmt.Errorf("list backup instances by name: %w", err)
	}
	if len(instances) != 1 {
		return 0, ErrInstanceNotFound
	}

	return instances[0].ID, nil
}

func (s *NotificationService) findChannel(ctx context.Context, id uint) (model.NotificationChannel, error) {
	var channel model.NotificationChannel
	if err := s.db.WithContext(ctx).First(&channel, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NotificationChannel{}, ErrNotificationChannelNotFound
		}
		return model.NotificationChannel{}, fmt.Errorf("find notification channel: %w", err)
	}

	return channel, nil
}

func (s *NotificationService) findSubscription(ctx context.Context, id uint, actor AuthIdentity) (model.NotificationSubscription, error) {
	var subscription model.NotificationSubscription
	if err := s.db.WithContext(ctx).Preload("Channel").First(&subscription, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.NotificationSubscription{}, ErrNotificationSubscriptionNotFound
		}
		return model.NotificationSubscription{}, fmt.Errorf("find notification subscription: %w", err)
	}
	if !actor.IsAdmin && subscription.UserID != actor.UserID {
		return model.NotificationSubscription{}, ErrPermissionDenied
	}

	return subscription, nil
}

func (s *NotificationService) findInstance(ctx context.Context, id uint) (model.BackupInstance, error) {
	var instance model.BackupInstance
	if err := s.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, ErrInstanceNotFound
		}
		return model.BackupInstance{}, fmt.Errorf("find backup instance: %w", err)
	}

	return instance, nil
}

func (s *NotificationService) requireInstanceRole(ctx context.Context, actor AuthIdentity, instanceID uint, role string) error {
	if actor.UserID == 0 && !actor.IsAdmin {
		return ErrUserRequired
	}
	if _, err := s.findInstance(ctx, instanceID); err != nil {
		return err
	}
	if actor.IsAdmin {
		return nil
	}

	allowed, err := s.permissionService.HasInstanceRole(ctx, actor, instanceID, role)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermissionDenied
	}

	return nil
}