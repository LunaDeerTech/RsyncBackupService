package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const (
	defaultRollingDataDir = "./data"
	rollingTaskQueuedStep = "queued"
	rollingTaskPrepStep   = "preparing snapshot"
	rollingTaskSyncStep   = "syncing snapshot"
	rollingTaskLatestStep = "updating latest link"
	rollingTaskPullStep   = "pulling to relay"
	rollingTaskPushStep   = "pushing to target"
	rollingTaskDoneStep   = "completed"
	relayCurrentDirName   = "current"
)

type RollingBackupExecutor struct {
	rs *RsyncExecutor
	db *store.DB

	dataDir      string
	now          func() time.Time
	dialSSH      func(context.Context, model.RemoteConfig) (*ssh.Client, error)
	executeRsync func(context.Context, RsyncConfig, func(ProgressInfo)) (*RsyncResult, error)
	removeAll    func(string) error
}

type relayStats struct {
	Mode string     `json:"mode"`
	Pull RsyncStats `json:"pull"`
	Push RsyncStats `json:"push"`
}

func NewRollingBackupExecutor(rsync *RsyncExecutor, db *store.DB) *RollingBackupExecutor {
	if rsync == nil {
		rsync = NewRsyncExecutor()
	}

	executor := &RollingBackupExecutor{
		rs:      rsync,
		db:      db,
		dataDir: resolveRollingDataDir(),
		now: func() time.Time {
			return time.Now().UTC()
		},
		dialSSH:   service.DialSSHClient,
		removeAll: os.RemoveAll,
	}
	executor.executeRsync = executor.rs.Execute

	return executor
}

