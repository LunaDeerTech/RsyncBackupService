package model

import "time"

type NotificationChannel struct {
	ID            uint `gorm:"primaryKey"`
	Name          string
	Type          string
	Config        string
	Enabled       bool
	Subscriptions []NotificationSubscription `gorm:"foreignKey:ChannelID"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
