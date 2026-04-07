package model

import "time"

type Instance struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	SourceType     string    `json:"source_type"`
	SourcePath     string    `json:"source_path"`
	RemoteConfigID *int64    `json:"remote_config_id,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Backup struct {
	ID              int64      `json:"id"`
	InstanceID      int64      `json:"instance_id"`
	PolicyID        int64      `json:"policy_id"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	SnapshotPath    string     `json:"snapshot_path"`
	BackupSizeBytes int64      `json:"backup_size_bytes"`
	ActualSizeBytes int64      `json:"actual_size_bytes"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds int64      `json:"duration_seconds"`
	ErrorMessage    string     `json:"error_message"`
	RsyncStats      string     `json:"rsync_stats"`
	CreatedAt       time.Time  `json:"created_at"`
}

type BackupTrendPoint struct {
	Date         string `json:"date"`
	Count        int64  `json:"count"`
	SuccessCount int64  `json:"success_count"`
	FailureCount int64  `json:"failure_count"`
}

type InstanceStats struct {
	BackupCount          int64              `json:"backup_count"`
	SuccessBackupCount   int64              `json:"success_backup_count"`
	FailureBackupCount   int64              `json:"failure_backup_count"`
	TotalBackupSizeBytes int64              `json:"total_backup_size_bytes"`
	PolicyCount          int64              `json:"policy_count"`
	LastBackup           *Backup            `json:"last_backup,omitempty"`
	RecentTrend          []BackupTrendPoint `json:"recent_trend"`
}