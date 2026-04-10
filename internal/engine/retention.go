package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const defaultRetentionCleanupInterval = 6 * time.Hour

var coldSplitPartPattern = regexp.MustCompile(`\.part\d+$`)

type RetentionCleaner struct {
	db               *store.DB
	dataDir          string
	scheduleInterval time.Duration
	now              func() time.Time
	dialSSH          func(context.Context, model.RemoteConfig) (*ssh.Client, error)
	removeAll        func(string) error
	readDir          func(string) ([]os.DirEntry, error)
	readlink         func(string) (string, error)
	symlink          func(string, string) error
	glob             func(string) ([]string, error)
}

func NewRetentionCleaner(db *store.DB, dataDir string) *RetentionCleaner {
	cleaner := &RetentionCleaner{
		db:               db,
		dataDir:          strings.TrimSpace(dataDir),
		scheduleInterval: defaultRetentionCleanupInterval,
		now:              func() time.Time { return time.Now().UTC() },
		dialSSH:          service.DialSSHClient,
		removeAll:        os.RemoveAll,
		readDir:          os.ReadDir,
		readlink:         os.Readlink,
		symlink:          os.Symlink,
		glob:             filepath.Glob,
	}
	if cleaner.dataDir == "" {
		cleaner.dataDir = resolveRollingDataDir()
	}
	return cleaner
}

func (rc *RetentionCleaner) CleanByPolicy(ctx context.Context, policy *model.Policy) error {
	if rc == nil {
		return fmt.Errorf("retention cleaner is nil")
	}
	if rc.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	switch strings.ToLower(strings.TrimSpace(policy.RetentionType)) {
	case "time":
		return rc.cleanByTime(ctx, policy)
	case "count":
		return rc.cleanByCount(ctx, policy)
	case "", "none":
		return nil
	default:
		return fmt.Errorf("unsupported retention type %q for policy %d", policy.RetentionType, policy.ID)
	}
}

func (rc *RetentionCleaner) CleanAll(ctx context.Context) error {
	if rc == nil || rc.db == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	instances, err := rc.db.ListInstances()
	if err != nil {
		return err
	}

	var joined error
	for index := range instances {
		policies, err := rc.db.ListPoliciesByInstance(instances[index].ID)
		if err != nil {
			joined = errors.Join(joined, err)
			slog.Error("retention cleanup list policies failed", "instance_id", instances[index].ID, "error", err)
			continue
		}

		for policyIndex := range policies {
			if err := ctx.Err(); err != nil {
				return errors.Join(joined, err)
			}
			if err := rc.CleanByPolicy(ctx, &policies[policyIndex]); err != nil {
				joined = errors.Join(joined, err)
				slog.Error("retention cleanup by policy failed", "policy_id", policies[policyIndex].ID, "error", err)
			}
		}
	}

	return joined
}

func (rc *RetentionCleaner) StartSchedule(ctx context.Context) {
	if rc == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		if err := rc.CleanAll(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("retention scheduled cleanup failed", "error", err)
		}

		ticker := time.NewTicker(rc.scheduleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := rc.CleanAll(ctx); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("retention scheduled cleanup failed", "error", err)
				}
			}
		}
	}()
}

func (rc *RetentionCleaner) cleanByTime(ctx context.Context, policy *model.Policy) error {
	days := policy.RetentionValue
	if days < 0 {
		days = 0
	}
	before := rc.now().UTC().AddDate(0, 0, -days)
	backups, err := rc.db.ListExpiredBackups(policy.ID, before)
	if err != nil {
		return err
	}
	return rc.cleanBackups(ctx, policy, backups)
}

func (rc *RetentionCleaner) cleanByCount(ctx context.Context, policy *model.Policy) error {
	keepCount := policy.RetentionValue
	if keepCount < 0 {
		keepCount = 0
	}
	backups, err := rc.db.ListExcessBackups(policy.ID, keepCount)
	if err != nil {
		return err
	}
	return rc.cleanBackups(ctx, policy, backups)
}

