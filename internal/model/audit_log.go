package model

import "time"

type AuditLog struct {
	ID           uint `gorm:"primaryKey"`
	UserID       uint
	User         User
	Action       string
	ResourceType string
	ResourceID   uint
	Detail       string
	IPAddress    string
	CreatedAt    time.Time
}
