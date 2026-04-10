package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	backupcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const (
	coldTaskPrepStep     = "preparing workspace"
	coldTaskSyncStep     = "syncing source"
	coldTaskPackageStep  = "packaging backup"
	coldTaskCompressStep = "compressing backup"
	coldTaskEncryptStep  = "encrypting backup"
	coldTaskSplitStep    = "splitting backup"
	coldTaskMoveStep     = "moving backup"
	coldTaskDoneStep     = "completed"
	coldSplitPartFormat  = ".part%03d"
)

type coldBackupContextKey string

const coldBackupEncryptionKeyContextKey coldBackupContextKey = "cold-backup-encryption-key"

type coldBackupStats struct {
	Sync     RsyncStats  `json:"sync"`
	Delivery *RsyncStats `json:"delivery,omitempty"`
}

type ColdBackupExecutor struct {
	rsync *RsyncExecutor
	db    *store.DB

	dataDir      string
	now          func() time.Time
	dialSSH      func(context.Context, model.RemoteConfig) (*ssh.Client, error)
	executeRsync func(context.Context, RsyncConfig, func(ProgressInfo)) (*RsyncResult, error)
	newCommand   commandFactory
	encryptFile  func(context.Context, string, string, []byte) error
}

func NewColdBackupExecutor(rsync *RsyncExecutor, db *store.DB, dataDir string) *ColdBackupExecutor {
	if rsync == nil {
		rsync = NewRsyncExecutor()
	}

	executor := &ColdBackupExecutor{
		rsync:      rsync,
		db:         db,
		dataDir:    strings.TrimSpace(dataDir),
		now:        func() time.Time { return time.Now().UTC() },
		dialSSH:    service.DialSSHClient,
		newCommand: exec.CommandContext,
		encryptFile: func(ctx context.Context, inputPath, outputPath string, key []byte) error {
			return backupcrypto.EncryptFileWithContext(ctx, inputPath, outputPath, key)
		},
	}
	executor.executeRsync = executor.rsync.Execute
	if executor.dataDir == "" {
		executor.dataDir = resolveRollingDataDir()
	}

	return executor
}

func WithColdBackupEncryptionKey(ctx context.Context, key string) context.Context {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, coldBackupEncryptionKeyContextKey, trimmed)
}

func ColdBackupEncryptionKeyFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(coldBackupEncryptionKeyContextKey).(string)
	return strings.TrimSpace(value)
}

