package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/storage"
	"gorm.io/gorm"
)

type ExecutorService struct {
	db               *gorm.DB
	config           config.Config
	instanceRepo     repository.InstanceRepository
	strategyRepo     repository.StrategyRepository
	storageTargetRepo repository.StorageTargetRepository
	sshKeyRepo       repository.SSHKeyRepository
	retentionService retentionCleaner
	taskManager      *executorpkg.TaskManager
	notificationDispatcher notificationDispatcher
	runner           executorpkg.Runner
	clock            func() time.Time
}

type retentionCleaner interface {
	Cleanup(ctx context.Context, strategy model.Strategy, target model.StorageTarget) error
}

type ListBackupRecordsRequest struct {
	InstanceID     uint
	StrategyID     *uint
	BackupType     string
	Status         string
	RestorableOnly bool
}

const relayCacheSuccessMarkerName = ".relay-complete"

func NewExecutorService(db *gorm.DB, cfg config.Config, runner executorpkg.Runner, taskManager *executorpkg.TaskManager, dispatchers ...notificationDispatcher) *ExecutorService {
	if runner == nil {
		runner = executorpkg.NewExecRunner()
	}
	if taskManager == nil {
		taskManager = executorpkg.NewTaskManager()
	}
	var dispatcher notificationDispatcher
	if len(dispatchers) > 0 {
		dispatcher = dispatchers[0]
	}

	return &ExecutorService{
		db:               db,
		config:           cfg,
		instanceRepo:     repository.NewInstanceRepository(db),
		strategyRepo:     repository.NewStrategyRepository(db),
		storageTargetRepo: repository.NewStorageTargetRepository(db),
		sshKeyRepo:       repository.NewSSHKeyRepository(db),
		retentionService: NewRetentionService(db),
		taskManager:      taskManager,
		notificationDispatcher: dispatcher,
		runner:           runner,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *ExecutorService) RunStrategy(ctx context.Context, strategy model.Strategy) error {
	if s == nil {
		return nil
	}
	switch strings.TrimSpace(strategy.BackupType) {
	case BackupTypeRolling, BackupTypeCold:
	default:
		return fmt.Errorf("backup type %q is not implemented", strategy.BackupType)
	}

	if len(strategy.StorageTargets) == 0 {
		loadedStrategy, err := s.strategyRepo.GetByID(ctx, strategy.ID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStrategyNotFound
			}
			return err
		}
		strategy = loadedStrategy
	}

	instance, err := s.instanceRepo.GetByID(ctx, strategy.InstanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInstanceNotFound
		}
		return err
	}

	var runErrors []error
	for _, target := range strategy.StorageTargets {
		var err error
		switch strings.TrimSpace(strategy.BackupType) {
		case BackupTypeRolling:
			err = s.runRollingTarget(ctx, strategy, instance, target)
		case BackupTypeCold:
			err = s.runColdTarget(ctx, strategy, instance, target)
		}
		if err != nil {
			runErrors = append(runErrors, fmt.Errorf("target %d: %w", target.ID, err))
		}
	}

	return errors.Join(runErrors...)
}

func (s *ExecutorService) ListBackupRecords(ctx context.Context, req ListBackupRecordsRequest) ([]model.BackupRecord, error) {
	query := s.db.WithContext(ctx).
		Model(&model.BackupRecord{}).
		Where("instance_id = ?", req.InstanceID).
		Order("started_at DESC").
		Order("id DESC")

	if req.StrategyID != nil {
		query = query.Where("strategy_id = ?", *req.StrategyID)
	}
	if strings.TrimSpace(req.BackupType) != "" {
		query = query.Where("backup_type = ?", strings.TrimSpace(req.BackupType))
	}
	if strings.TrimSpace(req.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(req.Status))
	}
	if req.RestorableOnly {
		query = query.Where("status = ? AND snapshot_path <> ''", model.BackupStatusSuccess)
	}

	var records []model.BackupRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list backup records: %w", err)
	}

	return records, nil
}

