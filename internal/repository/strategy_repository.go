package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type StrategyRepository interface {
	ListByInstanceID(ctx context.Context, instanceID uint) ([]model.Strategy, error)
	GetByID(ctx context.Context, id uint) (model.Strategy, error)
	Create(ctx context.Context, strategy *model.Strategy) error
	Update(ctx context.Context, strategy *model.Strategy) error
	Delete(ctx context.Context, id uint) error
}

type GormStrategyRepository struct {
	db *gorm.DB
}

func NewStrategyRepository(db *gorm.DB) StrategyRepository {
	return &GormStrategyRepository{db: db}
}

func (r *GormStrategyRepository) ListByInstanceID(ctx context.Context, instanceID uint) ([]model.Strategy, error) {
	var strategies []model.Strategy
	if err := r.db.WithContext(ctx).
		Preload("StorageTargets").
		Where("instance_id = ?", instanceID).
		Order("id ASC").
		Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("list strategies by instance: %w", err)
	}

	return strategies, nil
}

func (r *GormStrategyRepository) GetByID(ctx context.Context, id uint) (model.Strategy, error) {
	var strategy model.Strategy
	if err := r.db.WithContext(ctx).Preload("StorageTargets").First(&strategy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Strategy{}, err
		}
		return model.Strategy{}, fmt.Errorf("get strategy: %w", err)
	}

	return strategy, nil
}

func (r *GormStrategyRepository) Create(ctx context.Context, strategy *model.Strategy) error {
	if err := r.db.WithContext(ctx).Create(strategy).Error; err != nil {
		return fmt.Errorf("create strategy: %w", err)
	}

	return nil
}

func (r *GormStrategyRepository) Update(ctx context.Context, strategy *model.Strategy) error {
	if err := r.db.WithContext(ctx).Save(strategy).Error; err != nil {
		return fmt.Errorf("update strategy: %w", err)
	}

	return nil
}

func (r *GormStrategyRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.Strategy{}, id).Error; err != nil {
		return fmt.Errorf("delete strategy: %w", err)
	}

	return nil
}