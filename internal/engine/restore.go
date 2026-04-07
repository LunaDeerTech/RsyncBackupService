package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	backupcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const (
	restoreTaskPrepStep    = "preparing restore"
	restoreTaskFetchStep   = "fetching backup artifact"
	restoreTaskMergeStep   = "merging backup parts"
	restoreTaskDecryptStep = "decrypting backup"
	restoreTaskExtractStep = "extracting backup"
	restoreTaskSyncStep    = "syncing restored data"
	restoreTaskDoneStep    = "completed"
)

type RestoreRequest struct {
	RestoreType   string
	TargetPath    string
	EncryptionKey string
}

type RestoreExecutor struct {
	rsync *RsyncExecutor
	db    *store.DB

	dataDir      string
	now          func() time.Time
	dialSSH      func(context.Context, model.RemoteConfig) (*ssh.Client, error)
	executeRsync func(context.Context, RsyncConfig, func(ProgressInfo)) (*RsyncResult, error)
	newCommand   commandFactory
	decryptFile  func(context.Context, string, string, []byte) error
}

func NewRestoreExecutor(rsync *RsyncExecutor, db *store.DB, dataDir string) *RestoreExecutor {
	if rsync == nil {
		rsync = NewRsyncExecutor()
	}

	executor := &RestoreExecutor{
		rsync:      rsync,
		db:         db,
		dataDir:    strings.TrimSpace(dataDir),
		now:        func() time.Time { return time.Now().UTC() },
		dialSSH:    service.DialSSHClient,
		newCommand: exec.CommandContext,
		decryptFile: func(ctx context.Context, inputPath, outputPath string, key []byte) error {
			return backupcrypto.DecryptFileWithContext(ctx, inputPath, outputPath, key)
		},
	}
	executor.executeRsync = executor.rsync.Execute
	if executor.dataDir == "" {
		executor.dataDir = resolveRollingDataDir()
	}

	return executor
}

func (e *RestoreExecutor) Execute(ctx context.Context, task *model.Task, backup *model.Backup, restoreReq *RestoreRequest, progressCb func(ProgressInfo)) error {
	if err := e.validateExecutor(task, backup); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if restoreReq == nil {
		restoreReq = &RestoreRequest{}
	}
	if err := e.startRun(task); err != nil {
		return err
	}

	if err := e.validateRequest(restoreReq); err != nil {
		return e.finishRun(task, err)
	}
	if strings.TrimSpace(backup.SnapshotPath) == "" {
		return e.finishRun(task, fmt.Errorf("backup snapshot path is required"))
	}
	if backup.Status != "success" {
		return e.finishRun(task, fmt.Errorf("backup %d is not restorable from status %q", backup.ID, backup.Status))
	}

	policy, err := e.db.GetPolicyByID(backup.PolicyID)
	if err != nil {
		return e.finishRun(task, err)
	}
	instance, err := e.db.GetInstanceByID(backup.InstanceID)
	if err != nil {
		return e.finishRun(task, err)
	}
	target, err := e.db.GetBackupTargetByID(policy.TargetID)
	if err != nil {
		return e.finishRun(task, err)
	}

	restoreTargetPath, restoreTargetType, deleteExtraneous, targetRemote, err := e.resolveRestoreTarget(instance, restoreReq)
	if err != nil {
		return e.finishRun(task, err)
	}
	backupSourceRemote, err := e.loadRemoteConfig(target.StorageType, target.RemoteConfigID, "backup source")
	if err != nil {
		return e.finishRun(task, err)
	}

	var runErr error
	switch strings.ToLower(strings.TrimSpace(backup.Type)) {
	case "rolling":
		runErr = e.restoreRolling(ctx, task, backup, target, backupSourceRemote, restoreTargetPath, restoreTargetType, targetRemote, deleteExtraneous, progressCb)
	case "cold":
		runErr = e.restoreCold(ctx, task, backup, policy, target, backupSourceRemote, restoreReq, restoreTargetPath, restoreTargetType, targetRemote, deleteExtraneous, progressCb)
	default:
		runErr = fmt.Errorf("unsupported restore backup type %q", backup.Type)
	}
	if runErr != nil {
		return e.finishRun(task, runErr)
	}

	return e.completeRun(task)
}

