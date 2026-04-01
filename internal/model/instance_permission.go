package model

import "time"

type InstancePermission struct {
	ID         uint `gorm:"primaryKey"`
	UserID     uint
	User       User
	InstanceID uint
	Instance   BackupInstance
	Role       string
	CreatedAt  time.Time
}
