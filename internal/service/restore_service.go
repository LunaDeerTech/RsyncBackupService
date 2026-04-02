package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"gorm.io/gorm"
)

const (
	RestoreStatusRunning = "running"
	RestoreStatusSuccess = "success"
	RestoreStatusFailed  = "failed"
)

type RestoreRequest struct {
	InstanceID        uint
	BackupRecordID    uint
	RestoreTargetPath string
	Overwrite         bool
	VerifyToken       string
	TriggeredBy       uint
}

type ListRestoreRecordsRequest struct {
	Actor      AuthIdentity
	InstanceID *uint
}

type verifyTokenConsumer interface {
	ConsumeVerifyToken(ctx context.Context, userID uint, token string) error
}

type RestoreService struct {
	db                *gorm.DB
	config            config.Config
	instanceRepo      repository.InstanceRepository
	storageTargetRepo repository.StorageTargetRepository
	sshKeyRepo        repository.SSHKeyRepository
	verifyTokenConsumer verifyTokenConsumer
	notificationDispatcher notificationDispatcher
	runner            executorpkg.Runner
	clock             func() time.Time
}

func NewRestoreService(db *gorm.DB, cfg config.Config, runner executorpkg.Runner, verifyTokenConsumer verifyTokenConsumer, dispatchers ...notificationDispatcher) *RestoreService {
	if runner == nil {
		runner = executorpkg.NewExecRunner()
	}
	var dispatcher notificationDispatcher
	if len(dispatchers) > 0 {
		dispatcher = dispatchers[0]
	}

	return &RestoreService{
		db:                db,
		config:            cfg,
		instanceRepo:      repository.NewInstanceRepository(db),
		storageTargetRepo: repository.NewStorageTargetRepository(db),
		sshKeyRepo:        repository.NewSSHKeyRepository(db),
		verifyTokenConsumer: verifyTokenConsumer,
		notificationDispatcher: dispatcher,
		runner:            runner,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *RestoreService) Start(ctx context.Context, req RestoreRequest) (*model.RestoreRecord, error) {
	verifyToken := strings.TrimSpace(req.VerifyToken)
	if verifyToken == "" {
		return nil, ErrVerifyTokenRequired
	}
	if req.TriggeredBy == 0 {
		return nil, ErrUserRequired
	}
	if !HasValidatedVerifyToken(ctx, req.TriggeredBy, verifyToken) {
		if s.verifyTokenConsumer == nil {
			return nil, ErrVerifyTokenInvalid
		}
		if err := s.verifyTokenConsumer.ConsumeVerifyToken(ctx, req.TriggeredBy, verifyToken); err != nil {
			return nil, err
		}
	}

	restoreTargetPath, err := normalizeRestoreTargetPath(req.RestoreTargetPath)
	if err != nil {
		return nil, err
	}

	instance, err := s.loadInstance(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}
	backupRecord, err := s.loadBackupRecord(ctx, req.InstanceID, req.BackupRecordID)
	if err != nil {
		return nil, err
	}
	if backupRecord.Status != model.BackupStatusSuccess || strings.TrimSpace(backupRecord.SnapshotPath) == "" {
		return nil, ErrBackupRecordNotRestorable
	}

	restoreRecord := model.RestoreRecord{
		InstanceID:        req.InstanceID,
		BackupRecordID:    req.BackupRecordID,
		RestoreTargetPath: restoreTargetPath,
		Overwrite:         req.Overwrite,
		Status:            RestoreStatusRunning,
		StartedAt:         s.clock(),
		TriggeredBy:       req.TriggeredBy,
	}
	if err := s.db.WithContext(ctx).Create(&restoreRecord).Error; err != nil {
		return nil, fmt.Errorf("create restore record: %w", err)
	}

	executeErr := s.executeRestore(ctx, instance, backupRecord, restoreTargetPath, req.Overwrite)
	if executeErr != nil {
		if completeErr := s.completeRestoreRecord(ctx, restoreRecord.ID, RestoreStatusFailed, executeErr.Error()); completeErr != nil {
			return &restoreRecord, errors.Join(executeErr, completeErr)
		}
		s.dispatchRestoreNotification(instance, restoreRecord, RestoreStatusFailed, executeErr.Error())
		restoreRecord.Status = RestoreStatusFailed
		restoreRecord.ErrorMessage = executeErr.Error()
		finishedAt := s.clock()
		restoreRecord.FinishedAt = &finishedAt
		return &restoreRecord, executeErr
	}

	if err := s.completeRestoreRecord(ctx, restoreRecord.ID, RestoreStatusSuccess, ""); err != nil {
		return &restoreRecord, err
	}
	s.dispatchRestoreNotification(instance, restoreRecord, RestoreStatusSuccess, "")
	finishedAt := s.clock()
	restoreRecord.Status = RestoreStatusSuccess
	restoreRecord.FinishedAt = &finishedAt

	return &restoreRecord, nil
}

func (s *RestoreService) List(ctx context.Context, req ListRestoreRecordsRequest) ([]model.RestoreRecord, error) {
	if req.Actor.UserID == 0 && !req.Actor.IsAdmin {
		return nil, ErrUserRequired
	}

	query := s.db.WithContext(ctx).Model(&model.RestoreRecord{}).Order("started_at DESC").Order("id DESC")
	if req.InstanceID != nil {
		query = query.Where("instance_id = ?", *req.InstanceID)
	}
	if !req.Actor.IsAdmin {
		query = query.
			Joins("JOIN instance_permissions ON instance_permissions.instance_id = restore_records.instance_id").
			Where("instance_permissions.user_id = ?", req.Actor.UserID).
			Distinct("restore_records.*")
	}

	var records []model.RestoreRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list restore records: %w", err)
	}

	return records, nil
}