func (e *RollingBackupExecutor) Execute(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
	if err := e.validateInputs(task, policy, instance, target); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if normalizeRsyncType(instance.SourceType) == "ssh" && normalizeRsyncType(target.StorageType) == "ssh" {
		return e.ExecuteRelay(ctx, task, policy, instance, target, progressCb)
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
	snapshotPath, latestLinkPath, err := e.allocateSnapshotPaths(ctx, target, targetRemote, storageKey)
	if err != nil {
		return err
	}

	linkDestPath, err := e.findLatestLinkDest(policy, instance, target)
	if err != nil {
		return err
	}

	backup, err := e.startRun(task, policy, instance, snapshotPath)
	if err != nil {
		return err
	}

	if err := e.prepareDirectory(ctx, target.StorageType, snapshotPath, targetRemote); err != nil {
		return e.finishRun(task, backup, err)
	}

	if err := e.updateTaskStep(task, rollingTaskSyncStep, 0, nil); err != nil {
		return e.finishRun(task, backup, err)
	}

	var progressErr error
	result, err := e.runRsync(ctx, RsyncConfig{
		SourcePath:       instance.SourcePath,
		SourceType:       instance.SourceType,
		SourceRemote:     sourceRemote,
		DestPath:         snapshotPath,
		DestType:         target.StorageType,
		DestRemote:       targetRemote,
		LinkDestPath:     linkDestPath,
		BandwidthLimitKB: policy.BandwidthLimitKB,
		ExcludePatterns:  instance.ExcludePatterns,
	}, func(progress ProgressInfo) {
		if updateErr := e.reportProgress(task, rollingTaskSyncStep, progress, progressCb); updateErr != nil {
			progressErr = errors.Join(progressErr, updateErr)
		}
	})
	if err != nil {
		return e.finishRun(task, backup, err)
	}
	if progressErr != nil {
		return e.finishRun(task, backup, progressErr)
	}

	if err := e.updateTaskStep(task, rollingTaskLatestStep, 99, nil); err != nil {
		return e.finishRun(task, backup, err)
	}
	if err := e.updateLatestLink(ctx, target.StorageType, latestLinkPath, snapshotPath, targetRemote); err != nil {
		return e.finishRun(task, backup, err)
	}

	statsJSON, err := json.Marshal(result.Stats)
	if err != nil {
		return e.finishRun(task, backup, fmt.Errorf("marshal rsync stats: %w", err))
	}

	return e.completeRun(task, backup, result.Stats.TransferSize, result.Stats.TotalSize, string(statsJSON))
}

func (e *RollingBackupExecutor) ExecuteRelay(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error {
	if err := e.validateInputs(task, policy, instance, target); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if normalizeRsyncType(instance.SourceType) != "ssh" || normalizeRsyncType(target.StorageType) != "ssh" {
		return fmt.Errorf("relay mode requires ssh source and ssh target")
	}
	storageKey := backupInstanceStorageKey(instance)

	sourceRemote, err := e.loadRemoteConfig(instance.SourceType, instance.RemoteConfigID, "source")
	if err != nil {
		return err
	}
	targetRemote, err := e.loadRemoteConfig(target.StorageType, target.RemoteConfigID, "target")
	if err != nil {
		return err
	}

	snapshotPath, latestLinkPath, err := e.allocateSnapshotPaths(ctx, target, targetRemote, storageKey)
	if err != nil {
		return err
	}
	remoteLinkDest, err := e.findLatestLinkDest(policy, instance, target)
	if err != nil {
		return err
	}

	relayBase := filepath.Join(e.dataDir, "relay", strconv.FormatInt(instance.ID, 10))
	relayCurrentPath := filepath.Join(relayBase, relayCurrentDirName)
	relayStagePath := filepath.Join(relayBase, filepath.Base(snapshotPath))
	if relayStagePath == relayCurrentPath {
		relayStagePath = filepath.Join(relayBase, filepath.Base(snapshotPath)+"-next")
	}

	backup, err := e.startRun(task, policy, instance, snapshotPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(relayBase, 0o755); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("create relay base %q: %w", relayBase, err))
	}
	if err := os.RemoveAll(relayStagePath); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("reset relay staging path %q: %w", relayStagePath, err))
	}
	if err := os.MkdirAll(relayStagePath, 0o755); err != nil {
		return e.finishRun(task, backup, fmt.Errorf("create relay staging path %q: %w", relayStagePath, err))
	}

	pullLinkDest := ""
	if info, statErr := os.Stat(relayCurrentPath); statErr == nil && info.IsDir() {
		pullLinkDest = relayCurrentPath
	}

	var pullProgressErr error
	pullResult, err := e.runRsync(ctx, RsyncConfig{
		SourcePath:       instance.SourcePath,
		SourceType:       instance.SourceType,
		SourceRemote:     sourceRemote,
		DestPath:         relayStagePath,
		DestType:         "local",
		LinkDestPath:     pullLinkDest,
		BandwidthLimitKB: policy.BandwidthLimitKB,
		ExcludePatterns:  instance.ExcludePatterns,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 0, 50)
		if updateErr := e.reportProgress(task, rollingTaskPullStep, mapped, progressCb); updateErr != nil {
			pullProgressErr = errors.Join(pullProgressErr, updateErr)
		}
	})
	if err != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, err)
	}
	if pullProgressErr != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, pullProgressErr)
	}

	if err := e.prepareDirectory(ctx, target.StorageType, snapshotPath, targetRemote); err != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, err)
	}

	var pushProgressErr error
	pushResult, err := e.runRsync(ctx, RsyncConfig{
		SourcePath:   relayStagePath,
		SourceType:   "local",
		DestPath:     snapshotPath,
		DestType:     target.StorageType,
		DestRemote:   targetRemote,
		LinkDestPath: remoteLinkDest,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 50, 100)
		if updateErr := e.reportProgress(task, rollingTaskPushStep, mapped, progressCb); updateErr != nil {
			pushProgressErr = errors.Join(pushProgressErr, updateErr)
		}
	})
	if err != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, err)
	}
	if pushProgressErr != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, pushProgressErr)
	}

	if err := e.updateTaskStep(task, rollingTaskLatestStep, 99, nil); err != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, err)
	}
	if err := e.updateLatestLink(ctx, target.StorageType, latestLinkPath, snapshotPath, targetRemote); err != nil {
		_ = os.RemoveAll(relayStagePath)
		return e.finishRun(task, backup, err)
	}
	if err := e.promoteRelaySnapshot(relayBase, relayStagePath, relayCurrentPath); err != nil {
		return e.finishRun(task, backup, err)
	}

	statsJSON, err := json.Marshal(relayStats{
		Mode: "relay",
		Pull: pullResult.Stats,
		Push: pushResult.Stats,
	})
	if err != nil {
		return e.finishRun(task, backup, fmt.Errorf("marshal relay rsync stats: %w", err))
	}

	return e.completeRun(task, backup, pushResult.Stats.TransferSize, pushResult.Stats.TotalSize, string(statsJSON))
}

