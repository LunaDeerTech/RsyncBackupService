package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

const (
	BackupTypeCold    = "cold"
	BackupTypeRolling = "rolling"
)

var coldVolumeSizePattern = regexp.MustCompile(`^[1-9][0-9]*[KMGTP]$`)

type CreateStrategyRequest struct {
	Name                string  `json:"name"`
	BackupType          string  `json:"backup_type"`
	CronExpr            *string `json:"cron_expr"`
	IntervalSeconds     int     `json:"interval_seconds"`
	RetentionDays       int     `json:"retention_days"`
	RetentionCount      int     `json:"retention_count"`
	ColdVolumeSize      *string `json:"cold_volume_size"`
	MaxExecutionSeconds int     `json:"max_execution_seconds"`
	StorageTargetIDs    []uint  `json:"storage_target_ids"`
	Enabled             bool    `json:"enabled"`
}

type UpdateStrategyRequest = CreateStrategyRequest

type StrategyService struct {
	db                *gorm.DB
	instanceRepo      repository.InstanceRepository
	strategyRepo      repository.StrategyRepository
	storageTargetRepo repository.StorageTargetRepository
	permissionService *PermissionService
}

func NewStrategyService(db *gorm.DB) *StrategyService {
	return &StrategyService{
		db:                db,
		instanceRepo:      repository.NewInstanceRepository(db),
		strategyRepo:      repository.NewStrategyRepository(db),
		storageTargetRepo: repository.NewStorageTargetRepository(db),
		permissionService: NewPermissionService(db),
	}
}

func (s *StrategyService) ValidateCreate(req CreateStrategyRequest) error {
	trimmedName := strings.TrimSpace(req.Name)
	if trimmedName == "" {
		return ErrNameRequired
	}

	trimmedBackupType := strings.TrimSpace(req.BackupType)
	if trimmedBackupType != BackupTypeRolling && trimmedBackupType != BackupTypeCold {
		return ErrInvalidBackupType
	}

	trimmedCronExpr := normalizeOptionalString(req.CronExpr)
	if trimmedCronExpr != nil && req.IntervalSeconds > 0 {
		return fmt.Errorf("%w: cron_expr and interval_seconds are mutually exclusive", ErrInvalidSchedule)
	}
	if trimmedCronExpr == nil && req.IntervalSeconds == 0 {
		return fmt.Errorf("%w: either cron_expr or interval_seconds is required", ErrScheduleRequired)
	}
	if req.IntervalSeconds < 0 {
		return fmt.Errorf("%w: interval_seconds must be >= 0", ErrInvalidSchedule)
	}
	if trimmedCronExpr != nil {
		if _, err := cron.ParseStandard(*trimmedCronExpr); err != nil {
			return fmt.Errorf("%w: cron_expr is invalid", ErrInvalidSchedule)
		}
	}
	if req.RetentionDays < 0 || req.RetentionCount < 0 {
		return fmt.Errorf("%w: retention values must be >= 0", ErrInvalidRetention)
	}
	if req.MaxExecutionSeconds < 0 {
		return fmt.Errorf("%w: max_execution_seconds must be >= 0", ErrInvalidMaxExecution)
	}
	if len(req.StorageTargetIDs) == 0 {
		return ErrStorageTargetsRequired
	}

	trimmedColdVolumeSize := normalizeOptionalString(req.ColdVolumeSize)
	if trimmedBackupType != BackupTypeCold && trimmedColdVolumeSize != nil {
		return fmt.Errorf("%w: cold_volume_size is only supported for cold backups", ErrInvalidColdVolumeSize)
	}
	if trimmedColdVolumeSize != nil && !coldVolumeSizePattern.MatchString(strings.ToUpper(*trimmedColdVolumeSize)) {
		return fmt.Errorf("%w: cold_volume_size must use formats like 500M or 1G", ErrInvalidColdVolumeSize)
	}

	return nil
}

func (s *StrategyService) ListByInstance(ctx context.Context, actor AuthIdentity, instanceID uint) ([]model.Strategy, error) {
	if _, err := s.findInstance(ctx, instanceID); err != nil {
		return nil, err
	}
	if err := s.requireInstanceRole(ctx, actor, instanceID, RoleViewer); err != nil {
		return nil, err
	}

	return s.strategyRepo.ListByInstanceID(ctx, instanceID)
}

func (s *StrategyService) Create(ctx context.Context, actor AuthIdentity, instanceID uint, req CreateStrategyRequest) (model.Strategy, error) {
	if _, err := s.findInstance(ctx, instanceID); err != nil {
		return model.Strategy{}, err
	}
	if err := s.requireInstanceRole(ctx, actor, instanceID, RoleAdmin); err != nil {
		return model.Strategy{}, err
	}
	if err := s.ValidateCreate(req); err != nil {
		return model.Strategy{}, err
	}

	storageTargets, err := s.loadAndValidateStorageTargets(ctx, req.StorageTargetIDs, req.BackupType)
	if err != nil {
		return model.Strategy{}, err
	}

	strategy := model.Strategy{
		InstanceID:          instanceID,
		Name:                strings.TrimSpace(req.Name),
		BackupType:          strings.TrimSpace(req.BackupType),
		CronExpr:            normalizeOptionalString(req.CronExpr),
		IntervalSeconds:     req.IntervalSeconds,
		RetentionDays:       req.RetentionDays,
		RetentionCount:      req.RetentionCount,
		ColdVolumeSize:      normalizeOptionalString(req.ColdVolumeSize),
		MaxExecutionSeconds: req.MaxExecutionSeconds,
		Enabled:             req.Enabled,
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&strategy).Error; err != nil {
			return fmt.Errorf("create strategy: %w", err)
		}
		if err := tx.Model(&strategy).Association("StorageTargets").Replace(storageTargets); err != nil {
			return fmt.Errorf("bind strategy storage targets: %w", err)
		}
		return nil
	}); err != nil {
		return model.Strategy{}, err
	}

	return s.findStrategy(ctx, strategy.ID)
}