func (s *RestoreService) executeRestore(ctx context.Context, instance model.BackupInstance, backupRecord model.BackupRecord, restoreTargetPath string, overwrite bool) error {
	switch strings.TrimSpace(backupRecord.BackupType) {
	case BackupTypeRolling:
		return s.executeRollingRestore(ctx, instance, backupRecord, restoreTargetPath, overwrite)
	case BackupTypeCold:
		return s.executeColdRestore(ctx, instance, backupRecord, restoreTargetPath, overwrite)
	default:
		return ErrInvalidBackupType
	}
}

func (s *RestoreService) executeRollingRestore(ctx context.Context, instance model.BackupInstance, backupRecord model.BackupRecord, restoreTargetPath string, overwrite bool) error {
	target, err := s.loadStorageTarget(ctx, backupRecord.StorageTargetID)
	if err != nil {
		return err
	}

	request := executorpkg.RestoreSyncRequest{
		SourcePath:        strings.TrimSpace(backupRecord.SnapshotPath),
		SourceRemote:      isSSHStorageTargetType(target.Type),
		DestinationPath:   restoreTargetPath,
		DestinationRemote: instanceUsesRemoteSource(instance),
		Overwrite:         overwrite,
	}
	if request.SourceRemote {
		request.SourceHost = target.Host
		request.SourcePort = target.Port
		request.SourceUser = target.User
		request.SourceSSHKeyPath, err = s.lookupTargetSSHKeyPath(ctx, target)
		if err != nil {
			return err
		}
	}
	if request.DestinationRemote {
		request.DestinationHost = instance.SourceHost
		request.DestinationPort = instance.SourcePort
		request.DestinationUser = instance.SourceUser
		request.DestinationSSHKeyPath, err = s.lookupInstanceSSHKeyPath(ctx, instance)
		if err != nil {
			return err
		}
	}

	if request.SourceRemote && request.DestinationRemote {
		request.RelayDir, err = s.createRestoreWorkspace("relay")
		if err != nil {
			return err
		}
		defer os.RemoveAll(request.RelayDir)
	}

	return s.executeRestoreSync(ctx, request)
}

