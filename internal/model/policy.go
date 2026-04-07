package model

import "time"

type Policy struct {
	ID                int64     `json:"id"`
	InstanceID        int64     `json:"instance_id"`
	Name              string    `json:"name"`
	Type              string    `json:"type"`
	TargetID          int64     `json:"target_id"`
	ScheduleType      string    `json:"schedule_type"`
	ScheduleValue     string    `json:"schedule_value"`
	Enabled           bool      `json:"enabled"`
	Compression       bool      `json:"compression"`
	Encryption        bool      `json:"encryption"`
	EncryptionKeyHash *string   `json:"-"`
	SplitEnabled      bool      `json:"split_enabled"`
	SplitSizeMB       *int      `json:"split_size_mb,omitempty"`
	RetentionType     string    `json:"retention_type"`
	RetentionValue    int       `json:"retention_value"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PolicyExecutionSummary struct {
	LastExecutionTime   *time.Time `json:"last_execution_time,omitempty"`
	LastExecutionStatus *string    `json:"last_execution_status,omitempty"`
	LatestBackupID      *int64     `json:"latest_backup_id,omitempty"`
}
