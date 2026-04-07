package model

import (
	"encoding/json"
	"time"
)

type AuditLog struct {
	ID         int64           `json:"id"`
	InstanceID *int64          `json:"instance_id,omitempty"`
	UserID     *int64          `json:"user_id,omitempty"`
	Action     string          `json:"action"`
	Detail     json.RawMessage `json:"detail"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AuditLogWithUser struct {
	AuditLog
	UserName  string `json:"user_name"`
	UserEmail string `json:"user_email"`
}
