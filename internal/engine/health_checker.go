package engine

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/openlist"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

const defaultHealthCheckInterval = 30 * time.Minute

type HealthChecker struct {
	db               *store.DB
	scheduleInterval time.Duration
	disasterRecovery *service.DisasterRecoveryService
	riskDetector     *RiskDetector
}

func NewHealthChecker(db *store.DB) *HealthChecker {
	return &HealthChecker{
		db:               db,
		scheduleInterval: defaultHealthCheckInterval,
	}
}

func (hc *HealthChecker) SetDisasterRecoveryService(disasterRecovery *service.DisasterRecoveryService) {
	if hc == nil {
		return
	}
	hc.disasterRecovery = disasterRecovery
}

func (hc *HealthChecker) SetRiskDetector(riskDetector *RiskDetector) {
	if hc == nil {
		return
	}
	hc.riskDetector = riskDetector
}

func (hc *HealthChecker) CheckTarget(target *model.BackupTarget) (status, message string, total, used *int64, err error) {
	if target == nil {
		return "", "", nil, nil, fmt.Errorf("backup target is nil")
	}

	switch strings.ToLower(strings.TrimSpace(target.StorageType)) {
	case "local":
		return hc.checkLocalTarget(target)
	case "ssh":
		return hc.checkSSHTarget(target)
	case "openlist":
		return hc.checkOpenListTarget(target)
	case "cloud":
		return "degraded", "cloud storage health checks are not supported yet", nil, nil, nil
	default:
		return "unreachable", "unsupported storage type", nil, nil, nil
	}
}

func (hc *HealthChecker) CheckAll() {
	if hc == nil || hc.db == nil {
		return
	}

	targets, err := hc.db.ListBackupTargets()
	if err != nil {
		slog.Error("health check list backup targets failed", "error", err)
		return
	}

	for index := range targets {
		target := &targets[index]
		status, message, total, used, err := hc.CheckTarget(target)
		if err != nil {
			slog.Error("health check target failed", "target_id", target.ID, "error", err)
			continue
		}
		if err := hc.db.UpdateHealthStatus(target.ID, status, message, total, used); err != nil {
			slog.Error("persist health check result failed", "target_id", target.ID, "error", err)
			continue
		}
		if hc.riskDetector != nil {
			if err := hc.riskDetector.OnHealthCheckComplete(context.Background(), target.ID, status); err != nil {
				slog.Error("health check risk detection failed", "target_id", target.ID, "status", status, "error", err)
			}
		}
		if TargetHealthChanged(target, status, message, total, used) && hc.disasterRecovery != nil {
			hc.disasterRecovery.InvalidateByTarget(target.ID)
		}
	}
	if hc.riskDetector != nil {
		if err := hc.riskDetector.PeriodicCheck(context.Background()); err != nil {
			slog.Error("periodic risk scan failed", "error", err)
		}
	}
}

func (hc *HealthChecker) StartSchedule(ctx context.Context) {
	if hc == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		hc.CheckAll()

		ticker := time.NewTicker(hc.scheduleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.CheckAll()
			}
		}
	}()
}

func (hc *HealthChecker) checkLocalTarget(target *model.BackupTarget) (string, string, *int64, *int64, error) {
	path := strings.TrimSpace(target.StoragePath)
	if path == "" {
		return "unreachable", "storage path is required", nil, nil, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "unreachable", "local path does not exist", nil, nil, nil
		}
		return "unreachable", "failed to access local path", nil, nil, nil
	}
	if !info.IsDir() {
		return "unreachable", "local path must be a directory", nil, nil, nil
	}

	file, err := os.CreateTemp(path, ".rbs-health-*")
	if err != nil {
		return "unreachable", "local path is not writable", nil, nil, nil
	}
	fileName := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(fileName)
		return "unreachable", "local path is not writable", nil, nil, nil
	}
	_ = os.Remove(fileName)

	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return "degraded", "local path is writable but capacity probe failed", nil, nil, nil
	}

	total := int64(stats.Blocks) * int64(stats.Bsize)
	used := int64(stats.Blocks-stats.Bfree) * int64(stats.Bsize)
	return "healthy", "local target is healthy", &total, &used, nil
}