func (rc *RetentionCleaner) cleanBackups(ctx context.Context, policy *model.Policy, backups []model.Backup) error {
	if len(backups) == 0 {
		return nil
	}

	instance, err := rc.db.GetInstanceByID(policy.InstanceID)
	if err != nil {
		return err
	}
	target, err := rc.db.GetBackupTargetByID(policy.TargetID)
	if err != nil {
		return err
	}
	remote, err := rc.loadTargetRemote(target)
	if err != nil {
		return err
	}

	var joined error
	for index := range backups {
		if err := ctx.Err(); err != nil {
			return errors.Join(joined, err)
		}
		if err := rc.deleteBackup(ctx, policy, instance, target, remote, &backups[index]); err != nil {
			joined = errors.Join(joined, err)
		}
	}

	return joined
}

func (rc *RetentionCleaner) deleteBackup(ctx context.Context, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, remote *model.RemoteConfig, backup *model.Backup) error {
	if backup == nil {
		return nil
	}

	slog.Info("retention cleanup deleting backup",
		"backup_id", backup.ID,
		"policy_id", policy.ID,
		"instance_id", instance.ID,
		"type", backup.Type,
		"path", backup.SnapshotPath,
	)

	if err := rc.deleteBackupArtifacts(ctx, target, remote, backup); err != nil {
		slog.Error("retention cleanup delete artifacts failed",
			"backup_id", backup.ID,
			"policy_id", policy.ID,
			"error", err,
		)
		rc.writeAuditLog(instance.ID, "backup.cleanup_failed", fmt.Sprintf("backup_id=%d policy_id=%d path=%s error=%s", backup.ID, policy.ID, backup.SnapshotPath, strings.TrimSpace(err.Error())))
		return err
	}

	var joined error
	refreshLatest, err := rc.shouldRefreshLatestLink(ctx, policy, instance, target, remote, backup)
	if err != nil {
		joined = errors.Join(joined, err)
		slog.Error("retention cleanup inspect latest link failed", "backup_id", backup.ID, "policy_id", policy.ID, "error", err)
	}
	if refreshLatest {
		replacementPath := ""
		replacement, replacementErr := rc.db.GetLatestSuccessfulBackupExcluding(instance.ID, policy.ID, backup.ID)
		if replacementErr != nil && !errors.Is(replacementErr, sql.ErrNoRows) {
			joined = errors.Join(joined, replacementErr)
			slog.Error("retention cleanup load replacement latest failed", "backup_id", backup.ID, "policy_id", policy.ID, "error", replacementErr)
		} else if replacement != nil {
			replacementPath = strings.TrimSpace(replacement.SnapshotPath)
		}

		if updateErr := rc.setLatestLink(ctx, instance, target, remote, replacementPath); updateErr != nil {
			joined = errors.Join(joined, updateErr)
			slog.Error("retention cleanup update latest link failed",
				"backup_id", backup.ID,
				"policy_id", policy.ID,
				"error", updateErr,
			)
		}
	}

	if deleteErr := rc.deleteBackupRows(ctx, backup.ID); deleteErr != nil {
		slog.Error("retention cleanup delete database rows failed",
			"backup_id", backup.ID,
			"policy_id", policy.ID,
			"error", deleteErr,
		)
		return errors.Join(joined, deleteErr)
	}

	return joined
}

func (rc *RetentionCleaner) loadTargetRemote(target *model.BackupTarget) (*model.RemoteConfig, error) {
	if target == nil {
		return nil, fmt.Errorf("backup target is nil")
	}
	storageType := strings.ToLower(strings.TrimSpace(target.StorageType))
	if storageType != "ssh" && storageType != "openlist" {
		return nil, nil
	}
	if target.RemoteConfigID == nil {
		return nil, fmt.Errorf("%s target %d is missing remote config", storageType, target.ID)
	}

	remote, err := rc.db.GetRemoteConfigByID(*target.RemoteConfigID)
	if err != nil {
		return nil, err
	}
	if storageType == "ssh" && remote.Type != "ssh" {
		return nil, fmt.Errorf("target remote config %d must be ssh", remote.ID)
	}
	if storageType == "openlist" && !openlist.IsRemoteConfig(*remote) {
		return nil, fmt.Errorf("target remote config %d must be openlist", remote.ID)
	}

	return remote, nil
}

