package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

type backupTaskExecutor interface {
	Execute(context.Context, *model.Task, *model.Policy, *model.Instance, *model.BackupTarget, func(ProgressInfo)) error
}

type restoreTaskExecutor interface {
	Execute(context.Context, *model.Task, *model.Backup, *RestoreRequest, func(ProgressInfo)) error
}

type retryState struct {
	attempt       int
	maxRetries    int
	encryptionKey string
}

type WorkerPool struct {
	workers          int
	queue            *TaskQueue
	rolling          backupTaskExecutor
	cold             backupTaskExecutor
	restore          restoreTaskExecutor
	db               *store.DB
	retention        *RetentionCleaner
	audit            *audit.Logger
	disasterRecovery *service.DisasterRecoveryService
	riskDetector     *RiskDetector
	retryMu          sync.Mutex
	retryStates      map[int64]*retryState // keyed by backup ID
}

func NewWorkerPool(workers int, queue *TaskQueue, rolling *RollingBackupExecutor, cold *ColdBackupExecutor, db *store.DB, retention *RetentionCleaner) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if db == nil && queue != nil {
		db = queue.db
	}
	if rolling == nil && db != nil {
		rolling = NewRollingBackupExecutor(nil, db)
	}
	if cold == nil && db != nil {
		cold = NewColdBackupExecutor(nil, db, resolveRollingDataDir())
	}
	restore := restoreTaskExecutor(nil)
	if db != nil {
		restore = NewRestoreExecutor(nil, db, resolveRollingDataDir())
	}

	return &WorkerPool{
		workers:     workers,
		queue:       queue,
		rolling:     rolling,
		cold:        cold,
		restore:     restore,
		db:          db,
		retention:   retention,
		retryStates: make(map[int64]*retryState),
	}
}

func (wp *WorkerPool) SetAuditLogger(logger *audit.Logger) {
	if wp == nil {
		return
	}
	wp.audit = logger
}

func (wp *WorkerPool) SetDisasterRecoveryService(disasterRecovery *service.DisasterRecoveryService) {
	if wp == nil {
		return
	}
	wp.disasterRecovery = disasterRecovery
}

func (wp *WorkerPool) SetRiskDetector(riskDetector *RiskDetector) {
	if wp == nil {
		return
	}
	wp.riskDetector = riskDetector
}

func (wp *WorkerPool) Start(ctx context.Context) {
	if wp == nil || wp.queue == nil || wp.db == nil {
		return
	}

	var once sync.Once
	stopWorkers := func() {
		once.Do(func() {
			slog.Info("worker pool stopped")
		})
	}

	for workerID := 0; workerID < wp.workers; workerID++ {
		go func(id int) {
			for {
				task, err := wp.queue.Dequeue(ctx)
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
						stopWorkers()
						return
					}
					slog.Error("worker dequeue failed", "worker", id, "error", err)
					continue
				}
				if task == nil {
					continue
				}

				if err := wp.processTask(ctx, task); err != nil && !errors.Is(err, context.Canceled) {
					slog.Error("worker task execution failed", "worker", id, "task_id", task.ID, "error", err)
				}
			}
		}(workerID + 1)
	}
}

