package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type UserService struct {
	db          *gorm.DB
	authService *AuthService
}

func NewUserService(db *gorm.DB, authService *AuthService) *UserService {
	return &UserService{db: db, authService: authService}
}

func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	var users []model.User
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

func (s *UserService) Create(ctx context.Context, username, password string, isAdmin bool) (model.User, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return model.User{}, ErrUsernameRequired
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return model.User{}, err
	}

	var existingUser model.User
	if err := s.db.WithContext(ctx).Where("username = ?", trimmedUsername).First(&existingUser).Error; err == nil {
		return model.User{}, ErrUserExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, fmt.Errorf("check existing user: %w", err)
	}

	user := model.User{
		Username:     trimmedUsername,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *UserService) ResetPassword(ctx context.Context, userID uint, newPassword string) error {
	if s.authService != nil {
		return s.authService.ResetPassword(ctx, userID, newPassword)
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_hash", passwordHash)
	if result.Error != nil {
		return fmt.Errorf("reset password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}