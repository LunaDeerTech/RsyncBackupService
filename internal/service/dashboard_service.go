package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"gorm.io/gorm"
)

type SystemStatus struct {
	Version        string `json:"version"`
	DataDir        string `json:"data_dir"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
	DiskTotalBytes uint64 `json:"disk_total_bytes"`
	DiskFreeBytes  uint64 `json:"disk_free_bytes"`
}

type DashboardSummary struct {
	InstanceCount    int64                     `json:"instance_count"`
	TodayBackupCount int64                     `json:"today_backup_count"`
	SuccessCount     int64                     `json:"success_count"`
	FailedCount      int64                     `json:"failed_count"`
	RunningTasks     []RunningTaskStatus       `json:"running_tasks"`
	RecentBackups    []DashboardBackupSummary  `json:"recent_backups"`
	StorageOverview  []DashboardStorageSummary `json:"storage_overview"`
}

type DashboardBackupSummary struct {
	ID              uint    `json:"id"`
	InstanceID      uint    `json:"instance_id"`
	InstanceName    string  `json:"instance_name"`
	StorageTargetID uint    `json:"storage_target_id"`
	BackupType      string  `json:"backup_type"`
	Status          string  `json:"status"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      *string `json:"finished_at,omitempty"`
}

type DashboardStorageSummary struct {
	StorageTargetID   uint    `json:"storage_target_id"`
	StorageTargetName string  `json:"storage_target_name"`
	StorageTargetType string  `json:"storage_target_type"`
	AvailableBytes    uint64  `json:"available_bytes"`
	BackupCount       int64   `json:"backup_count"`
	LastBackupAt      *string `json:"last_backup_at,omitempty"`
}

type DashboardService struct {
	db              *gorm.DB
	config          config.Config
	executorService *ExecutorService
	sshKeyRepo      repository.SSHKeyRepository
	startedAt       time.Time
	clock           func() time.Time
	version         string
}

func NewDashboardService(db *gorm.DB, cfg config.Config, executorService *ExecutorService) *DashboardService {
	return &DashboardService{
		db:              db,
		config:          cfg,
		executorService: executorService,
		sshKeyRepo:      repository.NewSSHKeyRepository(db),
		startedAt:       time.Now().UTC(),
		clock: func() time.Time {
			return time.Now().UTC()
		},
		version: resolveBuildVersion(),
	}
}