func (wp *WorkerPool) processTask(ctx context.Context, task *model.Task) error {
	if wp == nil {
		return fmt.Errorf("worker pool is nil")
	}
	if wp.queue == nil {
		return fmt.Errorf("task queue is nil")
	}
	if wp.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if task == nil {
		return fmt.Errorf("task is nil")
	}

	loadedTask, err := wp.db.GetTaskByID(task.ID)
	if err != nil {
		return err
	}
	if loadedTask.Status != "queued" {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	started, err := wp.queue.beginTask(loadedTask, cancel)
	if err != nil {
		cancel()
		return err
	}
	if !started {
		cancel()
		return wp.queue.Enqueue(loadedTask)
	}
	defer cancel()
	defer wp.queue.clearTaskRuntimeData(loadedTask.ID)
	defer wp.queue.OnTaskComplete(loadedTask.InstanceID)
	defer wp.invalidateDisasterRecovery(loadedTask)

	backup, policy, instance, target, err := wp.loadTaskContext(loadedTask)
	if backup != nil {
		defer wp.queue.reloadScheduledPolicy(backup)
	}
	if err != nil {
		return wp.failTask(loadedTask, backup, err)
	}

	normalizedType := normalizeTaskType(loadedTask.Type, policy)
	if normalizedType != loadedTask.Type {
		loadedTask.Type = normalizedType
		if err := wp.db.UpdateTask(loadedTask); err != nil {
			return wp.failTask(loadedTask, backup, err)
		}
	}

	if err := wp.db.UpdateInstanceStatus(instance.ID, "running"); err != nil {
		return wp.failTask(loadedTask, backup, err)
	}
	defer func() {
		if err := wp.db.UpdateInstanceStatus(instance.ID, "idle"); err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("restore instance status failed", "instance_id", instance.ID, "task_id", loadedTask.ID, "error", err)
		}
	}()

	var runErr error
	switch loadedTask.Type {
	case "rolling":
		if wp.rolling == nil {
			return wp.failTask(loadedTask, backup, fmt.Errorf("rolling executor is unavailable"))
		}
		runErr = wp.rolling.Execute(runCtx, loadedTask, policy, instance, target, nil)
	case "cold":
		if wp.cold == nil {
			return wp.failTask(loadedTask, backup, fmt.Errorf("cold executor is unavailable"))
		}
		if policy.Encryption {
			runCtx = WithColdBackupEncryptionKey(runCtx, wp.queue.coldEncryptionKey(loadedTask.ID))
		}
		runErr = wp.cold.Execute(runCtx, loadedTask, policy, instance, target, nil)
	case "restore":
		if wp.restore == nil {
			return wp.failTask(loadedTask, backup, fmt.Errorf("restore executor is unavailable"))
		}
		restoreReq := &RestoreRequest{
			RestoreType:    loadedTask.RestoreType,
			TargetPath:     loadedTask.TargetPath,
			RemoteConfigID: loadedTask.RemoteConfigID,
			EncryptionKey:  wp.queue.restoreEncryptionKey(loadedTask.ID),
		}
		runErr = wp.restore.Execute(runCtx, loadedTask, backup, restoreReq, nil)
	default:
		return wp.failTask(loadedTask, backup, fmt.Errorf("unsupported task type %q", loadedTask.Type))
	}
	if runErr != nil {
		if taskUsesManagedBackup(loadedTask) && policy != nil && policy.RetryEnabled && policy.RetryMaxRetries > 0 {
			attempt := wp.getRetryAttempt(backup)
			if attempt < policy.RetryMaxRetries {
				wp.writeRetryAudit(loadedTask, backup, policy, attempt+1, policy.RetryMaxRetries, runErr)
				wp.cleanupRetryState(backup)
				wp.scheduleRetry(loadedTask, backup, policy, attempt+1)
				return runErr
			}
			wp.writeRetryFinalFailAudit(loadedTask, backup, policy, attempt, policy.RetryMaxRetries, runErr)
			wp.cleanupRetryState(backup)
		}
		wp.writeExecutionAudit(runCtx, loadedTask, backup, policy)
		wp.handleRiskAfterFailure(runCtx, loadedTask, backup)
		return runErr
	}
	wp.cleanupRetryState(backup)
	if wp.retention != nil && taskUsesManagedBackup(loadedTask) {
		if err := wp.retention.CleanByPolicy(runCtx, policy); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("retention cleanup after backup failed", "task_id", loadedTask.ID, "policy_id", policy.ID, "error", err)
		}
	}
	wp.writeExecutionAudit(runCtx, loadedTask, backup, policy)
	wp.handleRiskAfterSuccess(runCtx, loadedTask, backup)

	return nil
}