func (s *RestoreService) executeColdRestore(ctx context.Context, instance model.BackupInstance, backupRecord model.BackupRecord, restoreTargetPath string, overwrite bool) error {
	target, err := s.loadStorageTarget(ctx, backupRecord.StorageTargetID)
	if err != nil {
		return err
	}
	backend, err := buildStorageBackend(ctx, s.sshKeyRepo, target)
	if err != nil {
		return err
	}

	workspaceDir, err := s.createRestoreWorkspace("archive")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspaceDir)

	archiveRelativePath, ok := relativeTargetPath(backupRecord.SnapshotPath, target.BasePath)
	if !ok {
		return ErrBackupRecordNotRestorable
	}
	archiveObjectPaths, err := resolveColdArchiveObjectPaths(ctx, backend, archiveRelativePath, backupRecord.VolumeCount)
	if err != nil {
		return err
	}

	downloadDir := filepath.Join(workspaceDir, "download")
	extractDir := filepath.Join(workspaceDir, "extract")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return fmt.Errorf("create archive download directory: %w", err)
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return fmt.Errorf("create archive extract directory: %w", err)
	}

	if len(archiveObjectPaths) == 1 && archiveObjectPaths[0] == archiveRelativePath {
		localArchivePath := filepath.Join(downloadDir, "archive.tar.gz")
		if err := backend.Download(ctx, archiveObjectPaths[0], localArchivePath); err != nil {
			return fmt.Errorf("download archive: %w", err)
		}
		if err := s.runner.Run(ctx, executorpkg.BuildArchiveExtractCommand(localArchivePath, extractDir), nil); err != nil {
			return err
		}
	} else {
		localArchiveParts := make([]string, 0, len(archiveObjectPaths))
		for _, archivePart := range archiveObjectPaths {
			localArchivePart := filepath.Join(downloadDir, filepath.Base(archivePart))
			if err := backend.Download(ctx, archivePart, localArchivePart); err != nil {
				return fmt.Errorf("download archive part %q: %w", archivePart, err)
			}
			localArchiveParts = append(localArchiveParts, localArchivePart)
		}
		if err := s.runner.Run(ctx, executorpkg.BuildSplitArchiveExtractCommand(localArchiveParts, extractDir), nil); err != nil {
			return err
		}
	}

	destinationSSHKeyPath := ""
	if instanceUsesRemoteSource(instance) {
		destinationSSHKeyPath, err = s.lookupInstanceSSHKeyPath(ctx, instance)
		if err != nil {
			return err
		}
	}

	return s.executeRestoreSync(ctx, executorpkg.RestoreSyncRequest{
		SourcePath:            extractDir,
		SourceRemote:          false,
		DestinationPath:       restoreTargetPath,
		DestinationRemote:     instanceUsesRemoteSource(instance),
		DestinationHost:       instance.SourceHost,
		DestinationPort:       instance.SourcePort,
		DestinationUser:       instance.SourceUser,
		DestinationSSHKeyPath: destinationSSHKeyPath,
		Overwrite:             overwrite,
	})
}

func (s *RestoreService) executeRestoreSync(ctx context.Context, request executorpkg.RestoreSyncRequest) error {
	if request.DestinationRemote {
		if err := s.ensureRemoteRestoreTarget(ctx, request); err != nil {
			return err
		}
	} else {
		if err := ensureLocalRestoreTarget(request.DestinationPath, request.Overwrite); err != nil {
			return err
		}
	}
	if request.DestinationRemote && strings.TrimSpace(request.DestinationSSHKeyPath) == "" {
		return ErrSSHKeyRequired
	}

	commandSpecs, err := executorpkg.BuildRestoreCommandSpecs(request)
	if err != nil {
		return err
	}
	for _, commandSpec := range commandSpecs {
		if err := s.runner.Run(ctx, commandSpec, nil); err != nil {
			return err
		}
	}

	return nil
}

