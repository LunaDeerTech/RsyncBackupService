package model

import "time"

type Strategy struct {
	ID                  uint `gorm:"primaryKey"`
	InstanceID          uint
	Instance            BackupInstance
	Name                string
	BackupType          string
	CronExpr            *string
	IntervalSeconds     int
	RetentionDays       int
	RetentionCount      int
	ColdVolumeSize      *string
	MaxExecutionSeconds int
	Enabled             bool
	StorageTargets      []StorageTarget `gorm:"many2many:strategy_storage_bindings;"`
	BackupRecords       []BackupRecord  `gorm:"foreignKey:StrategyID"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