func (wp *WorkerPool) loadTaskContext(task *model.Task) (*model.Backup, *model.Policy, *model.Instance, *model.BackupTarget, error) {
	if task == nil {
		return nil, nil, nil, nil, fmt.Errorf("task is nil")
	}

	instance, err := wp.db.GetInstanceByID(task.InstanceID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if task.BackupID == nil {
		return nil, nil, instance, nil, fmt.Errorf("task %d is missing backup id", task.ID)
	}

	backup, err := wp.db.GetBackupByID(*task.BackupID)
	if err != nil {
		return nil, nil, instance, nil, err
	}

	policy, err := wp.db.GetPolicyByID(backup.PolicyID)
	if err != nil {
		return backup, nil, instance, nil, err
	}

	target, err := wp.db.GetBackupTargetByID(policy.TargetID)
	if err != nil {
		return backup, policy, instance, nil, err
	}

	return backup, policy, instance, target, nil
}

func (wp *WorkerPool) failTask(task *model.Task, backup *model.Backup, runErr error) error {
	completedAt := time.Now().UTC()
	trimmed := strings.TrimSpace(runErr.Error())
	var persistErr error

	if backup != nil && taskUsesManagedBackup(task) {
		backup.Status = "failed"
		backup.CompletedAt = &completedAt
		backup.DurationSeconds = elapsedSeconds(backup.StartedAt, completedAt)
		backup.ErrorMessage = trimmed
		if err := wp.db.UpdateBackup(backup); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}
	if task != nil {
		task.Status = "failed"
		task.CompletedAt = &completedAt
		task.EstimatedEnd = nil
		task.ErrorMessage = trimmed
		if err := wp.db.UpdateTask(task); err != nil {
			persistErr = errors.Join(persistErr, err)
		}
	}

	if persistErr != nil {
		return errors.Join(runErr, persistErr)
	}
	wp.writeExecutionAudit(context.Background(), task, backup, nil)
	wp.handleRiskAfterFailure(context.Background(), task, backup)

	return runErr
}

func (wp *WorkerPool) writeExecutionAudit(ctx context.Context, task *model.Task, backup *model.Backup, policy *model.Policy) {
	if wp == nil || wp.audit == nil || wp.db == nil || task == nil {
		return
	}

	loadedTask, err := wp.db.GetTaskByID(task.ID)
	if err != nil {
		slog.Error("load task for audit failed", "task_id", task.ID, "error", err)
		return
	}

	switch normalizeTaskType(loadedTask.Type, policy) {
	case "restore":
		wp.writeRestoreAudit(ctx, loadedTask, backup)
	case "rolling", "cold", "backup":
		wp.writeBackupAudit(ctx, loadedTask, backup, policy)
	}
}

func (wp *WorkerPool) writeBackupAudit(ctx context.Context, task *model.Task, fallbackBackup *model.Backup, fallbackPolicy *model.Policy) {
	if task == nil || task.BackupID == nil {
		return
	}
	backup, err := wp.db.GetBackupByID(*task.BackupID)
	if err != nil {
		slog.Error("load backup for audit failed", "task_id", task.ID, "backup_id", *task.BackupID, "error", err)
		return
	}
	policyID := backup.PolicyID
	if policyID == 0 && fallbackPolicy != nil {
		policyID = fallbackPolicy.ID
	}

	var action string
	switch backup.Status {
	case "success":
		action = audit.ActionBackupComplete
	case "failed":
		action = audit.ActionBackupFail
	default:
		return
	}

	detail := map[string]any{
		"backup_id":        backup.ID,
		"policy_id":        policyID,
		"type":             backup.Type,
		"trigger_source":   backup.TriggerSource,
		"duration_seconds": backup.DurationSeconds,
	}
	if policyID > 0 {
		if p, err := wp.db.GetPolicyByID(policyID); err == nil && p != nil {
			detail["policy_name"] = p.Name
		}
	}
	if strings.TrimSpace(backup.ErrorMessage) != "" {
		detail["error_message"] = backup.ErrorMessage
	}
	if err := wp.audit.LogAction(ctx, backup.InstanceID, 0, action, detail); err != nil {
		slog.Error("write backup audit log failed", "task_id", task.ID, "backup_id", backup.ID, "action", action, "error", err)
	}

	if fallbackBackup != nil {
		*fallbackBackup = *backup
	}
}

func (wp *WorkerPool) writeRestoreAudit(ctx context.Context, task *model.Task, backup *model.Backup) {
	if task == nil {
		return
	}

	var action string
	switch task.Status {
	case "success":
		action = audit.ActionRestoreComplete
	case "failed":
		action = audit.ActionRestoreFail
	default:
		return
	}

	policyID := int64(0)
	backupID := int64(0)
	backupType := ""
	if backup != nil {
		policyID = backup.PolicyID
		backupID = backup.ID
		backupType = backup.Type
	}
	durationSeconds := int64(0)
	if task.StartedAt != nil && task.CompletedAt != nil {
		durationSeconds = elapsedSeconds(task.StartedAt, task.CompletedAt.UTC())
	}

	detail := map[string]any{
		"task_id":          task.ID,
		"backup_id":        backupID,
		"policy_id":        policyID,
		"backup_type":      backupType,
		"restore_type":     task.RestoreType,
		"target_path":      task.TargetPath,
		"duration_seconds": durationSeconds,
	}
	if policyID > 0 {
		if p, err := wp.db.GetPolicyByID(policyID); err == nil && p != nil {
			detail["policy_name"] = p.Name
		}
	}
	if strings.TrimSpace(task.ErrorMessage) != "" {
		detail["error_message"] = task.ErrorMessage
	}
	if err := wp.audit.LogAction(ctx, task.InstanceID, 0, action, detail); err != nil {
		slog.Error("write restore audit log failed", "task_id", task.ID, "action", action, "error", err)
	}
}

func normalizeTaskType(taskType string, policy *model.Policy) string {
	trimmed := strings.ToLower(strings.TrimSpace(taskType))
	if trimmed == "backup" && policy != nil {
		return strings.ToLower(strings.TrimSpace(policy.Type))
	}
	return trimmed
}

func taskUsesManagedBackup(task *model.Task) bool {
	if task == nil {
		return false
	}

	switch normalizeTaskType(task.Type, nil) {
	case "rolling", "cold", "backup":
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) invalidateDisasterRecovery(task *model.Task) {
	if wp == nil || wp.disasterRecovery == nil || task == nil || !taskUsesManagedBackup(task) {
		return
	}
	wp.disasterRecovery.Invalidate(task.InstanceID)
}

func (wp *WorkerPool) handleRiskAfterSuccess(ctx context.Context, task *model.Task, backup *model.Backup) {
	if wp == nil || wp.riskDetector == nil || task == nil || backup == nil || !taskUsesManagedBackup(task) {
		return
	}
	if err := wp.riskDetector.OnBackupSuccess(ctx, task.InstanceID, backup.PolicyID); err != nil {
		slog.Error("backup success risk detection failed", "task_id", task.ID, "instance_id", task.InstanceID, "policy_id", backup.PolicyID, "error", err)
	}
}

func (wp *WorkerPool) handleRiskAfterFailure(ctx context.Context, task *model.Task, backup *model.Backup) {
	if wp == nil || wp.riskDetector == nil || task == nil {
		return
	}
	if backup != nil && taskUsesManagedBackup(task) {
		if err := wp.riskDetector.OnBackupFailed(ctx, task.InstanceID, backup.PolicyID); err != nil {
			slog.Error("backup failure risk detection failed", "task_id", task.ID, "instance_id", task.InstanceID, "policy_id", backup.PolicyID, "error", err)
		}
		return
	}
	if normalizeTaskType(task.Type, nil) == "restore" {
		if err := wp.riskDetector.OnRestoreFailed(ctx, task.InstanceID); err != nil {
			slog.Error("restore failure risk detection failed", "task_id", task.ID, "instance_id", task.InstanceID, "error", err)
		}
	}
}

// ── Retry support ──

func (wp *WorkerPool) getRetryAttempt(backup *model.Backup) int {
	if wp == nil || backup == nil {
		return 0
	}
	wp.retryMu.Lock()
	defer wp.retryMu.Unlock()
	if state, ok := wp.retryStates[backup.ID]; ok {
		return state.attempt
	}
	return 0
}

func (wp *WorkerPool) setRetryState(backupID int64, state *retryState) {
	if wp == nil {
		return
	}
	wp.retryMu.Lock()
	defer wp.retryMu.Unlock()
	wp.retryStates[backupID] = state
}

func (wp *WorkerPool) cleanupRetryState(backup *model.Backup) {
	if wp == nil || backup == nil {
		return
	}
	wp.retryMu.Lock()
	defer wp.retryMu.Unlock()
	delete(wp.retryStates, backup.ID)
}

func (wp *WorkerPool) scheduleRetry(task *model.Task, backup *model.Backup, policy *model.Policy, nextAttempt int) {
	if wp == nil || wp.db == nil || wp.queue == nil || policy == nil || backup == nil {
		return
	}

	delay := time.Duration(nextAttempt) * 5 * time.Second
	encryptionKey := ""
	if policy.Type == "cold" && policy.Encryption {
		encryptionKey = wp.queue.coldEncryptionKey(task.ID)
	}

	slog.Info("scheduling backup retry",
		"policy_id", policy.ID,
		"backup_id", backup.ID,
		"attempt", nextAttempt,
		"max_retries", policy.RetryMaxRetries,
		"delay", delay,
	)

	time.AfterFunc(delay, func() {
		wp.executeRetry(policy, nextAttempt, encryptionKey, backup.TriggerSource)
	})
}

func (wp *WorkerPool) executeRetry(policy *model.Policy, attempt int, encryptionKey string, triggerSource string) {
	if wp == nil || wp.db == nil || wp.queue == nil || policy == nil {
		return
	}

	retryBackup, retryTask, err := wp.db.CreatePendingPolicyRunWithSource(policy, triggerSource)
	if err != nil {
		slog.Error("create retry backup+task failed",
			"policy_id", policy.ID,
			"attempt", attempt,
			"error", err,
		)
		return
	}

	retryTask.CurrentStep = fmt.Sprintf("queued（重试%d/%d）", attempt, policy.RetryMaxRetries)
	if err := wp.db.UpdateTask(retryTask); err != nil {
		slog.Error("update retry task step failed", "task_id", retryTask.ID, "error", err)
	}

	wp.setRetryState(retryBackup.ID, &retryState{
		attempt:       attempt,
		maxRetries:    policy.RetryMaxRetries,
		encryptionKey: encryptionKey,
	})

	if policy.Type == "cold" && encryptionKey != "" {
		wp.queue.SetColdEncryptionKey(retryTask.ID, encryptionKey)
	}

	if err := wp.queue.Enqueue(retryTask); err != nil {
		slog.Error("enqueue retry task failed",
			"policy_id", policy.ID,
			"task_id", retryTask.ID,
			"attempt", attempt,
			"error", err,
		)
	}
}

func (wp *WorkerPool) writeRetryAudit(task *model.Task, backup *model.Backup, policy *model.Policy, attempt, maxRetries int, runErr error) {
	if wp == nil || wp.audit == nil || task == nil {
		return
	}

	instanceID := task.InstanceID
	detail := map[string]any{
		"task_id":     task.ID,
		"policy_id":   policy.ID,
		"policy_name": policy.Name,
		"type":        policy.Type,
		"attempt":     attempt,
		"max_retries": maxRetries,
		"error":       strings.TrimSpace(runErr.Error()),
		"next_delay":  fmt.Sprintf("%ds", attempt*5),
	}
	if backup != nil {
		detail["backup_id"] = backup.ID
		detail["trigger_source"] = backup.TriggerSource
	}
	if err := wp.audit.LogAction(context.Background(), instanceID, 0, audit.ActionBackupRetry, detail); err != nil {
		slog.Error("write retry audit failed", "task_id", task.ID, "error", err)
	}
}

func (wp *WorkerPool) writeRetryFinalFailAudit(task *model.Task, backup *model.Backup, policy *model.Policy, attempt, maxRetries int, runErr error) {
	if wp == nil || wp.audit == nil || task == nil {
		return
	}

	instanceID := task.InstanceID
	detail := map[string]any{
		"task_id":     task.ID,
		"policy_id":   policy.ID,
		"policy_name": policy.Name,
		"type":        policy.Type,
		"attempt":     attempt,
		"max_retries": maxRetries,
		"error":       strings.TrimSpace(runErr.Error()),
		"final":       true,
	}
	if backup != nil {
		detail["backup_id"] = backup.ID
		detail["trigger_source"] = backup.TriggerSource
	}
	if err := wp.audit.LogAction(context.Background(), instanceID, 0, audit.ActionBackupRetryExhausted, detail); err != nil {
		slog.Error("write retry exhausted audit failed", "task_id", task.ID, "error", err)
	}
}
