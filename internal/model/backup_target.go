package model

import "time"

type BackupTarget struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	BackupType         string     `json:"backup_type"`
	StorageType        string     `json:"storage_type"`
	StoragePath        string     `json:"storage_path"`
	RemoteConfigID     *int64     `json:"remote_config_id,omitempty"`
	TotalCapacityBytes *int64     `json:"total_capacity_bytes,omitempty"`
	UsedCapacityBytes  *int64     `json:"used_capacity_bytes,omitempty"`
	LastHealthCheck    *time.Time `json:"last_health_check,omitempty"`
	HealthStatus       string     `json:"health_status"`
	HealthMessage      string     `json:"health_message"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