func (e *ColdBackupExecutor) Execute(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
	if err := e.validateInputs(task, policy, instance, target); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	sourceRemote, err := e.loadRemoteConfig(instance.SourceType, instance.RemoteConfigID, "source")
	if err != nil {
		return err
	}
	targetRemote, err := e.loadRemoteConfig(target.StorageType, target.RemoteConfigID, "target")
	if err != nil {
		return err
	}

	storageKey := backupInstanceStorageKey(instance)
	runName, runDir, err := e.allocateRunDirectory(ctx, target, targetRemote, storageKey)
	if err != nil {
		return err
	}

	backup, err := e.startRun(task, policy, instance, runDir)
	if err != nil {
		return err
	}

	tempRoot := filepath.Join(e.dataDir, "temp", strconv.FormatInt(task.ID, 10))
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("create temp workspace %q: %w", tempRoot, err))
	}
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	if err := ctx.Err(); err != nil {
		return e.finishRun(task, backup, err)
	}

	tempDataDir := filepath.Join(tempRoot, "data")
	if err := os.MkdirAll(tempDataDir, 0o755); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("create temp data dir %q: %w", tempDataDir, err))
	}

	if err := e.updateTaskStep(task, coldTaskSyncStep, 0, nil); err != nil {
		return e.finishRun(task, backup, err)
	}

	var syncProgressErr error
	syncResult, err := e.executeRsync(ctx, RsyncConfig{
		SourcePath:      instance.SourcePath,
		SourceType:      instance.SourceType,
		SourceRemote:    sourceRemote,
		DestPath:        tempDataDir,
		DestType:        "local",
		ExcludePatterns: instance.ExcludePatterns,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 0, 50)
		if updateErr := e.reportProgress(task, coldTaskSyncStep, mapped, progressCb); updateErr != nil {
			syncProgressErr = errors.Join(syncProgressErr, updateErr)
		}
	})
	if err != nil {
		return e.finishRun(task, backup, err)
	}
	if syncProgressErr != nil {
		return e.finishRun(task, backup, syncProgressErr)
	}

	artifactPath := tempDataDir
	artifactName := fmt.Sprintf("%s-%s", sanitizeBackupName(instance.Name), runName)

	if e.requiresPackedArtifact(policy, target) {
		step := coldTaskPackageStep
		extension := ".tar"
		compress := false
		if policy.Compression {
			step = coldTaskCompressStep
			extension = ".tar.gz"
			compress = true
		}
		if err := e.updateTaskStep(task, step, 50, nil); err != nil {
			return e.finishRun(task, backup, err)
		}

		archivePath := filepath.Join(tempRoot, artifactName+extension)
		if err := e.createArchive(ctx, tempRoot, archivePath, compress); err != nil {
			return e.finishRun(task, backup, err)
		}
		artifactPath = archivePath
		artifactName += extension
		if err := e.updateTaskStep(task, step, 65, nil); err != nil {
			return e.finishRun(task, backup, err)
		}
	}

	if policy.Encryption {
		if err := e.updateTaskStep(task, coldTaskEncryptStep, 65, nil); err != nil {
			return e.finishRun(task, backup, err)
		}

		encryptionKey, err := e.resolveEncryptionKey(ctx, policy)
		if err != nil {
			return e.finishRun(task, backup, err)
		}
		encryptedPath := artifactPath + ".enc"
		if err := e.encryptFile(ctx, artifactPath, encryptedPath, encryptionKey); err != nil {
			return e.finishRun(task, backup, err)
		}
		artifactPath = encryptedPath
		artifactName += ".enc"
		if err := e.updateTaskStep(task, coldTaskEncryptStep, 80, nil); err != nil {
			return e.finishRun(task, backup, err)
		}
	}

	stageDir := filepath.Join(tempRoot, "stage")
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("create staging dir %q: %w", stageDir, err))
	}

	stageEntries := make([]string, 0, 4)
	if policy.SplitEnabled {
		if err := e.updateTaskStep(task, coldTaskSplitStep, 80, nil); err != nil {
			return e.finishRun(task, backup, err)
		}

		partPaths, err := e.splitFile(ctx, artifactPath, policy.SplitSizeMB)
		if err != nil {
			return e.finishRun(task, backup, err)
		}
		for _, partPath := range partPaths {
			entryName := filepath.Base(partPath)
			if err := movePath(partPath, filepath.Join(stageDir, entryName)); err != nil {
				return e.finishRun(task, backup, fmt.Errorf("stage split part %q: %w", partPath, err))
			}
			stageEntries = append(stageEntries, entryName)
		}
		if err := e.updateTaskStep(task, coldTaskSplitStep, 90, nil); err != nil {
			return e.finishRun(task, backup, err)
		}
	} else {
		entryName := artifactName
		if !e.requiresPackedArtifact(policy, target) && !policy.Encryption {
			entryName = fmt.Sprintf("%s-%s", sanitizeBackupName(instance.Name), runName)
		}
		if err := movePath(artifactPath, filepath.Join(stageDir, entryName)); err != nil {
			return e.finishRun(task, backup, fmt.Errorf("stage backup artifact %q: %w", artifactPath, err))
		}
		stageEntries = append(stageEntries, entryName)
	}

	sort.Strings(stageEntries)
	totalBackupSize, err := stagedEntriesSize(stageDir, stageEntries)
	if err != nil {
		return e.finishRun(task, backup, err)
	}

	if err := e.updateTaskStep(task, coldTaskMoveStep, 90, nil); err != nil {
		return e.finishRun(task, backup, err)
	}

	deliveryStats, err := e.deliverArtifacts(ctx, target, targetRemote, runDir, stageDir, stageEntries, task, progressCb)
	if err != nil {
		return e.finishRun(task, backup, err)
	}

	backup.SnapshotPath = joinArtifactPath(target.StorageType, runDir, stageEntries[0])
	statsJSON, err := json.Marshal(coldBackupStats{
		Sync:     syncResult.Stats,
		Delivery: deliveryStats,
	})
	if err != nil {
		return e.finishRun(task, backup, fmt.Errorf("marshal cold backup stats: %w", err))
	}

	return e.completeRun(task, backup, totalBackupSize, syncResult.Stats.TotalSize, string(statsJSON))
}