func (rc *RetentionCleaner) deleteBackupArtifacts(ctx context.Context, target *model.BackupTarget, remote *model.RemoteConfig, backup *model.Backup) error {
	storageType := strings.ToLower(strings.TrimSpace(target.StorageType))
	backupType := strings.ToLower(strings.TrimSpace(backup.Type))
	if backupType == "" {
		backupType = strings.ToLower(strings.TrimSpace(target.BackupType))
	}

	switch backupType {
	case "rolling":
		return rc.deleteRollingArtifacts(ctx, storageType, remote, backup.SnapshotPath)
	case "cold":
		return rc.deleteColdArtifacts(ctx, storageType, remote, backup.SnapshotPath)
	default:
		return fmt.Errorf("unsupported backup type %q", backup.Type)
	}
}

func (rc *RetentionCleaner) deleteRollingArtifacts(ctx context.Context, storageType string, remote *model.RemoteConfig, snapshotPath string) error {
	snapshotPath = strings.TrimSpace(snapshotPath)
	if snapshotPath == "" {
		return fmt.Errorf("snapshot path is required")
	}

	switch storageType {
	case "local":
		if err := rc.removeAll(snapshotPath); err != nil {
			return fmt.Errorf("remove rolling snapshot %q: %w", snapshotPath, err)
		}
		rc.removeEmptyLocalDir(filepath.Dir(snapshotPath))
		return nil
	case "ssh":
		client, err := rc.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		if _, stderr, err := runSSHCommand(ctx, client, "rm -rf "+shellQuote(snapshotPath)); err != nil {
			return fmt.Errorf("remove remote rolling snapshot %q: %w (%s)", snapshotPath, err, strings.TrimSpace(stderr))
		}
		_, _, _ = runSSHCommand(ctx, client, "rmdir "+shellQuote(pathpkg.Dir(snapshotPath))+" >/dev/null 2>&1 || true")
		return nil
	case "openlist":
		session, err := rc.openListSession(ctx, remote)
		if err != nil {
			return err
		}
		if err := session.RemovePath(ctx, snapshotPath); err != nil && !errors.Is(err, openlist.ErrNotFound) {
			return fmt.Errorf("remove openlist cold artifact %q: %w", snapshotPath, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported target storage type %q", storageType)
	}
}

func (rc *RetentionCleaner) deleteColdArtifacts(ctx context.Context, storageType string, remote *model.RemoteConfig, snapshotPath string) error {
	snapshotPath = strings.TrimSpace(snapshotPath)
	if snapshotPath == "" {
		return fmt.Errorf("snapshot path is required")
	}

	if basePath, ok := splitPartBasePath(snapshotPath, storageType); ok {
		return rc.deleteSplitColdArtifacts(ctx, storageType, remote, snapshotPath, basePath)
	}

	switch storageType {
	case "local":
		if err := rc.removeAll(snapshotPath); err != nil {
			return fmt.Errorf("remove cold artifact %q: %w", snapshotPath, err)
		}
		rc.removeEmptyLocalDir(filepath.Dir(snapshotPath))
		return nil
	case "ssh":
		client, err := rc.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		if _, stderr, err := runSSHCommand(ctx, client, "rm -rf "+shellQuote(snapshotPath)); err != nil {
			return fmt.Errorf("remove remote cold artifact %q: %w (%s)", snapshotPath, err, strings.TrimSpace(stderr))
		}
		_, _, _ = runSSHCommand(ctx, client, "rmdir "+shellQuote(pathpkg.Dir(snapshotPath))+" >/dev/null 2>&1 || true")
		return nil
	default:
		return fmt.Errorf("unsupported target storage type %q", storageType)
	}
}

func (rc *RetentionCleaner) deleteSplitColdArtifacts(ctx context.Context, storageType string, remote *model.RemoteConfig, snapshotPath, basePath string) error {
	switch storageType {
	case "local":
		matches, err := rc.glob(basePath + ".part*")
		if err != nil {
			return fmt.Errorf("glob split cold artifacts %q: %w", basePath+".part*", err)
		}
		for _, match := range matches {
			if err := rc.removeAll(match); err != nil {
				return fmt.Errorf("remove split cold artifact %q: %w", match, err)
			}
		}
		if len(matches) == 0 {
			if err := rc.removeAll(snapshotPath); err != nil {
				return fmt.Errorf("remove split cold artifact %q: %w", snapshotPath, err)
			}
		}
		rc.removeEmptyLocalDir(filepath.Dir(snapshotPath))
		return nil
	case "ssh":
		client, err := rc.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		fileName := pathpkg.Base(basePath) + ".part*"
		command := "find " + shellQuote(pathpkg.Dir(snapshotPath)) + " -maxdepth 1 -type f -name " + shellQuote(fileName) + " -exec rm -f {} +"
		if _, stderr, err := runSSHCommand(ctx, client, command); err != nil {
			return fmt.Errorf("remove remote split cold artifacts %q: %w (%s)", snapshotPath, err, strings.TrimSpace(stderr))
		}
		_, _, _ = runSSHCommand(ctx, client, "rm -f "+shellQuote(snapshotPath))
		_, _, _ = runSSHCommand(ctx, client, "rmdir "+shellQuote(pathpkg.Dir(snapshotPath))+" >/dev/null 2>&1 || true")
		return nil
	case "openlist":
		session, err := rc.openListSession(ctx, remote)
		if err != nil {
			return err
		}
		for partIndex := 1; ; partIndex++ {
			partPath := fmt.Sprintf("%s.part%03d", basePath, partIndex)
			if err := session.RemovePath(ctx, partPath); err != nil {
				if errors.Is(err, openlist.ErrNotFound) {
					if partIndex == 1 {
						break
					}
					break
				}
				return fmt.Errorf("remove openlist split cold artifact %q: %w", partPath, err)
			}
		}
		_ = session.RemovePath(ctx, snapshotPath)
		return nil
	default:
		return fmt.Errorf("unsupported target storage type %q", storageType)
	}
}

func (rc *RetentionCleaner) shouldRefreshLatestLink(ctx context.Context, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, remote *model.RemoteConfig, backup *model.Backup) (bool, error) {
	if strings.ToLower(strings.TrimSpace(policy.Type)) != "rolling" {
		return false, nil
	}

	latestBackup, err := rc.db.GetLatestSuccessfulBackup(instance.ID, policy.ID)
	if err == nil && latestBackup != nil && latestBackup.ID == backup.ID {
		return true, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	latestLinkPath := joinStoragePath(strings.ToLower(strings.TrimSpace(target.StorageType)), target.StoragePath, backupInstanceStorageKey(instance), "latest")
	switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
	case "local":
		currentTarget, err := rc.readlink(latestLinkPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "invalid argument") {
				return false, nil
			}
			return false, err
		}
		return filepath.Clean(currentTarget) == filepath.Clean(strings.TrimSpace(backup.SnapshotPath)), nil
	case "ssh":
		client, err := rc.connectSSH(ctx, remote)
		if err != nil {
			return false, err
		}
		defer client.Close()

		stdout, stderr, err := runSSHCommand(ctx, client, "readlink "+shellQuote(latestLinkPath)+" || true")
		if err != nil {
			return false, fmt.Errorf("read remote latest link %q: %w (%s)", latestLinkPath, err, strings.TrimSpace(stderr))
		}
		return strings.TrimSpace(stdout) == strings.TrimSpace(backup.SnapshotPath), nil
	default:
		return false, nil
	}
}

func (rc *RetentionCleaner) setLatestLink(ctx context.Context, instance *model.Instance, target *model.BackupTarget, remote *model.RemoteConfig, replacementPath string) error {
	latestLinkPath := joinStoragePath(strings.ToLower(strings.TrimSpace(target.StorageType)), target.StoragePath, backupInstanceStorageKey(instance), "latest")
	replacementPath = strings.TrimSpace(replacementPath)

	switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
	case "local":
		if err := rc.removeAll(latestLinkPath); err != nil {
			return fmt.Errorf("remove local latest link %q: %w", latestLinkPath, err)
		}
		if replacementPath == "" {
			return nil
		}
		if err := rc.symlink(replacementPath, latestLinkPath); err != nil {
			return fmt.Errorf("create local latest link %q -> %q: %w", latestLinkPath, replacementPath, err)
		}
		return nil
	case "ssh":
		client, err := rc.connectSSH(ctx, remote)
		if err != nil {
			return err
		}
		defer client.Close()

		if _, stderr, err := runSSHCommand(ctx, client, "rm -f "+shellQuote(latestLinkPath)); err != nil {
			return fmt.Errorf("remove remote latest link %q: %w (%s)", latestLinkPath, err, strings.TrimSpace(stderr))
		}
		if replacementPath == "" {
			return nil
		}
		command := "ln -sfn " + shellQuote(replacementPath) + " " + shellQuote(latestLinkPath)
		if _, stderr, err := runSSHCommand(ctx, client, command); err != nil {
			return fmt.Errorf("create remote latest link %q -> %q: %w (%s)", latestLinkPath, replacementPath, err, strings.TrimSpace(stderr))
		}
		return nil
	default:
		return fmt.Errorf("unsupported target storage type %q", target.StorageType)
	}
}

