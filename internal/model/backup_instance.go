package model

import "time"

type BackupInstance struct {
	ID                        uint `gorm:"primaryKey"`
	Name                      string
	SourceType                string
	SourceHost                string
	SourcePort                int
	SourceUser                string
	SourceSSHKeyID            *uint
	SourceSSHKey              *SSHKey
	SourcePath                string
	ExcludePatterns           string
	Enabled                   bool
	CreatedBy                 uint
	Creator                   User                       `gorm:"foreignKey:CreatedBy"`
	Strategies                []Strategy                 `gorm:"foreignKey:InstanceID"`
	BackupRecords             []BackupRecord             `gorm:"foreignKey:InstanceID"`
	RestoreRecords            []RestoreRecord            `gorm:"foreignKey:InstanceID"`
	NotificationSubscriptions []NotificationSubscription `gorm:"foreignKey:InstanceID"`
	InstancePermissions       []InstancePermission       `gorm:"foreignKey:InstanceID"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}
