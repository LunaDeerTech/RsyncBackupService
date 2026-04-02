package model

import "time"

const (
	BackupStatusRunning   = "running"
	BackupStatusSuccess   = "success"
	BackupStatusFailed    = "failed"
	BackupStatusCancelled = "cancelled"
)

type BackupRecord struct {
	ID               uint `gorm:"primaryKey"`
	InstanceID       uint `gorm:"index"`
	Instance         BackupInstance
	StorageTargetID  uint `gorm:"index"`
	StorageTarget    StorageTarget
	StrategyID       *uint `gorm:"index"`
	Strategy         *Strategy
	BackupType       string
	Status           string `gorm:"index"`
	TargetLocationKey string `gorm:"index"`
	SnapshotPath     string
	BytesTransferred int64
	FilesTransferred int64
	TotalSize        int64
	VolumeCount      int
	StartedAt        time.Time
	FinishedAt       *time.Time
	ErrorMessage     string
}