func (e *RestoreExecutor) validateExecutor(task *model.Task, backup *model.Backup) error {
	if e == nil {
		return fmt.Errorf("restore executor is nil")
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
	if e.decryptFile == nil {
		e.decryptFile = func(ctx context.Context, inputPath, outputPath string, key []byte) error {
			return backupcrypto.DecryptFileWithContext(ctx, inputPath, outputPath, key)
		}
	}
	if strings.TrimSpace(e.dataDir) == "" {
		e.dataDir = resolveRollingDataDir()
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}
	if backup == nil {
		return fmt.Errorf("backup is nil")
	}

	return nil
}

func (e *RestoreExecutor) validateRequest(request *RestoreRequest) error {
	restoreType := strings.ToLower(strings.TrimSpace(request.RestoreType))
	switch restoreType {
	case "source":
		request.RestoreType = "source"
		request.TargetPath = ""
		return nil
	case "custom":
		request.RestoreType = "custom"
		request.TargetPath = strings.TrimSpace(request.TargetPath)
		if request.TargetPath == "" {
			return fmt.Errorf("target_path is required when restore_type is custom")
		}
		return nil
	default:
		return fmt.Errorf("restore_type must be source or custom")
	}
}

func (e *RestoreExecutor) startRun(task *model.Task) error {
	startedAt := e.now()
	task.Type = "restore"
	task.Status = "running"
	task.Progress = 0
	task.CurrentStep = restoreTaskPrepStep
	task.StartedAt = &startedAt
	task.CompletedAt = nil
	task.EstimatedEnd = nil
	task.ErrorMessage = ""
	if task.ID == 0 {
		return e.db.CreateTask(task)
	}
	return e.db.UpdateTask(task)
}

func (e *RestoreExecutor) finishRun(task *model.Task, runErr error) error {
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return e.cancelRun(task, runErr)
	}
	return e.failRun(task, runErr)
}

