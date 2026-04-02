package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
	"gorm.io/gorm"
)

type RetentionService struct {
	db          *gorm.DB
	instanceRepo repository.InstanceRepository
	sshKeyRepo  repository.SSHKeyRepository
	clock       func() time.Time
}

type retentionCandidate struct {
	path       string
	modifiedAt time.Time
}

func NewRetentionService(db *gorm.DB) *RetentionService {
	return &RetentionService{
		db:           db,
		instanceRepo: repository.NewInstanceRepository(db),
		sshKeyRepo:   repository.NewSSHKeyRepository(db),
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *RetentionService) Cleanup(ctx context.Context, strategy model.Strategy, target model.StorageTarget) error {
	if strategy.RetentionCount <= 0 && strategy.RetentionDays <= 0 {
		return nil
	}

	backend, err := buildStorageBackend(ctx, s.sshKeyRepo, target)
	if err != nil {
		return err
	}

	candidates, err := s.listRetentionCandidates(ctx, strategy, target)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return nil
	}

	deletePaths := s.selectDeletionPaths(candidates, strategy)
	for _, deletePath := range deletePaths {
		if err := backend.Delete(ctx, deletePath); err != nil {
			return fmt.Errorf("delete retained snapshot %q: %w", deletePath, normalizeSSHRuntimeError(err))
		}
	}

	return nil
}

func (s *RetentionService) listRetentionCandidates(ctx context.Context, strategy model.Strategy, target model.StorageTarget) ([]retentionCandidate, error) {
	instance, err := s.loadInstance(ctx, strategy)
	if err != nil {
		return nil, err
	}

	var records []model.BackupRecord
	query := s.db.WithContext(ctx).
		Model(&model.BackupRecord{}).
		Where("instance_id = ? AND backup_type = ? AND status = ? AND snapshot_path <> ''", strategy.InstanceID, BackupTypeRolling, model.BackupStatusSuccess).
		Order("finished_at DESC").
		Order("id DESC")
	if strategy.ID != 0 {
		query = query.Where("strategy_id = ?", strategy.ID)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list retained snapshots: %w", err)
	}

	candidates := make([]retentionCandidate, 0, len(records))
	for _, record := range records {
		snapshotPath := strings.TrimSpace(record.SnapshotPath)
		if snapshotPath == "" || filepath.Base(snapshotPath) == "latest" {
			continue
		}
		if !retentionRecordMatchesTarget(record, target, instance) {
			continue
		}
		relativePath, ok := relativeTargetPath(snapshotPath, target.BasePath)
		if !ok {
			continue
		}
		modifiedAt := record.StartedAt.UTC()
		if record.FinishedAt != nil {
			modifiedAt = record.FinishedAt.UTC()
		}
		candidates = append(candidates, retentionCandidate{path: relativePath, modifiedAt: modifiedAt})
	}

	return candidates, nil
}

func retentionRecordMatchesTarget(record model.BackupRecord, target model.StorageTarget, instance model.BackupInstance) bool {
	targetLocationKey := storageTargetLocationKey(target)
	if strings.TrimSpace(record.TargetLocationKey) != "" {
		return strings.TrimSpace(record.TargetLocationKey) == targetLocationKey
	}
	if isSSHStorageTargetType(target.Type) {
		return false
	}

	snapshotRoot := filepath.Join(target.BasePath, executorpkg.SnapshotRootDir(instance))
	return snapshotWithinTargetBase(strings.TrimSpace(record.SnapshotPath), snapshotRoot)
}

func relativeTargetPath(snapshotPath string, basePath string) (string, bool) {
	cleanSnapshotPath := filepath.Clean(strings.TrimSpace(snapshotPath))
	cleanBasePath := filepath.Clean(strings.TrimSpace(basePath))
	if !snapshotWithinTargetBase(cleanSnapshotPath, cleanBasePath) {
		return "", false
	}
	if cleanSnapshotPath == cleanBasePath {
		return ".", true
	}
	relativePath, err := filepath.Rel(cleanBasePath, cleanSnapshotPath)
	if err != nil {
		return "", false
	}
	if relativePath == "." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || relativePath == ".." {
		return "", false
	}

	return filepath.Clean(relativePath), true
}

func (s *RetentionService) loadInstance(ctx context.Context, strategy model.Strategy) (model.BackupInstance, error) {
	if strategy.Instance.ID != 0 {
		return strategy.Instance, nil
	}

	instance, err := s.instanceRepo.GetByID(ctx, strategy.InstanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, ErrInstanceNotFound
		}
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *RetentionService) selectDeletionPaths(candidates []retentionCandidate, strategy model.Strategy) []string {
	markedPaths := make(map[string]struct{})

	sortedCandidates := append([]retentionCandidate(nil), candidates...)
	sort.Slice(sortedCandidates, func(left, right int) bool {
		if sortedCandidates[left].modifiedAt.Equal(sortedCandidates[right].modifiedAt) {
			return sortedCandidates[left].path > sortedCandidates[right].path
		}
		return sortedCandidates[left].modifiedAt.After(sortedCandidates[right].modifiedAt)
	})

	if strategy.RetentionCount > 0 && len(sortedCandidates) > strategy.RetentionCount {
		for _, candidate := range sortedCandidates[strategy.RetentionCount:] {
			markedPaths[candidate.path] = struct{}{}
		}
	}

	if strategy.RetentionDays > 0 {
		threshold := s.clock().Add(-time.Duration(strategy.RetentionDays) * 24 * time.Hour)
		for _, candidate := range sortedCandidates {
			if candidate.modifiedAt.Before(threshold) {
				markedPaths[candidate.path] = struct{}{}
			}
		}
	}

	deletePaths := make([]string, 0, len(markedPaths))
	for deletePath := range markedPaths {
		deletePaths = append(deletePaths, deletePath)
	}
	sort.Strings(deletePaths)

	return deletePaths
}

func buildStorageBackend(ctx context.Context, sshKeyRepo repository.SSHKeyRepository, target model.StorageTarget) (storage.StorageBackend, error) {
	if isSSHStorageTargetType(target.Type) {
		if target.SSHKeyID == nil {
			return nil, ErrSSHKeyRequired
		}

		sshKey, err := sshKeyRepo.GetByID(ctx, *target.SSHKeyID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrSSHKeyNotFound
			}
			return nil, err
		}

		return storage.NewSSHStorage(storage.SSHConfig{
			Host:           target.Host,
			Port:           target.Port,
			User:           target.User,
			PrivateKeyPath: sshKey.PrivateKeyPath,
			BasePath:       strings.TrimSpace(target.BasePath),
		}), nil
	}

	return storage.NewLocalStorage(strings.TrimSpace(target.BasePath)), nil
}