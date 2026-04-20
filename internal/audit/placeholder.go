package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

const (
	ActionInstanceCreate           = "instance.create"
	ActionInstanceUpdate           = "instance.update"
	ActionInstanceDelete           = "instance.delete"
	ActionPolicyCreate             = "policy.create"
	ActionPolicyUpdate             = "policy.update"
	ActionPolicyDelete             = "policy.delete"
	ActionBackupTrigger            = "backup.trigger"
	ActionBackupComplete           = "backup.complete"
	ActionBackupFail               = "backup.fail"
	ActionBackupRetry              = "backup.retry"
	ActionBackupRetryExhausted     = "backup.retry_exhausted"
	ActionBackupMoveRetry          = "backup.move_retry"
	ActionBackupMoveRetryExhausted = "backup.move_retry_exhausted"
	ActionHookCommandSuccess       = "hook.command_success"
	ActionHookCommandFail          = "hook.command_fail"
	ActionRestoreTrigger           = "restore.trigger"
	ActionRestoreComplete          = "restore.complete"
	ActionRestoreFail              = "restore.fail"
	ActionUserCreate               = "user.create"
	ActionUserUpdate               = "user.update"
	ActionUserDelete               = "user.delete"
	ActionUserPasswordReset        = "user.password_reset"
	ActionTargetCreate             = "target.create"
	ActionTargetUpdate             = "target.update"
	ActionTargetDelete             = "target.delete"
	ActionRemoteCreate             = "remote.create"
	ActionRemoteUpdate             = "remote.update"
	ActionRemoteDelete             = "remote.delete"
	ActionBackupDownload           = "backup.download"
	ActionSystemConfigUpdate       = "system.config.update"
	ActionRiskCreate               = "risk.create"
	ActionRiskEscalate             = "risk.escalate"
	ActionRiskResolve              = "risk.resolve"
	ActionLoginSuccess             = "auth.login_success"
	ActionLoginFailed              = "auth.login_failed"
)

type Logger struct {
	db *store.DB
}

func NewLogger(db *store.DB) *Logger {
	return &Logger{db: db}
}

func (l *Logger) LogAction(ctx context.Context, instanceID, userID int64, action string, detail interface{}) error {
	_ = ctx
	if l == nil || l.db == nil {
		return nil
	}

	trimmedAction := strings.TrimSpace(action)
	if trimmedAction == "" {
		return fmt.Errorf("audit action is required")
	}

	payload, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("marshal audit detail: %w", err)
	}

	return l.db.CreateAuditLog(&model.AuditLog{
		InstanceID: optionalInt64(instanceID),
		UserID:     optionalInt64(userID),
		Action:     trimmedAction,
		Detail:     json.RawMessage(payload),
	})
}

func optionalInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}

	cloned := value
	return &cloned
}
