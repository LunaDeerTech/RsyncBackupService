package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type InstanceRepository interface {
	List(ctx context.Context) ([]model.BackupInstance, error)
	ListByUser(ctx context.Context, userID uint) ([]model.BackupInstance, error)
	GetByID(ctx context.Context, id uint) (model.BackupInstance, error)
	Create(ctx context.Context, instance *model.BackupInstance) error
	Update(ctx context.Context, instance *model.BackupInstance) error
	Delete(ctx context.Context, id uint) error
}

type GormInstanceRepository struct {
	db *gorm.DB
}

func NewInstanceRepository(db *gorm.DB) InstanceRepository {
	return &GormInstanceRepository{db: db}
}

func (r *GormInstanceRepository) List(ctx context.Context) ([]model.BackupInstance, error) {
	var instances []model.BackupInstance
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("list backup instances: %w", err)
	}

	return instances, nil
}

func (r *GormInstanceRepository) ListByUser(ctx context.Context, userID uint) ([]model.BackupInstance, error) {
	var instances []model.BackupInstance
	if err := r.db.WithContext(ctx).
		Model(&model.BackupInstance{}).
		Joins("JOIN instance_permissions ON instance_permissions.instance_id = backup_instances.id").
		Where("instance_permissions.user_id = ?", userID).
		Distinct("backup_instances.*").
		Order("backup_instances.id ASC").
		Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("list backup instances by user: %w", err)
	}

	return instances, nil
}

func (r *GormInstanceRepository) GetByID(ctx context.Context, id uint) (model.BackupInstance, error) {
	var instance model.BackupInstance
	if err := r.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, err
		}
		return model.BackupInstance{}, fmt.Errorf("get backup instance: %w", err)
	}

	return instance, nil
}

func (r *GormInstanceRepository) Create(ctx context.Context, instance *model.BackupInstance) error {
	if err := r.db.WithContext(ctx).Create(instance).Error; err != nil {
		return fmt.Errorf("create backup instance: %w", err)
	}

	return nil
}

func (r *GormInstanceRepository) Update(ctx context.Context, instance *model.BackupInstance) error {
	if err := r.db.WithContext(ctx).Save(instance).Error; err != nil {
		return fmt.Errorf("update backup instance: %w", err)
	}

	return nil
}

func (r *GormInstanceRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.BackupInstance{}, id).Error; err != nil {
		return fmt.Errorf("delete backup instance: %w", err)
	}

	return nil
}