func (s *ExecutorService) ListRestorableBackups(ctx context.Context, req ListBackupRecordsRequest) ([]model.BackupRecord, error) {
	req.RestorableOnly = true
	records, err := s.ListBackupRecords(ctx, req)
	if err != nil {
		return nil, err
	}

	filteredRecords := make([]model.BackupRecord, 0, len(records))
	targets := make(map[uint]model.StorageTarget)
	backends := make(map[uint]storage.StorageBackend)
	for _, record := range records {
		target, ok := targets[record.StorageTargetID]
		if !ok {
			target, err = s.storageTargetRepo.GetByID(ctx, record.StorageTargetID)
			if err != nil {
				return nil, err
			}
			targets[record.StorageTargetID] = target
		}

		backend, ok := backends[record.StorageTargetID]
		if !ok {
			backend, err = buildStorageBackend(ctx, s.sshKeyRepo, target)
			if err != nil {
				return nil, err
			}
			backends[record.StorageTargetID] = backend
		}

		exists, err := backupRecordArtifactsExist(ctx, backend, record, target)
		if err != nil {
			return nil, err
		}
		if exists {
			filteredRecords = append(filteredRecords, record)
		}
	}

	return filteredRecords, nil
}

func (s *ExecutorService) CheckTargetSpace(ctx context.Context, backend storage.StorageBackend, path string, estimatedSize uint64) error {
	if backend == nil {
		return nil
	}

	availableSpace, err := backend.SpaceAvailable(ctx, path)
	if err != nil {
		return fmt.Errorf("target space warning: unable to check available space at %q: %w", path, err)
	}
	if estimatedSize == 0 {
		return nil
	}
	if availableSpace < estimatedSize {
		return fmt.Errorf("target space warning: insufficient space at %q (estimated=%d available=%d)", path, estimatedSize, availableSpace)
	}

	return nil
}

