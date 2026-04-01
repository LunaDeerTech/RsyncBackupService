package model

import "time"

type RestoreRecord struct {
	ID                uint `gorm:"primaryKey"`
	InstanceID        uint
	Instance          BackupInstance
	BackupRecordID    uint
	BackupRecord      BackupRecord
	RestoreTargetPath string
	Overwrite         bool
	Status            string
	StartedAt         time.Time
	FinishedAt        *time.Time
	ErrorMessage      string
	TriggeredBy       uint
	TriggeredByUser   User `gorm:"foreignKey:TriggeredBy"`
}
