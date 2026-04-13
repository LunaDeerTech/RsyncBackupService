package model

import "time"

type APIKey struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id,omitempty"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	KeyHash    string     `json:"-"`
	Key        string     `json:"key,omitempty"`
}