func (rc *RetentionCleaner) deleteBackupRows(ctx context.Context, backupID int64) error {
	tx, err := rc.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete backup %d rows: %w", backupID, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM tasks WHERE backup_id = ?`, backupID); err != nil {
		return fmt.Errorf("delete tasks for backup %d: %w", backupID, err)
	}
	result, err := tx.Exec(`DELETE FROM backups WHERE id = ?`, backupID)
	if err != nil {
		return fmt.Errorf("delete backup %d: %w", backupID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete result for backup %d: %w", backupID, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete backup %d: %w", backupID, sql.ErrNoRows)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete backup %d rows: %w", backupID, err)
	}

	return nil
}

func (rc *RetentionCleaner) writeAuditLog(instanceID int64, action, detail string) {
	if rc == nil || rc.db == nil || strings.TrimSpace(action) == "" {
		return
	}
	if _, err := rc.db.Exec(
		`INSERT INTO audit_logs (instance_id, user_id, action, detail, created_at) VALUES (?, NULL, ?, ?, CURRENT_TIMESTAMP)`,
		instanceID,
		strings.TrimSpace(action),
		strings.TrimSpace(detail),
	); err != nil {
		slog.Error("retention cleanup write audit log failed", "instance_id", instanceID, "action", action, "error", err)
	}
}

