package model

import "time"

type InstancePermission struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint `gorm:"uniqueIndex:idx_instance_permissions_user_instance"`
	User       User
	InstanceID uint `gorm:"uniqueIndex:idx_instance_permissions_user_instance"`
	Instance   BackupInstance
	Role       string
	CreatedAt  time.Time
}
