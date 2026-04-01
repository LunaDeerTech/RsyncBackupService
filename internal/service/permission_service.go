package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm/clause"
	"gorm.io/gorm"
)

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
)

var roleOrder = map[string]int{
	RoleViewer: 1,
	RoleAdmin:  2,
}

type PermissionService struct {
	db *gorm.DB
}

type InstancePermissionView struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	InstanceID uint   `json:"instance_id"`
	Role       string `json:"role"`
}

func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

func (s *PermissionService) HasInstanceRole(ctx context.Context, user AuthIdentity, instanceID uint, minRole string) (bool, error) {
	if user.IsAdmin {
		return true, nil
	}
	if _, ok := roleOrder[minRole]; !ok {
		return false, ErrInvalidRole
	}

	var permission model.InstancePermission
	if err := s.db.WithContext(ctx).Where("user_id = ? AND instance_id = ?", user.UserID, instanceID).Order("id DESC").First(&permission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("find instance permission: %w", err)
	}

	permissionRank, ok := roleOrder[permission.Role]
	if !ok {
		return false, ErrInvalidRole
	}

	return permissionRank >= roleOrder[minRole], nil
}

func (s *PermissionService) ListInstancePermissions(ctx context.Context, instanceID uint) ([]InstancePermissionView, error) {
	if err := s.ensureInstanceExists(ctx, instanceID); err != nil {
		return nil, err
	}

	var permissions []model.InstancePermission
	if err := s.db.WithContext(ctx).Preload("User").Where("instance_id = ?", instanceID).Order("user_id ASC").Find(&permissions).Error; err != nil {
		return nil, fmt.Errorf("list instance permissions: %w", err)
	}

	views := make([]InstancePermissionView, 0, len(permissions))
	for _, permission := range permissions {
		views = append(views, InstancePermissionView{
			UserID:     permission.UserID,
			Username:   permission.User.Username,
			InstanceID: permission.InstanceID,
			Role:       permission.Role,
		})
	}

	return views, nil
}

func (s *PermissionService) SetInstanceRole(ctx context.Context, instanceID, userID uint, role string) (InstancePermissionView, error) {
	trimmedRole := strings.TrimSpace(role)
	if _, ok := roleOrder[trimmedRole]; !ok {
		return InstancePermissionView{}, ErrInvalidRole
	}
	if err := s.ensureInstanceExists(ctx, instanceID); err != nil {
		return InstancePermissionView{}, err
	}
	user, err := s.ensureUserExists(ctx, userID)
	if err != nil {
		return InstancePermissionView{}, err
	}

	permission := model.InstancePermission{
		UserID:     userID,
		InstanceID: instanceID,
		Role:       trimmedRole,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "instance_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"role"}),
	}).Create(&permission).Error; err != nil {
		return InstancePermissionView{}, fmt.Errorf("upsert instance permission: %w", err)
	}

	return InstancePermissionView{
		UserID:     permission.UserID,
		Username:   user.Username,
		InstanceID: permission.InstanceID,
		Role:       permission.Role,
	}, nil
}

func (s *PermissionService) ensureInstanceExists(ctx context.Context, instanceID uint) error {
	var instance model.BackupInstance
	if err := s.db.WithContext(ctx).First(&instance, instanceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInstanceNotFound
		}
		return fmt.Errorf("find instance: %w", err)
	}

	return nil
}

func (s *PermissionService) ensureUserExists(ctx context.Context, userID uint) (model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("find user: %w", err)
	}

	return user, nil
}