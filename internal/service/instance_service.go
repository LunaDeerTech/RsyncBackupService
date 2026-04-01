package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"gorm.io/gorm"
)

const (
	SourceTypeLocal  = "local"
	SourceTypeRemote = "remote"
)

type CreateInstanceRequest struct {
	Name            string   `json:"name"`
	SourceType      string   `json:"source_type"`
	SourceHost      string   `json:"source_host"`
	SourcePort      int      `json:"source_port"`
	SourceUser      string   `json:"source_user"`
	SourceSSHKeyID  *uint    `json:"source_ssh_key_id"`
	SourcePath      string   `json:"source_path"`
	ExcludePatterns []string `json:"exclude_patterns"`
	Enabled         bool     `json:"enabled"`
}

type UpdateInstanceRequest = CreateInstanceRequest

type InstanceService struct {
	db                *gorm.DB
	instanceRepo      repository.InstanceRepository
	sshKeyRepo        repository.SSHKeyRepository
	permissionService *PermissionService
}

func NewInstanceService(db *gorm.DB) *InstanceService {
	return &InstanceService{
		db:                db,
		instanceRepo:      repository.NewInstanceRepository(db),
		sshKeyRepo:        repository.NewSSHKeyRepository(db),
		permissionService: NewPermissionService(db),
	}
}

func (s *InstanceService) List(ctx context.Context, actor AuthIdentity) ([]model.BackupInstance, error) {
	if actor.IsAdmin {
		return s.instanceRepo.List(ctx)
	}

	return s.instanceRepo.ListByUser(ctx, actor.UserID)
}