func (hc *HealthChecker) checkSSHTarget(target *model.BackupTarget) (string, string, *int64, *int64, error) {
	if hc == nil || hc.db == nil {
		return "", "", nil, nil, fmt.Errorf("database unavailable")
	}
	if target.RemoteConfigID == nil {
		return "unreachable", "ssh target is missing remote config", nil, nil, nil
	}

	remoteConfig, err := hc.db.GetRemoteConfigByID(*target.RemoteConfigID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "unreachable", "ssh remote config not found", nil, nil, nil
		}
		return "", "", nil, nil, err
	}
	if remoteConfig.Type != "ssh" {
		return "unreachable", "remote config must be ssh", nil, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := service.DialSSHClient(ctx, *remoteConfig)
	if err != nil {
		return "unreachable", err.Error(), nil, nil, nil
	}
	defer client.Close()

	path := shellQuote(strings.TrimSpace(target.StoragePath))
	stdout, _, err := runSSHCommand(ctx, client, "test -d "+path+" && test -w "+path+" && echo ok")
	if err != nil || strings.TrimSpace(stdout) != "ok" {
		return "unreachable", "remote path is unavailable or not writable", nil, nil, nil
	}

	dfOutput, _, err := runSSHCommand(ctx, client, "df -B1 "+path)
	if err != nil {
		return "degraded", "remote path is reachable but capacity probe failed", nil, nil, nil
	}
	total, used, err := parseDFBytes(dfOutput)
	if err != nil {
		return "degraded", "remote path is reachable but capacity parsing failed", nil, nil, nil
	}

	return "healthy", "ssh target is healthy", total, used, nil
}

func (hc *HealthChecker) checkOpenListTarget(target *model.BackupTarget) (string, string, *int64, *int64, error) {
	if hc == nil || hc.db == nil {
		return "", "", nil, nil, fmt.Errorf("database unavailable")
	}
	if target.RemoteConfigID == nil {
		return "unreachable", "openlist target is missing remote config", nil, nil, nil
	}

	remoteConfig, err := hc.db.GetRemoteConfigByID(*target.RemoteConfigID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "unreachable", "openlist remote config not found", nil, nil, nil
		}
		return "", "", nil, nil, err
	}
	if !openlist.IsRemoteConfig(*remoteConfig) {
		return "unreachable", "remote config must be openlist", nil, nil, nil
	}

	config, err := openlist.ParseConfig(*remoteConfig)
	if err != nil {
		return "unreachable", "openlist remote config is invalid", nil, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := openlist.NewClient(nil).Open(ctx, config)
	if err != nil {
		return "unreachable", strings.TrimSpace(err.Error()), nil, nil, nil
	}
	object, err := session.Get(ctx, target.StoragePath)
	if err != nil {
		if errors.Is(err, openlist.ErrNotFound) {
			return "unreachable", "openlist path does not exist", nil, nil, nil
		}
		return "unreachable", strings.TrimSpace(err.Error()), nil, nil, nil
	}
	if !object.IsDir {
		return "unreachable", "openlist path must be a directory", nil, nil, nil
	}
	if object.MountDetails == nil || object.MountDetails.TotalSpace <= 0 {
		return "healthy", "openlist target is healthy", nil, nil, nil
	}

	total := object.MountDetails.TotalSpace
	free := object.MountDetails.FreeSpace
	if free < 0 || free > total {
		return "healthy", "openlist target is healthy", nil, nil, nil
	}
	used := total - free
	return "healthy", "openlist target is healthy", &total, &used, nil
}

func runSSHCommand(ctx context.Context, client *ssh.Client, command string) (string, string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	commandDone := make(chan error, 1)
	go func() {
		commandDone <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return stdout.String(), stderr.String(), ctx.Err()
	case err := <-commandDone:
		return stdout.String(), stderr.String(), err
	}
}

func parseDFBytes(output string) (*int64, *int64, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil, nil, fmt.Errorf("unexpected df output")
	}

	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 3 {
		return nil, nil, fmt.Errorf("unexpected df line")
	}

	total, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parse total capacity: %w", err)
	}
	used, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parse used capacity: %w", err)
	}

	return &total, &used, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TargetHealthChanged(target *model.BackupTarget, status, message string, total, used *int64) bool {
	if target == nil {
		return false
	}
	if target.HealthStatus != status || target.HealthMessage != message {
		return true
	}
	if !sameOptionalInt64(target.TotalCapacityBytes, total) {
		return true
	}
	if !sameOptionalInt64(target.UsedCapacityBytes, used) {
		return true
	}
	return false
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