func (s *ExecutorService) runRollingTarget(ctx context.Context, strategy model.Strategy, instance model.BackupInstance, target model.StorageTarget) error {
	lockKey := executorpkg.BuildTaskLockKey(instance.ID, target.ID)
	executionCtx, cancel := executorpkg.WithExecutionTimeout(ctx, strategy.MaxExecutionSeconds)
	task, ok := s.taskManager.TryStart(lockKey, cancel)
	if !ok {
		cancel()
		return executorpkg.NewTaskConflictError(lockKey)
	}
	defer func() {
		cancel()
		s.taskManager.Finish(task.ID)
	}()

	plan := executorpkg.BuildRollingPlan(instance, target)
	record, err := s.createBackupRecord(strategy, instance, target, plan.SnapshotPath)
	if err != nil {
		return err
	}
	if err := s.ensureLocalExecutionPaths(target, plan); err != nil {
		if completeErr := s.completeBackupRecord(record.ID, model.BackupStatusFailed, plan.SnapshotPath, err.Error(), executorpkg.ProgressSnapshot{}); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
		return err
	}

	backend, err := buildStorageBackend(executionCtx, s.sshKeyRepo, target)
	if err != nil {
		if completeErr := s.completeBackupRecord(record.ID, model.BackupStatusFailed, plan.SnapshotPath, err.Error(), executorpkg.ProgressSnapshot{}); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
		return err
	}

	spaceCheckPath := executorpkg.SnapshotRootDir(instance)
	if warningErr := s.CheckTargetSpace(executionCtx, backend, spaceCheckPath, s.estimateTargetSize(strategy, instance, target)); warningErr != nil {
		log.Printf("warning: %v", warningErr)
	}

	request, err := s.buildRollingExecutionRequest(executionCtx, instance, target, plan)
	if err != nil {
		if completeErr := s.completeBackupRecord(record.ID, model.BackupStatusFailed, plan.SnapshotPath, err.Error(), executorpkg.ProgressSnapshot{}); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
		return err
	}

	var lastProgress executorpkg.ProgressSnapshot
	executeErr := executorpkg.ExecuteRolling(executionCtx, s.runner, request, func(snapshot executorpkg.ProgressSnapshot) {
		lastProgress = snapshot
		if err := s.updateBackupRecordProgress(record.ID, snapshot); err != nil {
			log.Printf("warning: update backup record progress: %v", err)
		}
	})
	if executeErr != nil {
		if plan.RequiresRelay {
			if err := s.removeRelayCache(request.RelayCacheDir); err != nil {
				log.Printf("warning: remove failed relay cache: %v", err)
			}
		}

		status := model.BackupStatusFailed
		if errors.Is(executeErr, context.Canceled) || errors.Is(executeErr, context.DeadlineExceeded) {
			status = model.BackupStatusCancelled
		}
		if completeErr := s.completeBackupRecord(record.ID, status, plan.SnapshotPath, executeErr.Error(), lastProgress); completeErr != nil {
			return errors.Join(executeErr, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, status, executeErr.Error())
		return executeErr
	}

	var postProcessingErr error
	if plan.RequiresRelay {
		if err := s.markRelayCacheSuccessful(request.RelayCacheDir); err != nil {
			postProcessingErr = fmt.Errorf("mark relay cache complete: %w", err)
		}
	}

	if plan.RequiresRelay {
		if postProcessingErr == nil {
			if err := s.pruneRelayCache(request.RelayCacheDir); err != nil {
			postProcessingErr = fmt.Errorf("prune relay cache: %w", err)
			}
		}
	}

	if postProcessingErr == nil {
		if err := s.retentionService.Cleanup(executionCtx, strategy, target); err != nil {
			postProcessingErr = fmt.Errorf("cleanup retention: %w", err)
		}
	}

	if postProcessingErr != nil {
		if plan.RequiresRelay {
			if err := s.removeRelayCache(request.RelayCacheDir); err != nil {
				log.Printf("warning: remove failed relay cache: %v", err)
			}
		}

		if completeErr := s.completeBackupRecord(record.ID, model.BackupStatusFailed, plan.SnapshotPath, postProcessingErr.Error(), lastProgress); completeErr != nil {
			return errors.Join(postProcessingErr, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, postProcessingErr.Error())
		return postProcessingErr
	}

	if completeErr := s.completeBackupRecord(record.ID, model.BackupStatusSuccess, plan.SnapshotPath, "", lastProgress); completeErr != nil {
		return completeErr
	}
	s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusSuccess, "")

	return nil
}

func (s *ExecutorService) runColdTarget(ctx context.Context, strategy model.Strategy, instance model.BackupInstance, target model.StorageTarget) error {
	lockKey := executorpkg.BuildTaskLockKey(instance.ID, target.ID)
	executionCtx, cancel := executorpkg.WithExecutionTimeout(ctx, strategy.MaxExecutionSeconds)
	task, ok := s.taskManager.TryStart(lockKey, cancel)
	if !ok {
		cancel()
		return executorpkg.NewTaskConflictError(lockKey)
	}
	defer func() {
		cancel()
		s.taskManager.Finish(task.ID)
	}()

	archivePath, archiveRelativePath := s.buildColdArchivePaths(instance, target)
	record, err := s.createColdBackupRecord(strategy, instance, target, archivePath)
	if err != nil {
		return err
	}

	backend, err := buildStorageBackend(executionCtx, s.sshKeyRepo, target)
	if err != nil {
		if completeErr := s.completeColdBackupRecord(record.ID, model.BackupStatusFailed, archivePath, err.Error(), executorpkg.ColdBackupResult{}); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
		return err
	}

	excludePatterns, err := parseExcludePatterns(instance.ExcludePatterns)
	if err != nil {
		if completeErr := s.completeColdBackupRecord(record.ID, model.BackupStatusFailed, archivePath, err.Error(), executorpkg.ColdBackupResult{}); completeErr != nil {
			return errors.Join(err, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
		return err
	}

	var sourceSSHKeyPath string
	if instanceUsesRemoteSource(instance) {
		sourceSSHKeyPath, err = s.lookupSourceSSHKeyPath(executionCtx, instance)
		if err != nil {
			if completeErr := s.completeColdBackupRecord(record.ID, model.BackupStatusFailed, archivePath, err.Error(), executorpkg.ColdBackupResult{}); completeErr != nil {
				return errors.Join(err, completeErr)
			}
			s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, err.Error())
			return err
		}
	}

	var result executorpkg.ColdBackupResult
	coldExecutor := executorpkg.NewColdExecutor(s.runner)
	executeErr := coldExecutor.Run(executionCtx, executorpkg.ColdBackupRequest{
		Instance:            instance,
		Target:              target,
		Backend:             backend,
		SourceSSHKeyPath:    sourceSSHKeyPath,
		VolumeSize:          strategy.ColdVolumeSize,
		ArchivePath:         archivePath,
		ArchiveRelativePath: archiveRelativePath,
		TempRoot:            s.resolveTempRoot(),
		ExcludePatterns:     excludePatterns,
		Result:              &result,
	})
	if executeErr != nil {
		status := model.BackupStatusFailed
		if errors.Is(executeErr, context.Canceled) || errors.Is(executeErr, context.DeadlineExceeded) {
			status = model.BackupStatusCancelled
		}
		if completeErr := s.completeColdBackupRecord(record.ID, status, archivePath, executeErr.Error(), result); completeErr != nil {
			return errors.Join(executeErr, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, status, executeErr.Error())
		return executeErr
	}

	if completeErr := s.completeColdBackupRecord(record.ID, model.BackupStatusSuccess, archivePath, "", result); completeErr != nil {
		return completeErr
	}

	if err := s.retentionService.Cleanup(executionCtx, strategy, target); err != nil {
		cleanupErr := fmt.Errorf("cleanup retention: %w", err)
		if completeErr := s.completeColdBackupRecord(record.ID, model.BackupStatusFailed, archivePath, cleanupErr.Error(), result); completeErr != nil {
			return errors.Join(cleanupErr, completeErr)
		}
		s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusFailed, cleanupErr.Error())
		return cleanupErr
	}
	s.dispatchBackupNotification(strategy, instance, record, model.BackupStatusSuccess, "")

	return nil
}

func instanceUsesRemoteSource(instance model.BackupInstance) bool {
	return strings.EqualFold(strings.TrimSpace(instance.SourceType), SourceTypeRemote)
}

func (s *ExecutorService) ensureLocalExecutionPaths(target model.StorageTarget, plan executorpkg.RollingPlan) error {
	if !isSSHStorageTargetType(target.Type) && strings.TrimSpace(plan.SnapshotPath) != "" {
		if err := os.MkdirAll(filepath.Dir(plan.SnapshotPath), 0o755); err != nil {
			return fmt.Errorf("ensure snapshot directory parent: %w", err)
		}
	}

	relayCachePath := s.resolveRelayCachePath(plan.RelayCacheDir)
	if strings.TrimSpace(relayCachePath) != "" {
		if err := os.MkdirAll(filepath.Dir(relayCachePath), 0o755); err != nil {
			return fmt.Errorf("ensure relay cache directory parent: %w", err)
		}
	}

	return nil
}

func (s *ExecutorService) buildRollingExecutionRequest(ctx context.Context, instance model.BackupInstance, target model.StorageTarget, plan executorpkg.RollingPlan) (executorpkg.RollingExecutionRequest, error) {
	excludePatterns, err := parseExcludePatterns(instance.ExcludePatterns)
	if err != nil {
		return executorpkg.RollingExecutionRequest{}, err
	}

	request := executorpkg.RollingExecutionRequest{
		Instance:        instance,
		Target:          target,
		SnapshotPath:    plan.SnapshotPath,
		ExcludePatterns: excludePatterns,
	}

	if strings.EqualFold(strings.TrimSpace(instance.SourceType), SourceTypeRemote) {
		sourceSSHKeyPath, err := s.lookupSourceSSHKeyPath(ctx, instance)
		if err != nil {
			return executorpkg.RollingExecutionRequest{}, err
		}
		request.SourceSSHKeyPath = sourceSSHKeyPath
	}

	if isSSHStorageTargetType(target.Type) {
		targetSSHKeyPath, err := s.lookupTargetSSHKeyPath(ctx, target)
		if err != nil {
			return executorpkg.RollingExecutionRequest{}, err
		}
		request.TargetSSHKeyPath = targetSSHKeyPath
	}

	request.TargetLinkDest, err = s.findLatestSuccessfulSnapshot(instance.ID, target)
	if err != nil {
		return executorpkg.RollingExecutionRequest{}, err
	}

	if plan.RequiresRelay {
		request.RelayCacheDir = s.resolveRelayCachePath(plan.RelayCacheDir)
		request.RelayLinkDest, err = s.findLatestRelayCache(request.RelayCacheDir)
		if err != nil {
			return executorpkg.RollingExecutionRequest{}, err
		}
	}

	return request, nil
}

func (s *ExecutorService) createBackupRecord(strategy model.Strategy, instance model.BackupInstance, target model.StorageTarget, snapshotPath string) (model.BackupRecord, error) {
	strategyID := strategy.ID
	record := model.BackupRecord{
		InstanceID:      instance.ID,
		StorageTargetID: target.ID,
		StrategyID:      &strategyID,
		BackupType:      BackupTypeRolling,
		Status:          model.BackupStatusRunning,
		TargetLocationKey: storageTargetLocationKey(target),
		SnapshotPath:    snapshotPath,
		VolumeCount:     1,
		StartedAt:       s.clock(),
	}

	if err := s.db.Create(&record).Error; err != nil {
		return model.BackupRecord{}, fmt.Errorf("create backup record: %w", err)
	}

	return record, nil
}

func (s *ExecutorService) createColdBackupRecord(strategy model.Strategy, instance model.BackupInstance, target model.StorageTarget, archivePath string) (model.BackupRecord, error) {
	strategyID := strategy.ID
	record := model.BackupRecord{
		InstanceID:        instance.ID,
		StorageTargetID:   target.ID,
		StrategyID:        &strategyID,
		BackupType:        BackupTypeCold,
		Status:            model.BackupStatusRunning,
		TargetLocationKey: storageTargetLocationKey(target),
		SnapshotPath:      archivePath,
		VolumeCount:       1,
		StartedAt:         s.clock(),
	}

	if err := s.db.Create(&record).Error; err != nil {
		return model.BackupRecord{}, fmt.Errorf("create backup record: %w", err)
	}

	return record, nil
}

func (s *ExecutorService) buildColdArchivePaths(instance model.BackupInstance, target model.StorageTarget) (string, string) {
	timestamp := s.clock().UTC().Format("20060102T150405.000000000Z")
	archiveFileName := fmt.Sprintf("%s_%s.tar.gz", executorpkg.SnapshotRootName(instance.Name), timestamp)
	archiveRelativePath := filepath.Join(executorpkg.SnapshotRootDir(instance), archiveFileName)

	return filepath.Clean(filepath.Join(target.BasePath, archiveRelativePath)), filepath.Clean(archiveRelativePath)
}

func (s *ExecutorService) updateBackupRecordProgress(recordID uint, snapshot executorpkg.ProgressSnapshot) error {
	bytesTransferred := progressRecordBytes(snapshot)
	updates := map[string]any{
		"bytes_transferred": bytesTransferred,
	}
	totalSize := progressTotalSize(snapshot)
	if totalSize > 0 {
		updates["total_size"] = totalSize
	}

	if err := s.db.Model(&model.BackupRecord{}).Where("id = ?", recordID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update backup record progress: %w", err)
	}

	return nil
}

func (s *ExecutorService) completeBackupRecord(recordID uint, status, snapshotPath, errorMessage string, snapshot executorpkg.ProgressSnapshot) error {
	finishedAt := s.clock()
	bytesTransferred := progressRecordBytes(snapshot)
	updates := map[string]any{
		"status":            status,
		"snapshot_path":     snapshotPath,
		"bytes_transferred": bytesTransferred,
		"finished_at":       &finishedAt,
		"error_message":     errorMessage,
	}
	totalSize := progressTotalSize(snapshot)
	if totalSize > 0 {
		updates["total_size"] = totalSize
	}

	if err := s.db.Model(&model.BackupRecord{}).Where("id = ?", recordID).Updates(updates).Error; err != nil {
		return fmt.Errorf("complete backup record: %w", err)
	}

	return nil
}

func (s *ExecutorService) completeColdBackupRecord(recordID uint, status, archivePath, errorMessage string, result executorpkg.ColdBackupResult) error {
	finishedAt := s.clock()
	volumeCount := result.VolumeCount
	if volumeCount <= 0 {
		volumeCount = 1
	}
	updates := map[string]any{
		"status":            status,
		"snapshot_path":     archivePath,
		"bytes_transferred": result.BytesTransferred,
		"files_transferred": result.FilesTransferred,
		"total_size":        result.TotalSize,
		"volume_count":      volumeCount,
		"finished_at":       &finishedAt,
		"error_message":     errorMessage,
	}

	if err := s.db.Model(&model.BackupRecord{}).Where("id = ?", recordID).Updates(updates).Error; err != nil {
		return fmt.Errorf("complete cold backup record: %w", err)
	}

	return nil
}

func (s *ExecutorService) dispatchBackupNotification(strategy model.Strategy, instance model.BackupInstance, record model.BackupRecord, status, errorMessage string) {
	if s.notificationDispatcher == nil {
		return
	}
	if err := s.notificationDispatcher.Notify(context.Background(), buildBackupNotificationEvent(strategy, instance, record, status, errorMessage, s.clock())); err != nil {
		log.Printf("warning: dispatch backup notification: %v", err)
	}
}

func (s *ExecutorService) estimateTargetSize(strategy model.Strategy, instance model.BackupInstance, target model.StorageTarget) uint64 {
	var record model.BackupRecord
	query := s.db.Model(&model.BackupRecord{}).
		Where("instance_id = ? AND storage_target_id = ? AND backup_type = ? AND status = ?", instance.ID, target.ID, BackupTypeRolling, model.BackupStatusSuccess).
		Order("finished_at DESC").
		Order("id DESC")
	if strategy.ID != 0 {
		query = query.Where("strategy_id = ?", strategy.ID)
	}
	if err := query.First(&record).Error; err != nil {
		return 0
	}
	if record.TotalSize <= 0 {
		return 0
	}

	return uint64(record.TotalSize)
}

func (s *ExecutorService) lookupSourceSSHKeyPath(ctx context.Context, instance model.BackupInstance) (string, error) {
	if instance.SourceSSHKeyID == nil {
		return "", ErrSSHKeyRequired
	}

	sshKey, err := s.sshKeyRepo.GetByID(ctx, *instance.SourceSSHKeyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrSSHKeyNotFound
		}
		return "", err
	}

	return sshKey.PrivateKeyPath, nil
}

func (s *ExecutorService) lookupTargetSSHKeyPath(ctx context.Context, target model.StorageTarget) (string, error) {
	if target.SSHKeyID == nil {
		return "", ErrSSHKeyRequired
	}

	sshKey, err := s.sshKeyRepo.GetByID(ctx, *target.SSHKeyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrSSHKeyNotFound
		}
		return "", err
	}

	return sshKey.PrivateKeyPath, nil
}