func (s *InstanceService) Get(ctx context.Context, actor AuthIdentity, id uint) (model.BackupInstance, error) {
	instance, err := s.findInstance(ctx, id)
	if err != nil {
		return model.BackupInstance{}, err
	}

	if err := s.requireInstanceRole(ctx, actor, id, RoleViewer); err != nil {
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *InstanceService) Create(ctx context.Context, actor AuthIdentity, req CreateInstanceRequest) (model.BackupInstance, error) {
	instance, err := s.buildInstanceModel(ctx, req)
	if err != nil {
		return model.BackupInstance{}, err
	}
	instance.CreatedBy = actor.UserID

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&instance).Error; err != nil {
			return fmt.Errorf("create backup instance: %w", err)
		}

		permission := model.InstancePermission{
			UserID:     actor.UserID,
			InstanceID: instance.ID,
			Role:       RoleAdmin,
		}
		if err := tx.Create(&permission).Error; err != nil {
			return fmt.Errorf("create instance permission: %w", err)
		}

		return nil
	}); err != nil {
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *InstanceService) Update(ctx context.Context, actor AuthIdentity, id uint, req UpdateInstanceRequest) (model.BackupInstance, error) {
	instance, err := s.findInstance(ctx, id)
	if err != nil {
		return model.BackupInstance{}, err
	}
	if err := s.requireInstanceRole(ctx, actor, id, RoleAdmin); err != nil {
		return model.BackupInstance{}, err
	}

	updatedInstance, err := s.buildInstanceModel(ctx, req)
	if err != nil {
		return model.BackupInstance{}, err
	}

	instance.Name = updatedInstance.Name
	instance.SourceType = updatedInstance.SourceType
	instance.SourceHost = updatedInstance.SourceHost
	instance.SourcePort = updatedInstance.SourcePort
	instance.SourceUser = updatedInstance.SourceUser
	instance.SourceSSHKeyID = updatedInstance.SourceSSHKeyID
	instance.SourcePath = updatedInstance.SourcePath
	instance.ExcludePatterns = updatedInstance.ExcludePatterns
	instance.Enabled = updatedInstance.Enabled

	if err := s.instanceRepo.Update(ctx, &instance); err != nil {
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *InstanceService) Delete(ctx context.Context, actor AuthIdentity, id uint) error {
	if _, err := s.findInstance(ctx, id); err != nil {
		return err
	}
	if err := s.requireInstanceRole(ctx, actor, id, RoleAdmin); err != nil {
		return err
	}

	if err := s.ensureInstanceNotReferenced(ctx, id); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var strategyIDs []uint
		if err := tx.Model(&model.Strategy{}).Where("instance_id = ?", id).Pluck("id", &strategyIDs).Error; err != nil {
			return fmt.Errorf("list instance strategies: %w", err)
		}
		if len(strategyIDs) > 0 {
			if err := tx.Where("strategy_id IN ?", strategyIDs).Delete(&model.StrategyStorageBinding{}).Error; err != nil {
				return fmt.Errorf("delete instance strategy bindings: %w", err)
			}
		}

		if err := tx.Where("instance_id = ?", id).Delete(&model.Strategy{}).Error; err != nil {
			return fmt.Errorf("delete instance strategies: %w", err)
		}
		if err := tx.Where("instance_id = ?", id).Delete(&model.InstancePermission{}).Error; err != nil {
			return fmt.Errorf("delete instance permissions: %w", err)
		}
		if err := tx.Where("instance_id = ?", id).Delete(&model.NotificationSubscription{}).Error; err != nil {
			return fmt.Errorf("delete instance subscriptions: %w", err)
		}
		if err := tx.Delete(&model.BackupInstance{}, id).Error; err != nil {
			return fmt.Errorf("delete backup instance: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *InstanceService) buildInstanceModel(ctx context.Context, req CreateInstanceRequest) (model.BackupInstance, error) {
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return model.BackupInstance{}, ErrNameRequired
	}

	trimmedSourceType := strings.TrimSpace(req.SourceType)
	trimmedSourcePath := strings.TrimSpace(req.SourcePath)
	if trimmedSourcePath == "" {
		return model.BackupInstance{}, ErrSourcePathRequired
	}

	excludePatterns := req.ExcludePatterns
	if excludePatterns == nil {
		excludePatterns = []string{}
	}
	encodedExcludePatterns, err := json.Marshal(excludePatterns)
	if err != nil {
		return model.BackupInstance{}, fmt.Errorf("encode exclude patterns: %w", err)
	}

	instance := model.BackupInstance{
		Name:            trimmedName,
		SourceType:      trimmedSourceType,
		SourcePath:      trimmedSourcePath,
		ExcludePatterns: string(encodedExcludePatterns),
		Enabled:         req.Enabled,
	}

	switch trimmedSourceType {
	case SourceTypeLocal:
		if strings.TrimSpace(req.SourceHost) != "" || strings.TrimSpace(req.SourceUser) != "" || req.SourceSSHKeyID != nil || req.SourcePort != 0 {
			return model.BackupInstance{}, ErrUnexpectedSSHFields
		}
	case SourceTypeRemote:
		trimmedSourceHost := strings.TrimSpace(req.SourceHost)
		trimmedSourceUser := strings.TrimSpace(req.SourceUser)
		if trimmedSourceHost == "" {
			return model.BackupInstance{}, ErrHostRequired
		}
		if trimmedSourceUser == "" {
			return model.BackupInstance{}, ErrUserRequired
		}
		if req.SourceSSHKeyID == nil {
			return model.BackupInstance{}, ErrSSHKeyRequired
		}
		if req.SourcePort < 0 {
			return model.BackupInstance{}, ErrInvalidPort
		}
		if _, err := s.sshKeyRepo.GetByID(ctx, *req.SourceSSHKeyID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.BackupInstance{}, ErrSSHKeyNotFound
			}
			return model.BackupInstance{}, err
		}

		instance.SourceHost = trimmedSourceHost
		instance.SourcePort = req.SourcePort
		if instance.SourcePort == 0 {
			instance.SourcePort = 22
		}
		instance.SourceUser = trimmedSourceUser
		instance.SourceSSHKeyID = req.SourceSSHKeyID
	default:
		return model.BackupInstance{}, ErrInvalidSourceType
	}

	return instance, nil
}

func (s *InstanceService) ensureInstanceNotReferenced(ctx context.Context, instanceID uint) error {
	var backupRecordCount int64
	if err := s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("instance_id = ?", instanceID).Count(&backupRecordCount).Error; err != nil {
		return fmt.Errorf("count backup records by instance: %w", err)
	}
	if backupRecordCount > 0 {
		return ErrResourceInUse
	}

	var restoreRecordCount int64
	if err := s.db.WithContext(ctx).Model(&model.RestoreRecord{}).Where("instance_id = ?", instanceID).Count(&restoreRecordCount).Error; err != nil {
		return fmt.Errorf("count restore records by instance: %w", err)
	}
	if restoreRecordCount > 0 {
		return ErrResourceInUse
	}

	return nil
}

func (s *InstanceService) findInstance(ctx context.Context, id uint) (model.BackupInstance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, ErrInstanceNotFound
		}
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *InstanceService) requireInstanceRole(ctx context.Context, actor AuthIdentity, instanceID uint, role string) error {
	if actor.IsAdmin {
		return nil
	}

	allowed, err := s.permissionService.HasInstanceRole(ctx, actor, instanceID, role)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermissionDenied
	}

	return nil
}