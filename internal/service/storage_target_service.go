package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
	"gorm.io/gorm"
)

const (
	StorageTargetTypeColdLocal    = "cold_local"
	StorageTargetTypeColdSSH      = "cold_ssh"
	StorageTargetTypeRollingLocal = "rolling_local"
	StorageTargetTypeRollingSSH   = "rolling_ssh"
)

type CreateStorageTargetRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	SSHKeyID *uint  `json:"ssh_key_id"`
	BasePath string `json:"base_path"`
}

type UpdateStorageTargetRequest = CreateStorageTargetRequest

type StorageTargetService struct {
	db                *gorm.DB
	storageTargetRepo repository.StorageTargetRepository
	sshKeyRepo        repository.SSHKeyRepository
	schedulerService  StrategyScheduleRefresher
}


func NewStorageTargetService(db *gorm.DB, schedulerServices ...StrategyScheduleRefresher) *StorageTargetService {
	var schedulerService StrategyScheduleRefresher
	if len(schedulerServices) > 0 {
		schedulerService = schedulerServices[0]
	}

	return &StorageTargetService{
		db:                db,
		storageTargetRepo: repository.NewStorageTargetRepository(db),
		sshKeyRepo:        repository.NewSSHKeyRepository(db),
		schedulerService:  schedulerService,
	}
}

func (s *StorageTargetService) List(ctx context.Context) ([]model.StorageTarget, error) {
	return s.storageTargetRepo.List(ctx)
}

func (s *StorageTargetService) Create(ctx context.Context, req CreateStorageTargetRequest) (model.StorageTarget, error) {
	storageTarget, err := s.buildStorageTargetModel(ctx, req)
	if err != nil {
		return model.StorageTarget{}, err
	}
	if err := s.ensureUniqueTargetLocation(ctx, 0, storageTarget); err != nil {
		return model.StorageTarget{}, err
	}

	if err := s.storageTargetRepo.Create(ctx, &storageTarget); err != nil {
		return model.StorageTarget{}, err
	}

	return storageTarget, nil
}

func (s *StorageTargetService) Update(ctx context.Context, id uint, req UpdateStorageTargetRequest) (model.StorageTarget, error) {
	storageTarget, err := s.findStorageTarget(ctx, id)
	if err != nil {
		return model.StorageTarget{}, err
	}

	updatedStorageTarget, err := s.buildStorageTargetModel(ctx, req)
	if err != nil {
		return model.StorageTarget{}, err
	}
	if err := s.ensureUniqueTargetLocation(ctx, id, updatedStorageTarget); err != nil {
		return model.StorageTarget{}, err
	}
	if err := s.ensureBoundStrategiesCompatible(ctx, id, updatedStorageTarget.Type); err != nil {
		return model.StorageTarget{}, err
	}

	storageTarget.Name = updatedStorageTarget.Name
	storageTarget.Type = updatedStorageTarget.Type
	storageTarget.Host = updatedStorageTarget.Host
	storageTarget.Port = updatedStorageTarget.Port
	storageTarget.User = updatedStorageTarget.User
	storageTarget.SSHKeyID = updatedStorageTarget.SSHKeyID
	storageTarget.BasePath = updatedStorageTarget.BasePath

	if err := s.storageTargetRepo.Update(ctx, &storageTarget); err != nil {
		return model.StorageTarget{}, err
	}
	if err := s.refreshBoundStrategySchedules(ctx, storageTarget.ID); err != nil {
		return model.StorageTarget{}, err
	}

	return storageTarget, nil
}

func (s *StorageTargetService) Delete(ctx context.Context, id uint) error {
	if _, err := s.findStorageTarget(ctx, id); err != nil {
		return err
	}

	var bindingCount int64
	if err := s.db.WithContext(ctx).Model(&model.StrategyStorageBinding{}).Where("storage_target_id = ?", id).Count(&bindingCount).Error; err != nil {
		return fmt.Errorf("count strategy bindings by storage target: %w", err)
	}
	if bindingCount > 0 {
		return ErrResourceInUse
	}

	var backupRecordCount int64
	if err := s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("storage_target_id = ?", id).Count(&backupRecordCount).Error; err != nil {
		return fmt.Errorf("count backup records by storage target: %w", err)
	}
	if backupRecordCount > 0 {
		return ErrResourceInUse
	}

	return s.storageTargetRepo.Delete(ctx, id)
}

func (s *StorageTargetService) TestConnection(ctx context.Context, id uint) error {
	storageTarget, err := s.findStorageTarget(ctx, id)
	if err != nil {
		return err
	}

	backend, err := s.buildBackend(ctx, storageTarget)
	if err != nil {
		return err
	}

	if err := backend.TestConnection(ctx); err != nil {
		return normalizeSSHRuntimeError(err)
	}

	return nil
}