func (e *RestoreExecutor) failRun(task *model.Task, runErr error) error {
	completedAt := e.now()
	task.Status = "failed"
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = strings.TrimSpace(runErr.Error())
	if err := e.db.UpdateTask(task); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

func (e *RestoreExecutor) cancelRun(task *model.Task, runErr error) error {
	completedAt := e.now()
	task.Status = "cancelled"
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = strings.TrimSpace(runErr.Error())
	if err := e.db.UpdateTask(task); err != nil {
		return errors.Join(runErr, err)
	}
	return runErr
}

func (e *RestoreExecutor) completeRun(task *model.Task) error {
	completedAt := e.now()
	task.Status = "success"
	task.Progress = 100
	task.CurrentStep = restoreTaskDoneStep
	task.CompletedAt = &completedAt
	task.EstimatedEnd = nil
	task.ErrorMessage = ""
	return e.db.UpdateTask(task)
}

func (e *RestoreExecutor) updateTaskProgress(task *model.Task, progress ProgressInfo) error {
	if task == nil {
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

func (e *RestoreExecutor) reportProgress(task *model.Task, step string, progress ProgressInfo, externalCb func(ProgressInfo)) error {
	task.CurrentStep = step
	if err := e.updateTaskProgress(task, progress); err != nil {
		return err
	}
	if externalCb != nil {
		externalCb(progress)
	}
	return nil
}

func (e *RestoreExecutor) updateTaskStep(task *model.Task, step string, progress int, estimatedEnd *time.Time) error {
	if task == nil {
		return nil
	}
	task.CurrentStep = step
	task.Progress = clampProgress(progress)
	task.EstimatedEnd = estimatedEnd
	return e.db.UpdateTask(task)
}

func (e *RestoreExecutor) loadRemoteConfig(endpointType string, remoteConfigID *int64, role string) (*model.RemoteConfig, error) {
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

func (e *RestoreExecutor) resolveRestoreTarget(instance *model.Instance, request *RestoreRequest) (string, string, bool, *model.RemoteConfig, error) {
	if instance == nil {
		return "", "", false, nil, fmt.Errorf("instance is nil")
	}

	switch request.RestoreType {
	case "source":
		remote, err := e.loadRemoteConfig(instance.SourceType, instance.RemoteConfigID, "restore target")
		if err != nil {
			return "", "", false, nil, err
		}
		return instance.SourcePath, instance.SourceType, true, remote, nil
	case "custom":
		return request.TargetPath, "local", false, nil, nil
	default:
		return "", "", false, nil, fmt.Errorf("restore_type must be source or custom")
	}
}

func (e *RestoreExecutor) restoreRolling(ctx context.Context, task *model.Task, backup *model.Backup, target *model.BackupTarget, backupSourceRemote *model.RemoteConfig, restoreTargetPath, restoreTargetType string, restoreTargetRemote *model.RemoteConfig, deleteExtraneous bool, progressCb func(ProgressInfo)) error {
	if err := e.updateTaskStep(task, restoreTaskSyncStep, 0, nil); err != nil {
		return err
	}

	if normalizeRsyncType(target.StorageType) == "ssh" && normalizeRsyncType(restoreTargetType) == "ssh" {
		return e.restoreRollingViaRelay(ctx, task, backup, target, backupSourceRemote, restoreTargetPath, restoreTargetType, restoreTargetRemote, deleteExtraneous, progressCb)
	}

	var progressErr error
	_, err := e.executeRsync(ctx, RsyncConfig{
		SourcePath:    backup.SnapshotPath,
		SourceType:    target.StorageType,
		SourceRemote:  backupSourceRemote,
		DestPath:      restoreTargetPath,
		DestType:      restoreTargetType,
		DestRemote:    restoreTargetRemote,
		DisableDelete: !deleteExtraneous,
	}, func(progress ProgressInfo) {
		if updateErr := e.reportProgress(task, restoreTaskSyncStep, progress, progressCb); updateErr != nil {
			progressErr = errors.Join(progressErr, updateErr)
		}
	})
	if err != nil {
		return err
	}
	if progressErr != nil {
		return progressErr
	}

	return nil
}

func (e *RestoreExecutor) restoreRollingViaRelay(ctx context.Context, task *model.Task, backup *model.Backup, target *model.BackupTarget, backupSourceRemote *model.RemoteConfig, restoreTargetPath, restoreTargetType string, restoreTargetRemote *model.RemoteConfig, deleteExtraneous bool, progressCb func(ProgressInfo)) error {
	relayDir := filepath.Join(e.dataDir, "temp", fmt.Sprintf("restore-%d", task.ID), "rolling-relay")
	if err := os.RemoveAll(relayDir); err != nil {
		return fmt.Errorf("reset rolling relay directory %q: %w", relayDir, err)
	}
	if err := os.MkdirAll(relayDir, 0o755); err != nil {
		return fmt.Errorf("create rolling relay directory %q: %w", relayDir, err)
	}

	var pullErr error
	_, err := e.executeRsync(ctx, RsyncConfig{
		SourcePath:    backup.SnapshotPath,
		SourceType:    target.StorageType,
		SourceRemote:  backupSourceRemote,
		DestPath:      relayDir,
		DestType:      "local",
		DisableDelete: true,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 0, 50)
		if updateErr := e.reportProgress(task, restoreTaskSyncStep, mapped, progressCb); updateErr != nil {
			pullErr = errors.Join(pullErr, updateErr)
		}
	})
	if err != nil {
		return err
	}
	if pullErr != nil {
		return pullErr
	}

	var pushErr error
	_, err = e.executeRsync(ctx, RsyncConfig{
		SourcePath:    relayDir,
		SourceType:    "local",
		DestPath:      restoreTargetPath,
		DestType:      restoreTargetType,
		DestRemote:    restoreTargetRemote,
		DisableDelete: !deleteExtraneous,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 50, 100)
		if updateErr := e.reportProgress(task, restoreTaskSyncStep, mapped, progressCb); updateErr != nil {
			pushErr = errors.Join(pushErr, updateErr)
		}
	})
	if err != nil {
		return err
	}
	if pushErr != nil {
		return pushErr
	}

	return nil
}

func (e *RestoreExecutor) restoreCold(ctx context.Context, task *model.Task, backup *model.Backup, policy *model.Policy, target *model.BackupTarget, backupSourceRemote *model.RemoteConfig, restoreReq *RestoreRequest, restoreTargetPath, restoreTargetType string, restoreTargetRemote *model.RemoteConfig, deleteExtraneous bool, progressCb func(ProgressInfo)) error {
	tempRoot := filepath.Join(e.dataDir, "temp", fmt.Sprintf("restore-%d", task.ID))
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return fmt.Errorf("create restore temp root %q: %w", tempRoot, err)
	}
	defer func() {
		_ = os.RemoveAll(tempRoot)
	}()

	stagedPath, err := e.stageColdArtifact(ctx, task, target, backupSourceRemote, backup.SnapshotPath, filepath.Join(tempRoot, "artifact"), progressCb)
	if err != nil {
		return err
	}

	restoreSourcePath, err := e.prepareColdRestoreSource(ctx, task, policy, restoreReq, stagedPath)
	if err != nil {
		return err
	}

	if err := e.updateTaskStep(task, restoreTaskSyncStep, 70, nil); err != nil {
		return err
	}
	var progressErr error
	_, err = e.executeRsync(ctx, RsyncConfig{
		SourcePath:    restoreSourcePath,
		SourceType:    "local",
		DestPath:      restoreTargetPath,
		DestType:      restoreTargetType,
		DestRemote:    restoreTargetRemote,
		DisableDelete: !deleteExtraneous,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 70, 100)
		if updateErr := e.reportProgress(task, restoreTaskSyncStep, mapped, progressCb); updateErr != nil {
			progressErr = errors.Join(progressErr, updateErr)
		}
	})
	if err != nil {
		return err
	}
	if progressErr != nil {
		return progressErr
	}

	return nil
}

