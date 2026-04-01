package repository

import (
	"context"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type AuditLogRepository interface {
	Create(ctx context.Context, auditLog *model.AuditLog) error
}

type GormAuditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &GormAuditLogRepository{db: db}
}

func (r *GormAuditLogRepository) Create(ctx context.Context, auditLog *model.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(auditLog).Error; err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}

	return nil
}