func (e *ColdBackupExecutor) validateInputs(task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget) error {
	if e == nil {
		return fmt.Errorf("cold backup executor is nil")
	}
	if e.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if e.rsync == nil {
		e.rsync = NewRsyncExecutor()
	}
	if e.executeRsync == nil {
		e.executeRsync = e.rsync.Execute
	}
	if e.now == nil {
		e.now = func() time.Time { return time.Now().UTC() }
	}
	if e.dialSSH == nil {
		e.dialSSH = service.DialSSHClient
	}
	if e.newCommand == nil {
		e.newCommand = exec.CommandContext
	}
	if e.encryptFile == nil {
		e.encryptFile = func(ctx context.Context, inputPath, outputPath string, key []byte) error {
			return backupcrypto.EncryptFileWithContext(ctx, inputPath, outputPath, key)
		}
	}
	if strings.TrimSpace(e.dataDir) == "" {
		e.dataDir = resolveRollingDataDir()
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}
	if policy.Type != "cold" {
		return fmt.Errorf("cold executor only supports cold policies")
	}
	if instance == nil {
		return fmt.Errorf("instance is nil")
	}
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if target.BackupType != "cold" {
		return fmt.Errorf("cold executor only supports cold targets")
	}
	if normalizeRsyncType(target.StorageType) != "local" && normalizeRsyncType(target.StorageType) != "ssh" && normalizeRsyncType(target.StorageType) != "openlist" {
		return fmt.Errorf("cold backup target storage type %q is not supported", target.StorageType)
	}

	return nil
}

func (e *ColdBackupExecutor) loadRemoteConfig(endpointType string, remoteConfigID *int64, role string) (*model.RemoteConfig, error) {
	normalizedType := normalizeRsyncType(endpointType)
	if normalizedType != "ssh" && normalizedType != "openlist" {
		return nil, nil
	}
	if remoteConfigID == nil {
		return nil, fmt.Errorf("%s %s remote config is required", role, normalizedType)
	}

	remote, err := e.db.GetRemoteConfigByID(*remoteConfigID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s remote config %d not found", role, *remoteConfigID)
		}
		return nil, err
	}
	if normalizedType == "ssh" && remote.Type != "ssh" {
		return nil, fmt.Errorf("%s remote config %d must be ssh", role, remote.ID)
	}
	if normalizedType == "openlist" && !openlist.IsRemoteConfig(*remote) {
		return nil, fmt.Errorf("%s remote config %d must be openlist", role, remote.ID)
	}

	return remote, nil
}

func (e *ColdBackupExecutor) allocateRunDirectory(ctx context.Context, target *model.BackupTarget, remote *model.RemoteConfig, instanceStorageKey string) (string, string, error) {
	basePath := strings.TrimSpace(target.StoragePath)
	if basePath == "" {
		return "", "", fmt.Errorf("target storage path is required")
	}
	if strings.TrimSpace(instanceStorageKey) == "" {
		return "", "", fmt.Errorf("instance storage key is required")
	}

	storageType := normalizeRsyncType(target.StorageType)
	for attempt := 0; attempt < 100; attempt++ {
		runName := e.snapshotName(attempt)
		runDir := joinStoragePath(storageType, basePath, instanceStorageKey, runName)
		available, err := e.runDirectoryAvailable(ctx, storageType, runDir, remote)
		if err != nil {
			return "", "", err
		}
		if available {
			return runName, runDir, nil
		}
	}

	return "", "", fmt.Errorf("allocate unique cold backup target path for instance %q: too many collisions", instanceStorageKey)
}