func (e *RestoreExecutor) stageColdArtifact(ctx context.Context, task *model.Task, target *model.BackupTarget, remote *model.RemoteConfig, snapshotPath, stageRoot string, progressCb func(ProgressInfo)) (string, error) {
	if err := e.updateTaskStep(task, restoreTaskFetchStep, 0, nil); err != nil {
		return "", err
	}
	if err := os.RemoveAll(stageRoot); err != nil {
		return "", fmt.Errorf("reset staged restore artifact %q: %w", stageRoot, err)
	}
	if err := os.MkdirAll(stageRoot, 0o755); err != nil {
		return "", fmt.Errorf("create staged restore artifact %q: %w", stageRoot, err)
	}

	storageType := normalizeRsyncType(target.StorageType)
	if storageType == "local" {
		stagedPath, err := stageLocalColdArtifact(snapshotPath, stageRoot)
		if err != nil {
			return "", err
		}
		if err := e.reportProgress(task, restoreTaskFetchStep, ProgressInfo{Percentage: 100}, progressCb); err != nil {
			return "", err
		}
		return stagedPath, nil
	}

	if storageType != "ssh" {
		return "", fmt.Errorf("unsupported cold backup storage type %q", target.StorageType)
	}

	remotePath, stagedPath, extraArgs, err := buildRemoteColdStagePlan(snapshotPath)
	if err != nil {
		return "", err
	}
	var progressErr error
	_, err = e.executeRsync(ctx, RsyncConfig{
		SourcePath:    remotePath,
		SourceType:    "ssh",
		SourceRemote:  remote,
		DestPath:      stageRoot,
		DestType:      "local",
		DisableDelete: true,
		ExtraArgs:     extraArgs,
	}, func(progress ProgressInfo) {
		mapped := scaleProgress(progress, 0, 100)
		if updateErr := e.reportProgress(task, restoreTaskFetchStep, mapped, progressCb); updateErr != nil {
			progressErr = errors.Join(progressErr, updateErr)
		}
	})
	if err != nil {
		return "", err
	}
	if progressErr != nil {
		return "", progressErr
	}

	return filepath.Join(stageRoot, stagedPath), nil
}

func stageLocalColdArtifact(snapshotPath, stageRoot string) (string, error) {
	if basePath, ok := splitPartBasePath(snapshotPath, "local"); ok {
		matches, err := filepath.Glob(basePath + ".part*")
		if err != nil {
			return "", fmt.Errorf("glob split backup parts for %q: %w", snapshotPath, err)
		}
		if len(matches) == 0 {
			return "", fmt.Errorf("split backup parts for %q not found", snapshotPath)
		}
		sort.Strings(matches)
		for _, sourcePath := range matches {
			info, err := os.Stat(sourcePath)
			if err != nil {
				return "", fmt.Errorf("stat split backup part %q: %w", sourcePath, err)
			}
			if err := copyFilePath(sourcePath, filepath.Join(stageRoot, filepath.Base(sourcePath)), info.Mode()); err != nil {
				return "", fmt.Errorf("copy split backup part %q: %w", sourcePath, err)
			}
		}
		return filepath.Join(stageRoot, filepath.Base(snapshotPath)), nil
	}

	info, err := os.Stat(snapshotPath)
	if err != nil {
		return "", fmt.Errorf("stat cold backup snapshot %q: %w", snapshotPath, err)
	}
	destPath := filepath.Join(stageRoot, filepath.Base(snapshotPath))
	if info.IsDir() {
		if err := copyDir(snapshotPath, destPath); err != nil {
			return "", fmt.Errorf("copy cold backup directory %q: %w", snapshotPath, err)
		}
		return destPath, nil
	}
	if err := copyFilePath(snapshotPath, destPath, info.Mode()); err != nil {
		return "", fmt.Errorf("copy cold backup file %q: %w", snapshotPath, err)
	}
	return destPath, nil
}