func (rc *RetentionCleaner) connectSSH(ctx context.Context, remote *model.RemoteConfig) (*ssh.Client, error) {
	if remote == nil {
		return nil, fmt.Errorf("ssh remote config is required")
	}
	client, err := rc.dialSSH(ctx, *remote)
	if err != nil {
		return nil, fmt.Errorf("connect ssh remote %d: %w", remote.ID, err)
	}
	return client, nil
}

func (rc *RetentionCleaner) openListSession(ctx context.Context, remote *model.RemoteConfig) (*openlist.Session, error) {
	if remote == nil {
		return nil, fmt.Errorf("openlist remote config is required")
	}
	config, err := openlist.ParseConfig(*remote)
	if err != nil {
		return nil, err
	}
	return openlist.NewClient(nil).Open(ctx, config)
}

func (rc *RetentionCleaner) removeEmptyLocalDir(dir string) {
	if rc == nil || strings.TrimSpace(dir) == "" {
		return
	}
	entries, err := rc.readDir(dir)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = rc.removeAll(dir)
}

func splitPartBasePath(snapshotPath, storageType string) (string, bool) {
	trimmed := strings.TrimSpace(snapshotPath)
	if !coldSplitPartPattern.MatchString(trimmed) {
		return "", false
	}
	if strings.ToLower(strings.TrimSpace(storageType)) == "ssh" || strings.ToLower(strings.TrimSpace(storageType)) == "openlist" {
		return strings.TrimSuffix(trimmed, pathpkg.Ext(trimmed)), true
	}
	return strings.TrimSuffix(trimmed, filepath.Ext(trimmed)), true
}