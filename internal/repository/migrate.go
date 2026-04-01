package repository

import (
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

func MigrateAndSeed(db *gorm.DB, cfg config.Config) error {
	if err := db.AutoMigrate(allModels()...); err != nil {
		return fmt.Errorf("auto migrate database: %w", err)
	}

	if err := EnsureAdminUser(db, cfg.AdminUser, cfg.AdminPassword); err != nil {
		return fmt.Errorf("ensure admin user: %w", err)
	}

	return nil
}

func allModels() []any {
	return []any{
		&model.User{},
		&model.SSHKey{},
		&model.BackupInstance{},
		&model.StorageTarget{},
		&model.Strategy{},
		&model.StrategyStorageBinding{},
		&model.BackupRecord{},
		&model.RestoreRecord{},
		&model.NotificationChannel{},
		&model.NotificationSubscription{},
		&model.InstancePermission{},
		&model.AuditLog{},
	}
}
