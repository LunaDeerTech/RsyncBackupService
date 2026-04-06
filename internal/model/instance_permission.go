package model

import "time"

type InstancePermission struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	InstanceID int64     `json:"instance_id"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}
