package model

import "time"

type RemoteConfig struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Username       string    `json:"username"`
	PrivateKeyPath string    `json:"-"`
	CloudProvider  *string   `json:"cloud_provider,omitempty"`
	CloudConfig    *string   `json:"cloud_config,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
