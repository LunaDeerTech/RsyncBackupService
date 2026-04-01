package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type SSHKeyRepository interface {
	List(ctx context.Context) ([]model.SSHKey, error)
	GetByID(ctx context.Context, id uint) (model.SSHKey, error)
	Create(ctx context.Context, sshKey *model.SSHKey) error
	Delete(ctx context.Context, id uint) error
}

type GormSSHKeyRepository struct {
	db *gorm.DB
}

func NewSSHKeyRepository(db *gorm.DB) SSHKeyRepository {
	return &GormSSHKeyRepository{db: db}
}

func (r *GormSSHKeyRepository) List(ctx context.Context) ([]model.SSHKey, error) {
	var sshKeys []model.SSHKey
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&sshKeys).Error; err != nil {
		return nil, fmt.Errorf("list ssh keys: %w", err)
	}

	return sshKeys, nil
}

func (r *GormSSHKeyRepository) GetByID(ctx context.Context, id uint) (model.SSHKey, error) {
	var sshKey model.SSHKey
	if err := r.db.WithContext(ctx).First(&sshKey, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.SSHKey{}, err
		}
		return model.SSHKey{}, fmt.Errorf("get ssh key: %w", err)
	}

	return sshKey, nil
}

func (r *GormSSHKeyRepository) Create(ctx context.Context, sshKey *model.SSHKey) error {
	if err := r.db.WithContext(ctx).Create(sshKey).Error; err != nil {
		return fmt.Errorf("create ssh key: %w", err)
	}

	return nil
}

func (r *GormSSHKeyRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.SSHKey{}, id).Error; err != nil {
		return fmt.Errorf("delete ssh key: %w", err)
	}

	return nil
}