func (s *ExecutorService) findLatestSuccessfulSnapshot(instanceID uint, target model.StorageTarget) (string, error) {
	var record model.BackupRecord
	if err := s.db.Model(&model.BackupRecord{}).
		Where("instance_id = ? AND target_location_key = ? AND backup_type = ? AND status = ? AND snapshot_path <> ''", instanceID, storageTargetLocationKey(target), BackupTypeRolling, model.BackupStatusSuccess).
		Order("finished_at DESC").
		Order("id DESC").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.findLegacySuccessfulSnapshot(instanceID, target)
		}
		return "", fmt.Errorf("find latest successful snapshot: %w", err)
	}

	return strings.TrimSpace(record.SnapshotPath), nil
}

func (s *ExecutorService) findLegacySuccessfulSnapshot(instanceID uint, target model.StorageTarget) (string, error) {
	if isSSHStorageTargetType(target.Type) {
		return "", nil
	}

	var records []model.BackupRecord
	if err := s.db.Model(&model.BackupRecord{}).
		Where("instance_id = ? AND (target_location_key = '' OR target_location_key IS NULL) AND backup_type = ? AND status = ? AND snapshot_path <> ''", instanceID, BackupTypeRolling, model.BackupStatusSuccess).
		Order("finished_at DESC").
		Order("id DESC").
		Find(&records).Error; err != nil {
		return "", fmt.Errorf("find legacy successful snapshot: %w", err)
	}

	for _, record := range records {
		snapshotPath := strings.TrimSpace(record.SnapshotPath)
		if snapshotPath == "" {
			continue
		}
		if snapshotWithinTargetBase(snapshotPath, target.BasePath) {
			return snapshotPath, nil
		}
	}

	return "", nil
}

