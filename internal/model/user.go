package model

import "time"

type User struct {
	ID                        uint   `gorm:"primaryKey"`
	Username                  string `gorm:"uniqueIndex"`
	PasswordHash              string
	IsAdmin                   bool
	BackupInstancesCreated    []BackupInstance           `gorm:"foreignKey:CreatedBy"`
	NotificationSubscriptions []NotificationSubscription `gorm:"foreignKey:UserID"`
	InstancePermissions       []InstancePermission       `gorm:"foreignKey:UserID"`
	AuditLogs                 []AuditLog                 `gorm:"foreignKey:UserID"`
	RestoreRecordsTriggered   []RestoreRecord            `gorm:"foreignKey:TriggeredBy"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}