func (e *RollingBackupExecutor) updateTaskProgress(task *model.Task, progress ProgressInfo) error {
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

func (e *RollingBackupExecutor) validateInputs(task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget) error {
	if e == nil {
		return fmt.Errorf("rolling backup executor is nil")
	}
	if e.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if e.rs == nil {
		e.rs = NewRsyncExecutor()
	}
	if e.executeRsync == nil {
		e.executeRsync = e.rs.Execute
	}
	if e.now == nil {
		e.now = func() time.Time { return time.Now().UTC() }
	}
	if e.dialSSH == nil {
		e.dialSSH = service.DialSSHClient
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
	if policy.Type != "rolling" {
		return fmt.Errorf("rolling executor only supports rolling policies")
	}
	if instance == nil {
		return fmt.Errorf("instance is nil")
	}
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if target.BackupType != "rolling" {
		return fmt.Errorf("rolling executor only supports rolling targets")
	}

	return nil
}

func (e *RollingBackupExecutor) loadRemoteConfig(endpointType string, remoteConfigID *int64, role string) (*model.RemoteConfig, error) {
	if normalizeRsyncType(endpointType) != "ssh" {
		return nil, nil
	}
	if remoteConfigID == nil {
		return nil, fmt.Errorf("%s ssh remote config is required", role)
	}

	remote, err := e.db.GetRemoteConfigByID(*remoteConfigID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s remote config %d not found", role, *remoteConfigID)
		}
		return nil, err
	}
	if remote.Type != "ssh" {
		return nil, fmt.Errorf("%s remote config %d must be ssh", role, remote.ID)
	}

	return remote, nil
}

func (e *RollingBackupExecutor) allocateSnapshotPaths(ctx context.Context, target *model.BackupTarget, remote *model.RemoteConfig, instanceName string) (string, string, error) {
	basePath := strings.TrimSpace(target.StoragePath)
	if basePath == "" {
		return "", "", fmt.Errorf("target storage path is required")
	}

	storageType := normalizeRsyncType(target.StorageType)
	for attempt := 0; attempt < 100; attempt++ {
		snapshotName := e.snapshotName(attempt)
		snapshotPath := joinStoragePath(storageType, basePath, instanceName, snapshotName)
		available, err := e.snapshotPathAvailable(ctx, storageType, snapshotPath, remote)
		if err != nil {
			return "", "", err
		}
		if !available {
			continue
		}

		latestLinkPath := joinStoragePath(storageType, basePath, instanceName, "latest")
		return snapshotPath, latestLinkPath, nil
	}

	return "", "", fmt.Errorf("allocate unique snapshot path for instance %q: too many collisions", instanceName)
}

func (e *RollingBackupExecutor) snapshotName(attempt int) string {
	base := e.now().Format("20060102-150405")
	if attempt == 0 {
		return base
	}
	return fmt.Sprintf("%s-%02d", base, attempt)
}

func (e *RollingBackupExecutor) snapshotPathAvailable(ctx context.Context, storageType, snapshotPath string, remote *model.RemoteConfig) (bool, error) {
	switch normalizeRsyncType(storageType) {
	case "local":
		_, err := os.Stat(snapshotPath)
		if err == nil {
			return false, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("stat snapshot path %q: %w", snapshotPath, err)
	case "ssh":
		return e.remotePathAvailable(ctx, remote, snapshotPath)
	default:
		return false, fmt.Errorf("unsupported target storage type %q", storageType)
	}
}

func (e *RollingBackupExecutor) remotePathAvailable(ctx context.Context, remote *model.RemoteConfig, snapshotPath string) (bool, error) {
	client, err := e.connectSSH(ctx, remote)
	if err != nil {
		return false, err
	}
	defer client.Close()

	stdout, stderr, err := runSSHCommand(ctx, client, "if [ -e "+shellQuote(snapshotPath)+" ]; then echo exists; fi")
	if err != nil {
		return false, fmt.Errorf("check remote snapshot path %q: %w (%s)", snapshotPath, err, strings.TrimSpace(stderr))
	}

	return strings.TrimSpace(stdout) == "", nil
}

func (e *RollingBackupExecutor) findLatestLinkDest(policy *model.Policy, instance *model.Instance, target *model.BackupTarget) (string, error) {
	backup, err := e.db.GetLatestSuccessfulBackup(instance.ID, policy.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	linkDest := strings.TrimSpace(backup.SnapshotPath)
	if linkDest == "" {
		return "", nil
	}
	if !snapshotPathBelongsToTarget(target, instance, linkDest) {
		return "", nil
	}

	return linkDest, nil
}

func (e *RollingBackupExecutor) startRun(task *model.Task, policy *model.Policy, instance *model.Instance, snapshotPath string) (*model.Backup, error) {
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
	task.CurrentStep = rollingTaskPrepStep
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

func (e *RollingBackupExecutor) finishRun(task *model.Task, backup *model.Backup, runErr error) error {
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return e.cancelRun(task, backup, runErr)
	}

	return e.failRun(task, backup, runErr)
}

func (e *RollingBackupExecutor) failRun(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := e.now()
	var persistErr error

	if backup != nil {
		backup.Status = "failed"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	if task != nil {
		task.Status = "failed"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	if backup != nil && task != nil {
		if err := e.db.UpdateBackupAndTask(backup, task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
		e.cleanupFailedSnapshot(backup)
	} else {
		if backup != nil {
			if err := e.db.UpdateBackup(backup); err != nil {
				persistErr = errors.Join(persistErr, err)
			}
			e.cleanupFailedSnapshot(backup)
		}
		if task != nil {
			if err := e.db.UpdateTask(task); err != nil {
				persistErr = errors.Join(persistErr, err)
			}
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}

	return runErr
}

func (e *RollingBackupExecutor) cancelRun(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := e.now()
	var persistErr error

	if backup != nil {
		backup.Status = "cancelled"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	if task != nil {
		task.Status = "cancelled"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = strings.TrimSpace(runErr.Error())
	}
	if backup != nil && task != nil {
		if err := e.db.UpdateBackupAndTask(backup, task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
		e.cleanupFailedSnapshot(backup)
	} else {
		if backup != nil {
			if err := e.db.UpdateBackup(backup); err != nil {
				persistErr = errors.Join(persistErr, err)
			}
			e.cleanupFailedSnapshot(backup)
		}
		if task != nil {
			if err := e.db.UpdateTask(task); err != nil {
				persistErr = errors.Join(persistErr, err)
			}
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}

	return runErr
}

func (e *RollingBackupExecutor) completeRun(task *model.Task, backup *model.Backup, backupSizeBytes, actualSizeBytes int64, rsyncStats string) error {
	completedAt := e.now()

	backup.Status = "success"
	backup.CompletedAt = &completedAt
	backup.BackupSizeBytes = backupSizeBytes
	backup.ActualSizeBytes = actualSizeBytes
	backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
	backup.ErrorMessage = ""
	backup.RsyncStats = rsyncStats

	task.Status = "success"
	task.Progress = 100
	task.CurrentStep = rollingTaskDoneStep
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = ""
	return e.db.UpdateBackupAndTask(backup, task)
}

func (e *RollingBackupExecutor) cleanupFailedSnapshot(backup *model.Backup) {
	if backup == nil || strings.TrimSpace(backup.SnapshotPath) == "" {
		return
	}
	if e.removeAll == nil {
		return
	}
	if err := e.removeAll(backup.SnapshotPath); err != nil {
		slog.Warn("cleanup failed rolling snapshot directory failed",
			"backup_id", backup.ID,
			"path", backup.SnapshotPath,
			"error", err,
		)
	}
}

func (e *RollingBackupExecutor) prepareDirectory(ctx context.Context, storageType, dir string, remote *model.RemoteConfig) error {
	switch normalizeRsyncType(storageType) {
	case "local":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create snapshot dir %q: %w", dir, err)
		}
		return nil
	case "ssh":
		client, err := e.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		_, stderr, err := runSSHCommand(ctx, client, "mkdir -p "+shellQuote(dir))
		if err != nil {
			return fmt.Errorf("create remote snapshot dir %q: %w (%s)", dir, err, strings.TrimSpace(stderr))
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func (e *RollingBackupExecutor) updateLatestLink(ctx context.Context, storageType, latestLinkPath, snapshotPath string, remote *model.RemoteConfig) error {
	switch normalizeRsyncType(storageType) {
	case "local":
		if info, err := os.Lstat(latestLinkPath); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				if removeErr := os.Remove(latestLinkPath); removeErr != nil {
					return fmt.Errorf("remove existing latest path %q: %w", latestLinkPath, removeErr)
				}
			} else if removeErr := os.Remove(latestLinkPath); removeErr != nil {
				return fmt.Errorf("remove existing latest symlink %q: %w", latestLinkPath, removeErr)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat latest link %q: %w", latestLinkPath, err)
		}
		if err := os.Symlink(snapshotPath, latestLinkPath); err != nil {
			return fmt.Errorf("create latest symlink %q -> %q: %w", latestLinkPath, snapshotPath, err)
		}
		return nil
	case "ssh":
		client, err := e.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		command := "ln -sfn " + shellQuote(snapshotPath) + " " + shellQuote(latestLinkPath)
		_, stderr, err := runSSHCommand(ctx, client, command)
		if err != nil {
			return fmt.Errorf("update remote latest link %q: %w (%s)", latestLinkPath, err, strings.TrimSpace(stderr))
		}
		return nil
	default:
		return fmt.Errorf("unsupported storage type %q", storageType)
	}
}

func (e *RollingBackupExecutor) connectSSH(ctx context.Context, remote *model.RemoteConfig) (*ssh.Client, error) {
	if remote == nil {
		return nil, fmt.Errorf("ssh remote config is required")
	}
	client, err := e.dialSSH(ctx, *remote)
	if err != nil {
		return nil, fmt.Errorf("connect ssh remote %d: %w", remote.ID, err)
	}

	return client, nil
}

func (e *RollingBackupExecutor) runRsync(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error) {
	return e.executeRsync(ctx, cfg, progressCb)
}

func (e *RollingBackupExecutor) reportProgress(task *model.Task, step string, progress ProgressInfo, externalCb func(ProgressInfo)) error {
	task.CurrentStep = step
	if err := e.updateTaskProgress(task, progress); err != nil {
		return err
	}
	if externalCb != nil {
		externalCb(progress)
	}

	return nil
}

func (e *RollingBackupExecutor) updateTaskStep(task *model.Task, step string, progress int, estimatedEnd *time.Time) error {
	if task == nil {
		return nil
	}
	task.CurrentStep = step
	task.Progress = clampProgress(progress)
	task.EstimatedEnd = estimatedEnd
	return e.db.UpdateTask(task)
}

func (e *RollingBackupExecutor) promoteRelaySnapshot(relayBase, relayStagePath, relayCurrentPath string) error {
	if err := os.RemoveAll(relayCurrentPath); err != nil {
		return fmt.Errorf("remove previous relay snapshot %q: %w", relayCurrentPath, err)
	}
	if err := os.Rename(relayStagePath, relayCurrentPath); err != nil {
		return fmt.Errorf("promote relay snapshot %q -> %q: %w", relayStagePath, relayCurrentPath, err)
	}

	entries, err := os.ReadDir(relayBase)
	if err != nil {
		return fmt.Errorf("list relay base %q: %w", relayBase, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == relayCurrentDirName {
			continue
		}
		if err := os.RemoveAll(filepath.Join(relayBase, name)); err != nil {
			return fmt.Errorf("cleanup relay entry %q: %w", name, err)
		}
	}

	return nil
}

func resolveRollingDataDir() string {
	dataDir := strings.TrimSpace(os.Getenv("RBS_DATA_DIR"))
	if dataDir == "" {
		dataDir = defaultRollingDataDir
	}
	if absPath, err := filepath.Abs(dataDir); err == nil {
		return absPath
	}
	return dataDir
}

func joinStoragePath(storageType, parts0, part1, part2 string) string {
	if normalizeRsyncType(storageType) == "ssh" || normalizeRsyncType(storageType) == "openlist" {
		return pathpkg.Join(strings.TrimSpace(parts0), part1, part2)
	}
	return filepath.Join(strings.TrimSpace(parts0), part1, part2)
}

func backupInstanceStorageKey(instance *model.Instance) string {
	if instance == nil || instance.ID <= 0 {
		return ""
	}
	return strconv.FormatInt(instance.ID, 10)
}

func snapshotPathBelongsToTarget(target *model.BackupTarget, instance *model.Instance, snapshotPath string) bool {
	storageKey := backupInstanceStorageKey(instance)
	if strings.TrimSpace(storageKey) == "" {
		return false
	}

	storageType := normalizeRsyncType(target.StorageType)
	if storageType == "ssh" {
		root := pathpkg.Clean(pathpkg.Join(strings.TrimSpace(target.StoragePath), storageKey))
		candidate := pathpkg.Clean(snapshotPath)
		return candidate == root || strings.HasPrefix(candidate, root+"/")
	}

	root := filepath.Clean(filepath.Join(strings.TrimSpace(target.StoragePath), storageKey))
	candidate := filepath.Clean(snapshotPath)
	separator := string(os.PathSeparator)
	return candidate == root || strings.HasPrefix(candidate, root+separator)
}

func elapsedSeconds(startedAt *time.Time, completedAt time.Time) int64 {
	if startedAt == nil {
		return 0
	}
	duration := completedAt.Sub(*startedAt)
	if duration < 0 {
		return 0
	}
	return int64(duration.Seconds())
}

func clampProgress(progress int) int {
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}

func scaleProgress(progress ProgressInfo, start, end int) ProgressInfo {
	rangeSize := end - start
	progress.Percentage = clampProgress(start + (clampProgress(progress.Percentage)*rangeSize)/100)
	return progress
}

func parseRemainingDuration(raw string) (time.Duration, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}

	parts := strings.Split(trimmed, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	values := make([]int, len(parts))
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return 0, false
		}
		values[index] = value
	}

	if len(values) == 2 {
		return time.Duration(values[0])*time.Minute + time.Duration(values[1])*time.Second, true
	}

	return time.Duration(values[0])*time.Hour + time.Duration(values[1])*time.Minute + time.Duration(values[2])*time.Second, true
}
