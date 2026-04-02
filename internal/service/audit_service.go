package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

const (
	defaultAuditPageSize = 20
	maxAuditPageSize     = 100
)

type ListAuditLogsRequest struct {
	UserID       *uint
	Action       string
	ResourceType string
	StartTime    *time.Time
	EndTime      *time.Time
	Page         int
	PageSize     int
}

type AuditService struct {
	db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) List(ctx context.Context, req ListAuditLogsRequest) ([]model.AuditLog, int64, error) {
	page, pageSize, err := normalizeAuditPagination(req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	if req.StartTime != nil && req.EndTime != nil && req.StartTime.After(*req.EndTime) {
		return nil, 0, fmt.Errorf("%w: start_time must be before or equal to end_time", ErrAuditLogQueryInvalid)
	}

	query := s.db.WithContext(ctx).Model(&model.AuditLog{}).Preload("User")
	if req.UserID != nil {
		query = query.Where("user_id = ?", *req.UserID)
	}
	if trimmedAction := strings.TrimSpace(req.Action); trimmedAction != "" {
		query = query.Where("action = ?", trimmedAction)
	}
	if trimmedResourceType := strings.TrimSpace(req.ResourceType); trimmedResourceType != "" {
		query = query.Where("resource_type = ?", trimmedResourceType)
	}
	if req.StartTime != nil {
		query = query.Where("created_at >= ?", req.StartTime.UTC())
	}
	if req.EndTime != nil {
		query = query.Where("created_at <= ?", req.EndTime.UTC())
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	var logs []model.AuditLog
	if err := query.Order("created_at DESC").Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}

	return logs, total, nil
}

func normalizeAuditPagination(page, pageSize int) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = defaultAuditPageSize
	}
	if page < 1 {
		return 0, 0, fmt.Errorf("%w: page must be greater than 0", ErrAuditLogQueryInvalid)
	}
	if pageSize < 1 || pageSize > maxAuditPageSize {
		return 0, 0, fmt.Errorf("%w: page_size must be between 1 and %d", ErrAuditLogQueryInvalid, maxAuditPageSize)
	}

	return page, pageSize, nil
}