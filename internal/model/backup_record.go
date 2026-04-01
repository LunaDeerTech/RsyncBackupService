package model

import "time"

type BackupRecord struct {
	ID               uint `gorm:"primaryKey"`
	InstanceID       uint
	Instance         BackupInstance
	StorageTargetID  uint
	StorageTarget    StorageTarget
	StrategyID       *uint
	Strategy         *Strategy
	BackupType       string
	Status           string
	SnapshotPath     string
	BytesTransferred int64
	FilesTransferred int64
	TotalSize        int64
	VolumeCount      int
	StartedAt        time.Time
	FinishedAt       *time.Time
	ErrorMessage     string
}
