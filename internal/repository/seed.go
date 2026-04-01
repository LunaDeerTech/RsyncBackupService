package repository

import (
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func EnsureAdminUser(db *gorm.DB, username, password string) error {
	var userCount int64
	if err := db.Model(&model.User{}).Count(&userCount).Error; err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if userCount > 0 {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	adminUser := model.User{
		Username:     username,
		PasswordHash: string(passwordHash),
		IsAdmin:      true,
	}
	if err := db.Create(&adminUser).Error; err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	return nil
}