func (s *RestoreService) loadInstance(ctx context.Context, instanceID uint) (model.BackupInstance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupInstance{}, ErrInstanceNotFound
		}
		return model.BackupInstance{}, err
	}

	return instance, nil
}

func (s *RestoreService) loadBackupRecord(ctx context.Context, instanceID, backupRecordID uint) (model.BackupRecord, error) {
	var backupRecord model.BackupRecord
	if err := s.db.WithContext(ctx).Where("id = ? AND instance_id = ?", backupRecordID, instanceID).First(&backupRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BackupRecord{}, ErrBackupRecordNotFound
		}
		return model.BackupRecord{}, fmt.Errorf("load backup record: %w", err)
	}

	return backupRecord, nil
}

func (s *RestoreService) loadStorageTarget(ctx context.Context, storageTargetID uint) (model.StorageTarget, error) {
	target, err := s.storageTargetRepo.GetByID(ctx, storageTargetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.StorageTarget{}, ErrStorageTargetNotFound
		}
		return model.StorageTarget{}, err
	}

	return target, nil
}

func (s *RestoreService) lookupTargetSSHKeyPath(ctx context.Context, target model.StorageTarget) (string, error) {
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

func (s *RestoreService) lookupInstanceSSHKeyPath(ctx context.Context, instance model.BackupInstance) (string, error) {
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

func (s *RestoreService) completeRestoreRecord(ctx context.Context, recordID uint, status, errorMessage string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		finishedAt := s.clock()
		updates := map[string]any{
			"status":        status,
			"finished_at":   &finishedAt,
			"error_message": errorMessage,
		}
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.db.WithContext(updateCtx).Model(&model.RestoreRecord{}).Where("id = ?", recordID).Updates(updates).Error
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return fmt.Errorf("complete restore record: %w", lastErr)
}

func (s *RestoreService) dispatchRestoreNotification(instance model.BackupInstance, record model.RestoreRecord, status, errorMessage string) {
	if s.notificationDispatcher == nil {
		return
	}
	if err := s.notificationDispatcher.Notify(context.Background(), buildRestoreNotificationEvent(instance, record, status, errorMessage, s.clock())); err != nil {
		log.Printf("warning: dispatch restore notification: %v", err)
	}
}

func (s *RestoreService) createRestoreWorkspace(purpose string) (string, error) {
	workspaceRoot := strings.TrimSpace(s.config.DataDir)
	if workspaceRoot != "" {
		workspaceRoot = filepath.Join(workspaceRoot, "tmp")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			return "", fmt.Errorf("create restore temp root: %w", err)
		}
	}
	workspaceDir, err := os.MkdirTemp(workspaceRoot, "rbs-restore-"+purpose+"-*")
	if err != nil {
		return "", fmt.Errorf("create restore workspace: %w", err)
	}

	return workspaceDir, nil
}

func normalizeRestoreTargetPath(value string) (string, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return "", ErrRestoreTargetPathRequired
	}
	cleanValue := filepath.Clean(trimmedValue)
	if cleanValue == string(filepath.Separator) || !filepath.IsAbs(cleanValue) {
		return "", ErrInvalidRestoreTargetPath
	}

	return cleanValue, nil
}

func ensureLocalRestoreTarget(targetPath string, overwrite bool) error {
	status, err := inspectLocalRestoreTarget(targetPath)
	if err != nil {
		return err
	}

	switch status {
	case "symlink", "file":
		return ErrInvalidRestoreTargetPath
	case "nonempty":
		if !overwrite {
			return ErrRestoreTargetExists
		}
	case "missing":
		cleanTargetPath := filepath.Clean(strings.TrimSpace(targetPath))
		if err := os.MkdirAll(cleanTargetPath, 0o755); err != nil {
			return fmt.Errorf("create restore target directory: %w", err)
		}
	case "empty":
		return nil
	default:
		return fmt.Errorf("unexpected local restore target status %q", status)
	}

	return nil
}