func snapshotWithinTargetBase(snapshotPath string, basePath string) bool {
	cleanSnapshotPath := filepath.Clean(strings.TrimSpace(snapshotPath))
	cleanBasePath := filepath.Clean(strings.TrimSpace(basePath))
	if cleanSnapshotPath == "." || cleanBasePath == "." {
		return false
	}
	if cleanSnapshotPath == cleanBasePath {
		return true
	}

	return strings.HasPrefix(cleanSnapshotPath, cleanBasePath+string(filepath.Separator))
}

func (s *ExecutorService) resolveRelayCachePath(relayCacheDir string) string {
	trimmedRelayCacheDir := strings.TrimSpace(relayCacheDir)
	if trimmedRelayCacheDir == "" {
		return ""
	}
	if strings.TrimSpace(s.config.DataDir) == "" {
		return filepath.Clean(trimmedRelayCacheDir)
	}

	return filepath.Join(s.config.DataDir, trimmedRelayCacheDir)
}

func (s *ExecutorService) resolveTempRoot() string {
	if strings.TrimSpace(s.config.DataDir) == "" {
		return ""
	}

	return filepath.Join(s.config.DataDir, "tmp")
}

func (s *ExecutorService) findLatestRelayCache(currentRelayCacheDir string) (string, error) {
	trimmedCurrentDir := strings.TrimSpace(currentRelayCacheDir)
	if trimmedCurrentDir == "" {
		return "", nil
	}

	cacheRoot := filepath.Dir(trimmedCurrentDir)
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read relay cache root: %w", err)
	}

	currentName := filepath.Base(trimmedCurrentDir)
	priorDirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == currentName {
			continue
		}
		if _, err := os.Stat(filepath.Join(cacheRoot, entry.Name(), relayCacheSuccessMarkerName)); err != nil {
			continue
		}
		priorDirs = append(priorDirs, entry.Name())
	}
	if len(priorDirs) == 0 {
		return "", nil
	}

	sort.Strings(priorDirs)
	return filepath.Join(cacheRoot, priorDirs[len(priorDirs)-1]), nil
}