func (s *StorageTargetService) buildStorageTargetModel(ctx context.Context, req CreateStorageTargetRequest) (model.StorageTarget, error) {
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return model.StorageTarget{}, ErrNameRequired
	}

	trimmedType := strings.TrimSpace(req.Type)
	trimmedBasePath := strings.TrimSpace(req.BasePath)
	if trimmedBasePath == "" {
		return model.StorageTarget{}, ErrBasePathRequired
	}
	trimmedBasePath = filepath.Clean(trimmedBasePath)

	storageTarget := model.StorageTarget{
		Name:     trimmedName,
		Type:     trimmedType,
		BasePath: trimmedBasePath,
	}

	switch trimmedType {
	case StorageTargetTypeRollingLocal, StorageTargetTypeColdLocal:
		if strings.TrimSpace(req.Host) != "" || strings.TrimSpace(req.User) != "" || req.SSHKeyID != nil || req.Port != 0 {
			return model.StorageTarget{}, ErrUnexpectedSSHFields
		}
	case StorageTargetTypeRollingSSH, StorageTargetTypeColdSSH:
		trimmedHost := strings.ToLower(strings.TrimSpace(req.Host))
		trimmedUser := strings.TrimSpace(req.User)
		if trimmedHost == "" {
			return model.StorageTarget{}, ErrHostRequired
		}
		if trimmedUser == "" {
			return model.StorageTarget{}, ErrUserRequired
		}
		if req.SSHKeyID == nil {
			return model.StorageTarget{}, ErrSSHKeyRequired
		}
		if req.Port < 0 {
			return model.StorageTarget{}, ErrInvalidPort
		}
		if _, err := s.sshKeyRepo.GetByID(ctx, *req.SSHKeyID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.StorageTarget{}, ErrSSHKeyNotFound
			}
			return model.StorageTarget{}, err
		}

		storageTarget.Host = trimmedHost
		storageTarget.Port = req.Port
		if storageTarget.Port == 0 {
			storageTarget.Port = 22
		}
		storageTarget.User = trimmedUser
		storageTarget.SSHKeyID = req.SSHKeyID
	default:
		return model.StorageTarget{}, ErrInvalidStorageTargetType
	}

	return storageTarget, nil
}

func (s *StorageTargetService) ensureBoundStrategiesCompatible(ctx context.Context, storageTargetID uint, storageTargetType string) error {
	var strategies []model.Strategy
	if err := s.db.WithContext(ctx).
		Model(&model.Strategy{}).
		Joins("JOIN strategy_storage_bindings ON strategy_storage_bindings.strategy_id = strategies.id").
		Where("strategy_storage_bindings.storage_target_id = ?", storageTargetID).
		Find(&strategies).Error; err != nil {
		return fmt.Errorf("list bound strategies by storage target: %w", err)
	}

	for _, strategy := range strategies {
		if !storageTargetMatchesBackupType(storageTargetType, strategy.BackupType) {
			return ErrInvalidStorageTargetType
		}
	}

	return nil
}

func storageTargetLocationKey(storageTarget model.StorageTarget) string {
	basePath := filepath.Clean(strings.TrimSpace(storageTarget.BasePath))
	if isSSHStorageTargetType(storageTarget.Type) {
		port := storageTarget.Port
		if port == 0 {
			port = 22
		}
		return fmt.Sprintf("%s|%s|%d|%s|%s", strings.TrimSpace(storageTarget.Type), strings.ToLower(strings.TrimSpace(storageTarget.Host)), port, strings.TrimSpace(storageTarget.User), basePath)
	}

	return fmt.Sprintf("%s|%s", strings.TrimSpace(storageTarget.Type), basePath)
}

func (s *StorageTargetService) findStorageTarget(ctx context.Context, id uint) (model.StorageTarget, error) {
	storageTarget, err := s.storageTargetRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.StorageTarget{}, ErrStorageTargetNotFound
		}
		return model.StorageTarget{}, err
	}

	return storageTarget, nil
}

func (s *StorageTargetService) refreshBoundStrategySchedules(ctx context.Context, storageTargetID uint) error {
	if s.schedulerService == nil {
		return nil
	}

	var strategies []model.Strategy
	if err := s.db.WithContext(ctx).
		Model(&model.Strategy{}).
		Preload("StorageTargets").
		Joins("JOIN strategy_storage_bindings ON strategy_storage_bindings.strategy_id = strategies.id").
		Where("strategy_storage_bindings.storage_target_id = ?", storageTargetID).
		Find(&strategies).Error; err != nil {
		return fmt.Errorf("list bound strategies by storage target: %w", err)
	}

	for _, strategy := range strategies {
		if err := s.schedulerService.RefreshStrategy(strategy); err != nil {
			return fmt.Errorf("refresh strategy schedule: %w", err)
		}
	}

	return nil
}

func (s *StorageTargetService) ensureUniqueTargetLocation(ctx context.Context, currentStorageTargetID uint, storageTarget model.StorageTarget) error {
	query := s.db.WithContext(ctx).Model(&model.StorageTarget{}).Where("type = ?", storageTarget.Type)
	if currentStorageTargetID != 0 {
		query = query.Where("id <> ?", currentStorageTargetID)
	}

	var existingTargets []model.StorageTarget
	if err := query.Find(&existingTargets).Error; err != nil {
		return fmt.Errorf("list storage targets by location: %w", err)
	}
	desiredLocationKey := storageTargetLocationKey(storageTarget)
	for _, existingTarget := range existingTargets {
		if storageTargetLocationKey(existingTarget) == desiredLocationKey {
			return ErrDuplicateStorageTargetLocation
		}
	}

	return nil
}

func (s *StorageTargetService) buildBackend(ctx context.Context, storageTarget model.StorageTarget) (storage.StorageBackend, error) {
	if isSSHStorageTargetType(storageTarget.Type) {
		if storageTarget.SSHKeyID == nil {
			return nil, ErrSSHKeyRequired
		}

		sshKey, err := s.sshKeyRepo.GetByID(ctx, *storageTarget.SSHKeyID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrSSHKeyNotFound
			}
			return nil, err
		}

		return storage.NewSSHStorage(storage.SSHConfig{
			Host:           storageTarget.Host,
			Port:           storageTarget.Port,
			User:           storageTarget.User,
			PrivateKeyPath: sshKey.PrivateKeyPath,
			BasePath:       storageTarget.BasePath,
		}), nil
	}

	return storage.NewLocalStorage(storageTarget.BasePath), nil
}

func isSSHStorageTargetType(storageTargetType string) bool {
	switch strings.TrimSpace(storageTargetType) {
	case StorageTargetTypeRollingSSH, StorageTargetTypeColdSSH:
		return true
	default:
		return false
	}
}