func inspectLocalRestoreTarget(targetPath string) (string, error) {
	cleanTargetPath := filepath.Clean(strings.TrimSpace(targetPath))
	currentPath := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(cleanTargetPath, currentPath), string(filepath.Separator)) {
		if component == "" {
			continue
		}
		currentPath = filepath.Join(currentPath, component)
		info, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return "missing", nil
			}
			return "", fmt.Errorf("lstat restore target path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "symlink", nil
		}
		if !info.IsDir() {
			return "file", nil
		}
	}

	entries, err := os.ReadDir(cleanTargetPath)
	if err != nil {
		return "", fmt.Errorf("read restore target directory: %w", err)
	}
	if len(entries) > 0 {
		return "nonempty", nil
	}

	return "empty", nil
}

func (s *RestoreService) ensureRemoteRestoreTarget(ctx context.Context, request executorpkg.RestoreSyncRequest) error {
	status, err := s.inspectRemoteRestoreTarget(ctx, request)
	if err != nil {
		return err
	}

	switch status {
	case "symlink", "file":
		return ErrInvalidRestoreTargetPath
	case "nonempty":
		if !request.Overwrite {
			return ErrRestoreTargetExists
		}
	case "missing":
		if err := s.runner.Run(ctx, buildRemoteSSHCommandSpec(request.DestinationSSHKeyPath, request.DestinationPort, request.DestinationUser, request.DestinationHost, fmt.Sprintf("mkdir -p %s", serviceShellQuote(request.DestinationPath))), nil); err != nil {
			return err
		}
	case "empty":
		return nil
	default:
		return fmt.Errorf("unexpected remote restore target status %q", status)
	}

	return nil
}

func (s *RestoreService) inspectRemoteRestoreTarget(ctx context.Context, request executorpkg.RestoreSyncRequest) (string, error) {
	var outputLines []string
	command := buildRemoteRestoreTargetInspectCommand(request.DestinationPath)
	if err := s.runner.Run(ctx, buildRemoteSSHCommandSpec(request.DestinationSSHKeyPath, request.DestinationPort, request.DestinationUser, request.DestinationHost, command), func(line string) {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			outputLines = append(outputLines, trimmedLine)
		}
	}); err != nil {
		return "", err
	}
	if len(outputLines) == 0 {
		return "", fmt.Errorf("inspect remote restore target: missing status output")
	}

	return outputLines[len(outputLines)-1], nil
}

func buildRemoteRestoreTargetInspectCommand(targetPath string) string {
	return fmt.Sprintf("target=%s; current='/'; remaining=${target#/}; while [ -n \"$remaining\" ]; do segment=${remaining%%/*}; if [ \"$segment\" = \"$remaining\" ]; then remaining=''; else remaining=${remaining#*/}; fi; if [ -z \"$segment\" ]; then continue; fi; if [ \"$current\" = '/' ]; then current=\"/$segment\"; else current=\"$current/$segment\"; fi; if [ -L \"$current\" ]; then printf symlink; exit 0; fi; if [ -e \"$current\" ] && [ ! -d \"$current\" ]; then printf file; exit 0; fi; if [ ! -e \"$current\" ]; then printf missing; exit 0; fi; done; if [ \"$(find \"$target\" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)\" != \"\" ]; then printf nonempty; else printf empty; fi", serviceShellQuote(targetPath))
}

func buildRemoteSSHCommandSpec(privateKeyPath string, port int, user, host, command string) executorpkg.CommandSpec {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
	if strings.TrimSpace(privateKeyPath) != "" {
		args = append(args, "-i", strings.TrimSpace(privateKeyPath))
	}
	if port > 0 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, fmt.Sprintf("%s@%s", strings.TrimSpace(user), strings.TrimSpace(host)), command)

	return executorpkg.CommandSpec{Name: "ssh", Args: args}
}

func serviceShellQuote(value string) string {
	return "'" + strings.ReplaceAll(strings.TrimSpace(value), "'", "'\"'\"'") + "'"
}