func buildRemoteColdStagePlan(snapshotPath string) (string, string, []string, error) {
	trimmed := strings.TrimSpace(snapshotPath)
	if trimmed == "" {
		return "", "", nil, fmt.Errorf("backup snapshot path is required")
	}
	if basePath, ok := splitPartBasePath(trimmed, "ssh"); ok {
		baseName := pathpkg.Base(basePath)
		return pathpkg.Dir(trimmed), filepath.Base(trimmed), []string{
			"--include=/" + baseName + ".part*",
			"--exclude=*",
		}, nil
	}
	entryName := pathpkg.Base(trimmed)
	return pathpkg.Dir(trimmed), entryName, []string{
		"--include=/" + entryName,
		"--include=/" + entryName + "/**",
		"--exclude=*",
	}, nil
}

func (e *RestoreExecutor) prepareColdRestoreSource(ctx context.Context, task *model.Task, policy *model.Policy, restoreReq *RestoreRequest, stagedPath string) (string, error) {
	info, err := os.Stat(stagedPath)
	if err != nil {
		return "", fmt.Errorf("stat staged restore artifact %q: %w", stagedPath, err)
	}
	if info.IsDir() {
		return stagedPath, nil
	}

	currentPath := stagedPath
	if basePath, ok := splitPartBasePath(stagedPath, "local"); ok {
		if err := e.updateTaskStep(task, restoreTaskMergeStep, 20, nil); err != nil {
			return "", err
		}
		mergedPath := basePath
		if err := mergeSplitFiles(basePath, mergedPath); err != nil {
			return "", err
		}
		currentPath = mergedPath
	}

	if policy.Encryption {
		if err := e.updateTaskStep(task, restoreTaskDecryptStep, 35, nil); err != nil {
			return "", err
		}
		decryptionKey, err := e.resolveEncryptionKey(policy, restoreReq)
		if err != nil {
			return "", err
		}
		decryptedPath := strings.TrimSuffix(currentPath, ".enc")
		if decryptedPath == currentPath {
			decryptedPath = currentPath + ".dec"
		}
		if err := e.decryptFile(ctx, currentPath, decryptedPath, decryptionKey); err != nil {
			return "", err
		}
		currentPath = decryptedPath
	}

	if err := e.updateTaskStep(task, restoreTaskExtractStep, 50, nil); err != nil {
		return "", err
	}
	extractDir := filepath.Join(filepath.Dir(currentPath), "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", fmt.Errorf("create extracted restore directory %q: %w", extractDir, err)
	}
	if err := e.extractArchive(ctx, currentPath, extractDir); err != nil {
		return "", err
	}

	dataPath := filepath.Join(extractDir, "data")
	if info, err := os.Stat(dataPath); err == nil && info.IsDir() {
		return dataPath, nil
	}
	return extractDir, nil
}

func mergeSplitFiles(basePath, outputPath string) error {
	parts, err := filepath.Glob(basePath + ".part*")
	if err != nil {
		return fmt.Errorf("glob split restore parts for %q: %w", basePath, err)
	}
	if len(parts) == 0 {
		return fmt.Errorf("split restore parts for %q not found", basePath)
	}
	sort.Strings(parts)

	outputFile, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create merged restore file %q: %w", outputPath, err)
	}
	defer outputFile.Close()

	for _, partPath := range parts {
		inputFile, err := os.Open(partPath)
		if err != nil {
			return fmt.Errorf("open split restore part %q: %w", partPath, err)
		}
		if _, err := io.Copy(outputFile, inputFile); err != nil {
			inputFile.Close()
			return fmt.Errorf("append split restore part %q: %w", partPath, err)
		}
		if err := inputFile.Close(); err != nil {
			return fmt.Errorf("close split restore part %q: %w", partPath, err)
		}
	}

	return nil
}

func (e *RestoreExecutor) resolveEncryptionKey(policy *model.Policy, restoreReq *RestoreRequest) ([]byte, error) {
	if policy == nil || !policy.Encryption {
		return nil, nil
	}
	key := strings.TrimSpace(restoreReq.EncryptionKey)
	if key == "" {
		return nil, fmt.Errorf("encryption key is required for encrypted cold backup restore")
	}
	if policy.EncryptionKeyHash != nil && *policy.EncryptionKeyHash != "" && !backupcrypto.ValidateEncryptionKey(key, *policy.EncryptionKeyHash) {
		return nil, fmt.Errorf("encryption key does not match policy")
	}
	return []byte(key), nil
}

func (e *RestoreExecutor) extractArchive(ctx context.Context, archivePath, destDir string) error {
	args := []string{"-xf", archivePath, "-C", destDir}
	if strings.HasSuffix(archivePath, ".gz") {
		args[0] = "-xzf"
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