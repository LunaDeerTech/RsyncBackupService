package model

import "time"

type SSHKey struct {
	ID              uint `gorm:"primaryKey"`
	Name            string
	PrivateKeyPath  string
	Fingerprint     string
	BackupInstances []BackupInstance `gorm:"foreignKey:SourceSSHKeyID"`
	StorageTargets  []StorageTarget  `gorm:"foreignKey:SSHKeyID"`
	CreatedAt       time.Time
}
