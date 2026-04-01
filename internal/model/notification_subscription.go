package model

import "time"

type NotificationSubscription struct {
	ID            uint `gorm:"primaryKey"`
	UserID        uint
	User          User
	InstanceID    uint
	Instance      BackupInstance
	ChannelID     uint
	Channel       NotificationChannel
	Events        string
	ChannelConfig string
	Enabled       bool
	CreatedAt     time.Time
}
