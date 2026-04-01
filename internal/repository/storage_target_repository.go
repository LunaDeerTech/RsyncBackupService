package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type StorageTargetRepository interface {
	List(ctx context.Context) ([]model.StorageTarget, error)
	ListByIDs(ctx context.Context, ids []uint) ([]model.StorageTarget, error)
	GetByID(ctx context.Context, id uint) (model.StorageTarget, error)
	Create(ctx context.Context, storageTarget *model.StorageTarget) error
	Update(ctx context.Context, storageTarget *model.StorageTarget) error
	Delete(ctx context.Context, id uint) error
}

type GormStorageTargetRepository struct {
	db *gorm.DB
}

func NewStorageTargetRepository(db *gorm.DB) StorageTargetRepository {
	return &GormStorageTargetRepository{db: db}
}

func (r *GormStorageTargetRepository) List(ctx context.Context) ([]model.StorageTarget, error) {
	var storageTargets []model.StorageTarget
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&storageTargets).Error; err != nil {
		return nil, fmt.Errorf("list storage targets: %w", err)
	}

	return storageTargets, nil
}

func (r *GormStorageTargetRepository) ListByIDs(ctx context.Context, ids []uint) ([]model.StorageTarget, error) {
	var storageTargets []model.StorageTarget
	if len(ids) == 0 {
		return storageTargets, nil
	}

	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("id ASC").Find(&storageTargets).Error; err != nil {
		return nil, fmt.Errorf("list storage targets by ids: %w", err)
	}

	return storageTargets, nil
}

func (r *GormStorageTargetRepository) GetByID(ctx context.Context, id uint) (model.StorageTarget, error) {
	var storageTarget model.StorageTarget
	if err := r.db.WithContext(ctx).First(&storageTarget, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.StorageTarget{}, err
		}
		return model.StorageTarget{}, fmt.Errorf("get storage target: %w", err)
	}

	return storageTarget, nil
}

func (r *GormStorageTargetRepository) Create(ctx context.Context, storageTarget *model.StorageTarget) error {
	if err := r.db.WithContext(ctx).Create(storageTarget).Error; err != nil {
		return fmt.Errorf("create storage target: %w", err)
	}

	return nil
}

func (r *GormStorageTargetRepository) Update(ctx context.Context, storageTarget *model.StorageTarget) error {
	if err := r.db.WithContext(ctx).Save(storageTarget).Error; err != nil {
		return fmt.Errorf("update storage target: %w", err)
	}

	return nil
}

func (r *GormStorageTargetRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.StorageTarget{}, id).Error; err != nil {
		return fmt.Errorf("delete storage target: %w", err)
	}

	return nil
}