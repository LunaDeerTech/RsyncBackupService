package model

import "time"

type NotificationSubscription struct {
	ID            uint `gorm:"primaryKey"`
	UserID        uint `gorm:"uniqueIndex:idx_notification_subscriptions_user_instance_channel"`
	User          User
	InstanceID    uint `gorm:"uniqueIndex:idx_notification_subscriptions_user_instance_channel"`
	Instance      BackupInstance
	ChannelID     uint `gorm:"uniqueIndex:idx_notification_subscriptions_user_instance_channel"`
	Channel       NotificationChannel
	Events        string
	ChannelConfig string
	Enabled       bool
	CreatedAt     time.Time
}
