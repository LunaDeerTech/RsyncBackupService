package model

import "time"

type StorageTarget struct {
	ID            uint `gorm:"primaryKey"`
	Name          string
	Type          string
	Host          string
	Port          int
	User          string
	SSHKeyID      *uint
	SSHKey        *SSHKey
	BasePath      string
	Strategies    []Strategy     `gorm:"many2many:strategy_storage_bindings;"`
	BackupRecords []BackupRecord `gorm:"foreignKey:StorageTargetID"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