func (s *ExecutorService) pruneRelayCache(currentRelayCacheDir string) error {
	trimmedCurrentDir := strings.TrimSpace(currentRelayCacheDir)
	if trimmedCurrentDir == "" {
		return nil
	}

	cacheRoot := filepath.Dir(trimmedCurrentDir)
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	currentName := filepath.Base(trimmedCurrentDir)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == currentName {
			continue
		}
		if err := os.RemoveAll(filepath.Join(cacheRoot, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExecutorService) removeRelayCache(relayCacheDir string) error {
	trimmedRelayCacheDir := strings.TrimSpace(relayCacheDir)
	if trimmedRelayCacheDir == "" {
		return nil
	}

	if err := os.RemoveAll(trimmedRelayCacheDir); err != nil {
		return err
	}

	return nil
}

func (s *ExecutorService) markRelayCacheSuccessful(relayCacheDir string) error {
	trimmedRelayCacheDir := strings.TrimSpace(relayCacheDir)
	if trimmedRelayCacheDir == "" {
		return nil
	}

	if err := os.MkdirAll(trimmedRelayCacheDir, 0o755); err != nil {
		return err
	}

	markerPath := filepath.Join(trimmedRelayCacheDir, relayCacheSuccessMarkerName)
	if err := os.WriteFile(markerPath, []byte("ok\n"), 0o644); err != nil {
		return err
	}

	return nil
}

func parseExcludePatterns(raw string) ([]string, error) {
	trimmedRaw := strings.TrimSpace(raw)
	if trimmedRaw == "" {
		return []string{}, nil
	}

	var excludePatterns []string
	if err := json.Unmarshal([]byte(trimmedRaw), &excludePatterns); err != nil {
		return nil, fmt.Errorf("decode exclude patterns: %w", err)
	}

	return excludePatterns, nil
}

func progressTotalSize(snapshot executorpkg.ProgressSnapshot) int64 {
	if snapshot.EstimatedTotalSize > 0 {
		return int64(snapshot.EstimatedTotalSize)
	}
	if snapshot.BytesTransferred == 0 {
		return 0
	}
	phasePercentage := snapshot.PhasePercentage
	if phasePercentage <= 0 {
		phasePercentage = snapshot.Percentage
	}
	if phasePercentage <= 0 {
		return int64(snapshot.BytesTransferred)
	}

	phaseTransferred := snapshot.PhaseBytesTransferred
	if phaseTransferred == 0 {
		phaseTransferred = snapshot.BytesTransferred
	}
	totalSize := int64(float64(phaseTransferred) / (float64(phasePercentage) / 100.0))
	if totalSize < int64(snapshot.BytesTransferred) {
		return int64(snapshot.BytesTransferred)
	}

	return totalSize
}

func progressRecordBytes(snapshot executorpkg.ProgressSnapshot) int64 {
	totalSize := progressTotalSize(snapshot)
	if totalSize <= 0 {
		return int64(snapshot.BytesTransferred)
	}
	if snapshot.Percentage <= 0 {
		if int64(snapshot.BytesTransferred) > totalSize {
			return totalSize
		}
		return int64(snapshot.BytesTransferred)
	}

	logicalTransferred := int64(float64(totalSize) * float64(snapshot.Percentage) / 100.0)
	if logicalTransferred > totalSize {
		return totalSize
	}
	if logicalTransferred < 0 {
		return 0
	}

	return logicalTransferred
}