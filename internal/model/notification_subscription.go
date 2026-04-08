package model

import "time"

type NotificationSubscription struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	InstanceID   int64     `json:"instance_id"`
	InstanceName string    `json:"instance_name,omitempty"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}