func (e *ColdBackupExecutor) snapshotName(attempt int) string {
	base := e.now().Format("20060102-150405")
	if attempt == 0 {
		return base
	}
	return fmt.Sprintf("%s-%02d", base, attempt)
}

func (e *ColdBackupExecutor) runDirectoryAvailable(ctx context.Context, storageType, runDir string, remote *model.RemoteConfig) (bool, error) {
	switch normalizeRsyncType(storageType) {
	case "local":
		_, err := os.Stat(runDir)
		if err == nil {
			return false, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat cold backup target path %q: %w", runDir, err)
	case "ssh":
		client, err := e.connectSSH(ctx, remote)
		if err != nil {
			return false, err
		}
		defer client.Close()

		stdout, stderr, err := runSSHCommand(ctx, client, "if [ -e "+shellQuote(runDir)+" ]; then echo exists; fi")
		if err != nil {
			return false, fmt.Errorf("check remote cold backup path %q: %w (%s)", runDir, err, strings.TrimSpace(stderr))
		}
		return strings.TrimSpace(stdout) == "", nil
	case "openlist":
		session, err := e.openListSession(ctx, remote)
		if err != nil {
			return false, err
		}
		_, err = session.Get(ctx, runDir)
		if errors.Is(err, openlist.ErrNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported target storage type %q", storageType)
	}
}

func (e *ColdBackupExecutor) startRun(task *model.Task, policy *model.Policy, instance *model.Instance, snapshotPath string) (*model.Backup, error) {
	startedAt := e.now()

	var backup *model.Backup
	if task.BackupID != nil {
		loaded, err := e.db.GetBackupByID(*task.BackupID)
		if err != nil {
			return nil, err
		}
		backup = loaded
	} else {
		backup = &model.Backup{}
	}

	backup.InstanceID = instance.ID
	backup.PolicyID = policy.ID
	backup.Type = policy.Type
	backup.Status = "running"
	backup.SnapshotPath = snapshotPath
	backup.BackupSizeBytes = 0
	backup.ActualSizeBytes = 0
	backup.StartedAt = &startedAt
	backup.CompletedAt = nil
	backup.DurationSeconds = 0
	backup.ErrorMessage = ""
	backup.RsyncStats = ""

	if backup.ID == 0 {
		if err := e.db.CreateBackup(backup); err != nil {
			return nil, err
		}
	} else {
		if err := e.db.UpdateBackup(backup); err != nil {
			return nil, err
		}
	}

	task.InstanceID = instance.ID
	task.Type = policy.Type
	task.BackupID = &backup.ID
	task.Status = "running"
	task.Progress = 0
	task.CurrentStep = coldTaskPrepStep
	task.StartedAt = &startedAt
	task.CompletedAt = nil
	task.EstimatedEnd = nil
	task.ErrorMessage = ""

	if task.ID == 0 {
		if err := e.db.CreateTask(task); err != nil {
			return nil, err
		}
	} else {
		if err := e.db.UpdateTask(task); err != nil {
			return nil, err
		}
	}

	return backup, nil
}

func (e *ColdBackupExecutor) finishRun(task *model.Task, backup *model.Backup, runErr error) error {
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return e.cancelRun(task, backup, runErr)
	}
	return e.failRun(task, backup, runErr)
}

func (e *ColdBackupExecutor) failRun(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := e.now()
	var persistErr error

	if backup != nil {
		backup.Status = "failed"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = strings.TrimSpace(runErr.Error())
		if err := e.db.UpdateBackup(backup); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}
	if task != nil {
		task.Status = "failed"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = strings.TrimSpace(runErr.Error())
		if err := e.db.UpdateTask(task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}

	return runErr
}

func (e *ColdBackupExecutor) cancelRun(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := e.now()
	var persistErr error

	if backup != nil {
		backup.Status = "cancelled"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = strings.TrimSpace(runErr.Error())
		if err := e.db.UpdateBackup(backup); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}
	if task != nil {
		task.Status = "cancelled"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = strings.TrimSpace(runErr.Error())
		if err := e.db.UpdateTask(task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}

	return runErr
}

func (e *ColdBackupExecutor) completeRun(task *model.Task, backup *model.Backup, backupSizeBytes, actualSizeBytes int64, rsyncStats string) error {
	completedAt := e.now()

	backup.Status = "success"
	backup.CompletedAt = &completedAt
	backup.BackupSizeBytes = backupSizeBytes
	backup.ActualSizeBytes = actualSizeBytes
	backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
	backup.ErrorMessage = ""
	backup.RsyncStats = rsyncStats
	if err := e.db.UpdateBackup(backup); err != nil {
		return err
	}

	task.Status = "success"
	task.Progress = 100
	task.CurrentStep = coldTaskDoneStep
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = ""
	return e.db.UpdateTask(task)
}

func (e *ColdBackupExecutor) reportProgress(task *model.Task, step string, progress ProgressInfo, externalCb func(ProgressInfo)) error {
	task.CurrentStep = step
	if err := e.updateTaskProgress(task, progress); err != nil {
		return err
	}
	if externalCb != nil {
		externalCb(progress)
	}
	return nil
}

func (e *ColdBackupExecutor) updateTaskProgress(task *model.Task, progress ProgressInfo) error {
	if e == nil || e.db == nil || task == nil {
		return nil
	}

	task.Progress = clampProgress(progress.Percentage)
	if remaining, ok := parseRemainingDuration(progress.Remaining); ok {
		estimatedEnd := e.now().Add(remaining)
		task.EstimatedEnd = &estimatedEnd
	} else {
		task.EstimatedEnd = nil
	}

	return e.db.UpdateTask(task)
}

func (e *ColdBackupExecutor) updateTaskStep(task *model.Task, step string, progress int, estimatedEnd *time.Time) error {
	if task == nil {
		return nil
	}
	task.CurrentStep = step
	task.Progress = clampProgress(progress)
	task.EstimatedEnd = estimatedEnd
	return e.db.UpdateTask(task)
}

func (e *ColdBackupExecutor) resolveEncryptionKey(ctx context.Context, policy *model.Policy) ([]byte, error) {
	key := ColdBackupEncryptionKeyFromContext(ctx)
	if key == "" {
		return nil, fmt.Errorf("encryption key is required for cold backup execution")
	}
	if policy.EncryptionKeyHash != nil && *policy.EncryptionKeyHash != "" && !backupcrypto.ValidateEncryptionKey(key, *policy.EncryptionKeyHash) {
		return nil, fmt.Errorf("encryption key does not match policy")
	}
	return []byte(key), nil
}

func (e *ColdBackupExecutor) createArchive(ctx context.Context, tempRoot, outputPath string, compress bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := []string{"-cf", outputPath, "-C", tempRoot, "data"}
	if compress {
		args[0] = "-czf"
	}
	cmd := e.newCommand(ctx, "tar", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("run tar %v: %w (%s)", args, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (e *ColdBackupExecutor) splitFile(ctx context.Context, inputPath string, splitSizeMB *int) ([]string, error) {
	if splitSizeMB == nil || *splitSizeMB <= 0 {
		return nil, fmt.Errorf("split_size_mb must be positive when split is enabled")
	}
	partSize := int64(*splitSizeMB) * 1024 * 1024
	if partSize <= 0 {
		return nil, fmt.Errorf("split_size_mb must be positive when split is enabled")
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("open split input %q: %w", inputPath, err)
	}
	defer inputFile.Close()

	bufferSize := partSize
	if bufferSize > 4*1024*1024 {
		bufferSize = 4 * 1024 * 1024
	}
	if bufferSize < 64*1024 {
		bufferSize = 64 * 1024
	}
	buffer := make([]byte, bufferSize)

	partPaths := make([]string, 0, 4)
	cleanupParts := func() {
		for _, path := range partPaths {
			_ = os.Remove(path)
		}
	}
	defer func() {
		if err != nil {
			cleanupParts()
		}
	}()

	for partIndex := 1; ; partIndex++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		partPath := inputPath + fmt.Sprintf(coldSplitPartFormat, partIndex)
		partFile, openErr := os.OpenFile(partPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if openErr != nil {
			return nil, fmt.Errorf("create split part %q: %w", partPath, openErr)
		}

		written := int64(0)
		for written < partSize {
			if err := ctx.Err(); err != nil {
				partFile.Close()
				return nil, err
			}

			remaining := partSize - written
			readSize := len(buffer)
			if remaining < int64(readSize) {
				readSize = int(remaining)
			}

			readBytes, readErr := inputFile.Read(buffer[:readSize])
			if readBytes > 0 {
				if _, writeErr := partFile.Write(buffer[:readBytes]); writeErr != nil {
					partFile.Close()
					return nil, fmt.Errorf("write split part %q: %w", partPath, writeErr)
				}
				written += int64(readBytes)
			}

			if readErr == nil {
				continue
			}
			if readErr == io.EOF {
				break
			}
			partFile.Close()
			return nil, fmt.Errorf("read split input %q: %w", inputPath, readErr)
		}

		if closeErr := partFile.Close(); closeErr != nil {
			return nil, fmt.Errorf("close split part %q: %w", partPath, closeErr)
		}

		if written == 0 {
			_ = os.Remove(partPath)
			break
		}

		partPaths = append(partPaths, partPath)
		if written < partSize {
			break
		}
	}

	if len(partPaths) == 0 {
		return nil, fmt.Errorf("split input %q produced no output", inputPath)
	}

	if removeErr := os.Remove(inputPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return nil, fmt.Errorf("remove original split input %q: %w", inputPath, removeErr)
	}

	return partPaths, nil
}

func (e *ColdBackupExecutor) deliverArtifacts(ctx context.Context, target *model.BackupTarget, remote *model.RemoteConfig, runDir, stageDir string, entries []string, task *model.Task, progressCb func(ProgressInfo)) (*RsyncStats, error) {
	switch normalizeRsyncType(target.StorageType) {
	case "local":
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			return nil, fmt.Errorf("create final cold backup directory %q: %w", runDir, err)
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if err := movePath(filepath.Join(stageDir, entry), filepath.Join(runDir, entry)); err != nil {
				return nil, fmt.Errorf("move cold backup artifact %q: %w", entry, err)
			}
		}
		if progressCb != nil {
			progressCb(ProgressInfo{Percentage: 100})
		}
		return nil, nil
	case "ssh":
		client, err := e.connectSSH(ctx, remote)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		if _, stderr, err := runSSHCommand(ctx, client, "mkdir -p "+shellQuote(runDir)); err != nil {
			return nil, fmt.Errorf("create remote cold backup directory %q: %w (%s)", runDir, err, strings.TrimSpace(stderr))
		}

		var progressErr error
		result, err := e.executeRsync(ctx, RsyncConfig{
			SourcePath: stageDir,
			SourceType: "local",
			DestPath:   runDir,
			DestType:   "ssh",
			DestRemote: remote,
		}, func(progress ProgressInfo) {
			mapped := scaleProgress(progress, 90, 100)
			if updateErr := e.reportProgress(task, coldTaskMoveStep, mapped, progressCb); updateErr != nil {
				progressErr = errors.Join(progressErr, updateErr)
			}
		})
		if err != nil {
			return nil, err
		}
		if progressErr != nil {
			return nil, progressErr
		}
		return &result.Stats, nil
	case "openlist":
		session, err := e.openListSession(ctx, remote)
		if err != nil {
			return nil, err
		}
		if err := session.EnsureDir(ctx, runDir); err != nil {
			return nil, err
		}
		for index, entry := range entries {
			artifactPath := filepath.Join(stageDir, entry)
			info, err := os.Stat(artifactPath)
			if err != nil {
				return nil, fmt.Errorf("stat staged openlist artifact %q: %w", artifactPath, err)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("openlist targets only support file artifacts")
			}
			if err := session.UploadFile(ctx, artifactPath, pathpkg.Join(runDir, entry)); err != nil {
				return nil, err
			}
			if progressCb != nil {
				progressCb(ProgressInfo{Percentage: 90 + ((index + 1) * 10 / len(entries))})
			}
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported target storage type %q", target.StorageType)
	}
}

func (e *ColdBackupExecutor) connectSSH(ctx context.Context, remote *model.RemoteConfig) (*ssh.Client, error) {
	if remote == nil {
		return nil, fmt.Errorf("ssh remote config is required")
	}
	client, err := e.dialSSH(ctx, *remote)
	if err != nil {
		return nil, fmt.Errorf("connect ssh remote %d: %w", remote.ID, err)
	}
	return client, nil
}

func (e *ColdBackupExecutor) openListSession(ctx context.Context, remote *model.RemoteConfig) (*openlist.Session, error) {
	if remote == nil {
		return nil, fmt.Errorf("openlist remote config is required")
	}
	config, err := openlist.ParseConfig(*remote)
	if err != nil {
		return nil, err
	}
	return openlist.NewClient(nil).Open(ctx, config)
}

func (e *ColdBackupExecutor) requiresPackedArtifact(policy *model.Policy, target *model.BackupTarget) bool {
	if target != nil && normalizeRsyncType(target.StorageType) == "openlist" {
		return true
	}
	return policy.Compression || policy.Encryption || policy.SplitEnabled
}

func sanitizeBackupName(name string) string {
	replacer := strings.NewReplacer("/", "_", `\`, "_")
	trimmed := strings.TrimSpace(replacer.Replace(name))
	if trimmed == "" {
		return "backup"
	}
	return trimmed
}

func stagedEntriesSize(stageDir string, entries []string) (int64, error) {
	var total int64
	for _, entry := range entries {
		size, err := pathSize(filepath.Join(stageDir, entry))
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

func pathSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat path %q: %w", path, err)
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	err = filepath.Walk(path, func(current string, currentInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentInfo.IsDir() {
			return nil
		}
		total += currentInfo.Size()
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk path %q: %w", path, err)
	}
	return total, nil
}

func movePath(sourcePath, destPath string) error {
	if err := os.Rename(sourcePath, destPath); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := copyDir(sourcePath, destPath); err != nil {
			return err
		}
	} else {
		if err := copyFilePath(sourcePath, destPath, info.Mode()); err != nil {
			return err
		}
	}

	return os.RemoveAll(sourcePath)
}

func copyDir(sourcePath, destPath string) error {
	return filepath.Walk(sourcePath, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(sourcePath, current)
		if err != nil {
			return err
		}
		targetPath := destPath
		if relPath != "." {
			targetPath = filepath.Join(destPath, relPath)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		return copyFilePath(current, targetPath, info.Mode())
	})
}

func copyFilePath(sourcePath, destPath string, mode os.FileMode) error {
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	outputFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outputFile, inputFile); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

func joinArtifactPath(storageType, dirPath, name string) string {
	if normalizeRsyncType(storageType) == "ssh" || normalizeRsyncType(storageType) == "openlist" {
		return pathpkg.Join(dirPath, name)
	}
	return filepath.Join(dirPath, name)
}