func (s *StrategyService) Update(ctx context.Context, actor AuthIdentity, id uint, req UpdateStrategyRequest) (model.Strategy, error) {
	strategy, err := s.findStrategy(ctx, id)
	if err != nil {
		return model.Strategy{}, err
	}
	if err := s.requireInstanceRole(ctx, actor, strategy.InstanceID, RoleAdmin); err != nil {
		return model.Strategy{}, err
	}
	if err := s.ValidateCreate(req); err != nil {
		return model.Strategy{}, err
	}

	storageTargets, err := s.loadAndValidateStorageTargets(ctx, req.StorageTargetIDs, req.BackupType)
	if err != nil {
		return model.Strategy{}, err
	}

	strategy.Name = strings.TrimSpace(req.Name)
	strategy.BackupType = strings.TrimSpace(req.BackupType)
	strategy.CronExpr = normalizeOptionalString(req.CronExpr)
	strategy.IntervalSeconds = req.IntervalSeconds
	strategy.RetentionDays = req.RetentionDays
	strategy.RetentionCount = req.RetentionCount
	strategy.ColdVolumeSize = normalizeOptionalString(req.ColdVolumeSize)
	strategy.MaxExecutionSeconds = req.MaxExecutionSeconds
	strategy.Enabled = req.Enabled

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&strategy).Error; err != nil {
			return fmt.Errorf("update strategy: %w", err)
		}
		if err := tx.Model(&strategy).Association("StorageTargets").Replace(storageTargets); err != nil {
			return fmt.Errorf("replace strategy storage targets: %w", err)
		}
		return nil
	}); err != nil {
		return model.Strategy{}, err
	}

	return s.findStrategy(ctx, strategy.ID)
}

func (s *StrategyService) Delete(ctx context.Context, actor AuthIdentity, id uint) error {
	strategy, err := s.findStrategy(ctx, id)
	if err != nil {
		return err
	}
	if err := s.requireInstanceRole(ctx, actor, strategy.InstanceID, RoleAdmin); err != nil {
		return err
	}

	var backupRecordCount int64
	if err := s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("strategy_id = ?", id).Count(&backupRecordCount).Error; err != nil {
		return fmt.Errorf("count backup records by strategy: %w", err)
	}
	if backupRecordCount > 0 {
		return ErrResourceInUse
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("strategy_id = ?", id).Delete(&model.StrategyStorageBinding{}).Error; err != nil {
			return fmt.Errorf("delete strategy storage bindings: %w", err)
		}
		if err := tx.Delete(&model.Strategy{}, id).Error; err != nil {
			return fmt.Errorf("delete strategy: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *StrategyService) findInstance(ctx context.Context, id uint) (model.BackupInstance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, ErrInstanceNotFound
		}
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *StrategyService) findStrategy(ctx context.Context, id uint) (model.Strategy, error) {
	strategy, err := s.strategyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Strategy{}, ErrStrategyNotFound
		}
		return model.Strategy{}, err
	}

	return strategy, nil
}

func (s *StrategyService) loadAndValidateStorageTargets(ctx context.Context, ids []uint, backupType string) ([]model.StorageTarget, error) {
	uniqueIDs := uniqueUintIDs(ids)
	storageTargets, err := s.storageTargetRepo.ListByIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}
	if len(storageTargets) != len(uniqueIDs) {
		return nil, ErrStorageTargetNotFound
	}

	for _, storageTarget := range storageTargets {
		if !storageTargetMatchesBackupType(storageTarget.Type, backupType) {
			return nil, fmt.Errorf("%w: storage target %d is incompatible with %s backups", ErrInvalidStorageTargetType, storageTarget.ID, backupType)
		}
	}

	return storageTargets, nil
}

func (s *StrategyService) requireInstanceRole(ctx context.Context, actor AuthIdentity, instanceID uint, role string) error {
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

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmedValue := strings.TrimSpace(*value)
	if trimmedValue == "" {
		return nil
	}

	return &trimmedValue
}

func storageTargetMatchesBackupType(storageTargetType, backupType string) bool {
	switch strings.TrimSpace(backupType) {
	case BackupTypeRolling:
		return strings.HasPrefix(strings.TrimSpace(storageTargetType), "rolling_")
	case BackupTypeCold:
		return strings.HasPrefix(strings.TrimSpace(storageTargetType), "cold_")
	default:
		return false
	}
}

func uniqueUintIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	uniqueIDs := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	sort.Slice(uniqueIDs, func(left, right int) bool {
		return uniqueIDs[left] < uniqueIDs[right]
	})

	return uniqueIDs
}