func (s *DashboardService) GetSystemStatus(ctx context.Context) (SystemStatus, error) {
	if s == nil || s.db == nil {
		return SystemStatus{}, fmt.Errorf("dashboard service unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	dataDir := strings.TrimSpace(s.config.DataDir)
	if dataDir == "" {
		dataDir = "."
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return SystemStatus{}, fmt.Errorf("statfs data dir: %w", err)
	}

	uptimeSeconds := int64(s.clock().Sub(s.startedAt).Seconds())
	if uptimeSeconds < 0 {
		uptimeSeconds = 0
	}

	return SystemStatus{
		Version:        s.version,
		DataDir:        dataDir,
		UptimeSeconds:  uptimeSeconds,
		DiskTotalBytes: stat.Blocks * uint64(stat.Bsize),
		DiskFreeBytes:  stat.Bavail * uint64(stat.Bsize),
	}, nil
}

func (s *DashboardService) GetDashboard(ctx context.Context) (DashboardSummary, error) {
	if s == nil || s.db == nil {
		return DashboardSummary{}, fmt.Errorf("dashboard service unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := s.clock().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var instanceCount int64
	if err := s.db.WithContext(ctx).Model(&model.BackupInstance{}).Count(&instanceCount).Error; err != nil {
		return DashboardSummary{}, fmt.Errorf("count instances: %w", err)
	}

	todayBackupCount, err := s.countBackupsByStatus(ctx, dayStart, "")
	if err != nil {
		return DashboardSummary{}, err
	}
	successCount, err := s.countBackupsByStatus(ctx, dayStart, model.BackupStatusSuccess)
	if err != nil {
		return DashboardSummary{}, err
	}
	failedCount, err := s.countBackupsByStatus(ctx, dayStart, model.BackupStatusFailed)
	if err != nil {
		return DashboardSummary{}, err
	}
	recentBackups, err := s.listRecentBackups(ctx)
	if err != nil {
		return DashboardSummary{}, err
	}
	storageOverview, err := s.listStorageOverview(ctx)
	if err != nil {
		return DashboardSummary{}, err
	}

	var runningTasks []RunningTaskStatus
	if s.executorService != nil {
		runningTasks = s.executorService.ListRunningTasks()
	}

	return DashboardSummary{
		InstanceCount:    instanceCount,
		TodayBackupCount: todayBackupCount,
		SuccessCount:     successCount,
		FailedCount:      failedCount,
		RunningTasks:     runningTasks,
		RecentBackups:    recentBackups,
		StorageOverview:  storageOverview,
	}, nil
}

func (s *DashboardService) countBackupsByStatus(ctx context.Context, dayStart time.Time, status string) (int64, error) {
	query := s.db.WithContext(ctx).Model(&model.BackupRecord{}).Where("started_at >= ?", dayStart)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count backups: %w", err)
	}

	return count, nil
}

func (s *DashboardService) listRecentBackups(ctx context.Context) ([]DashboardBackupSummary, error) {
	var records []model.BackupRecord
	if err := s.db.WithContext(ctx).
		Preload("Instance").
		Order("started_at DESC").
		Order("id DESC").
		Limit(10).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list recent backups: %w", err)
	}

	items := make([]DashboardBackupSummary, 0, len(records))
	for _, record := range records {
		items = append(items, DashboardBackupSummary{
			ID:              record.ID,
			InstanceID:      record.InstanceID,
			InstanceName:    record.Instance.Name,
			StorageTargetID: record.StorageTargetID,
			BackupType:      record.BackupType,
			Status:          record.Status,
			StartedAt:       record.StartedAt.UTC().Format(http.TimeFormat),
			FinishedAt:      formatOptionalDashboardTime(record.FinishedAt),
		})
	}

	return items, nil
}

func (s *DashboardService) listStorageOverview(ctx context.Context) ([]DashboardStorageSummary, error) {
	type storageStatsRow struct {
		StorageTargetID uint
		BackupCount     int64
		LastBackupAt    sql.NullString
	}

	var statsRows []storageStatsRow
	if err := s.db.WithContext(ctx).
		Model(&model.BackupRecord{}).
		Select("storage_target_id, COUNT(*) AS backup_count, MAX(started_at) AS last_backup_at").
		Group("storage_target_id").
		Order("backup_count DESC").
		Order("storage_target_id ASC").
		Scan(&statsRows).Error; err != nil {
		return nil, fmt.Errorf("list storage overview: %w", err)
	}

	statsByTargetID := make(map[uint]storageStatsRow, len(statsRows))
	for _, row := range statsRows {
		statsByTargetID[row.StorageTargetID] = row
	}

	var targets []model.StorageTarget
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&targets).Error; err != nil {
		return nil, fmt.Errorf("list storage targets for dashboard: %w", err)
	}

	items := make([]DashboardStorageSummary, 0, len(targets))
	for _, target := range targets {
		stats := statsByTargetID[target.ID]
		lastBackupAt, err := parseDashboardAggregateTime(stats.LastBackupAt)
		if err != nil {
			return nil, err
		}
		var availableBytes uint64
		backend, err := buildStorageBackend(ctx, s.sshKeyRepo, target)
		if err == nil && backend != nil {
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			availableBytes, err = backend.SpaceAvailable(probeCtx, ".")
			cancel()
			if err != nil {
				availableBytes = 0
			}
		}

		items = append(items, DashboardStorageSummary{
			StorageTargetID:   target.ID,
			StorageTargetName: target.Name,
			StorageTargetType: target.Type,
			AvailableBytes:    availableBytes,
			BackupCount:       stats.BackupCount,
			LastBackupAt:      formatOptionalDashboardTime(lastBackupAt),
		})
	}

	return items, nil
}

func parseDashboardAggregateTime(value sql.NullString) (*time.Time, error) {
	trimmed := strings.TrimSpace(value.String)
	if !value.Valid || trimmed == "" {
		return nil, nil
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
	} {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}

	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.ParseInLocation(layout, trimmed, time.UTC); err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("parse dashboard aggregate time %q: unsupported format", value.String)
}

func formatOptionalDashboardTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}

	formatted := value.UTC().Format(http.TimeFormat)
	return &formatted
}

func resolveBuildVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}

	version := strings.TrimSpace(buildInfo.Main.Version)
	if version == "" || version == "(devel)" {
		return "dev"
	}

	return version
}
