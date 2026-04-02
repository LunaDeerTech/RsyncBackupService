package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/notify"
)

type notificationDispatcher interface {
	Notify(ctx context.Context, event notify.NotifyEvent) error
}

type backupNotificationDetail struct {
	InstanceID      uint   `json:"instance_id"`
	StrategyID      *uint  `json:"strategy_id,omitempty"`
	BackupRecordID  uint   `json:"backup_record_id"`
	StorageTargetID uint   `json:"storage_target_id"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

type restoreNotificationDetail struct {
	InstanceID      uint   `json:"instance_id"`
	RestoreRecordID uint   `json:"restore_record_id"`
	BackupRecordID  uint   `json:"backup_record_id"`
	TriggeredBy     uint   `json:"triggered_by"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"error_message,omitempty"`
}

func buildBackupNotificationEvent(strategy model.Strategy, instance model.BackupInstance, record model.BackupRecord, status, errorMessage string, occurredAt time.Time) notify.NotifyEvent {
	return notify.NotifyEvent{
		Type:       backupNotificationEventType(status),
		Instance:   instance.Name,
		Strategy:   strategy.Name,
		Message:    backupNotificationMessage(instance.Name, strategy.Name, status, errorMessage),
		OccurredAt: occurredAt,
		Detail: backupNotificationDetail{
			InstanceID:      instance.ID,
			StrategyID:      record.StrategyID,
			BackupRecordID:  record.ID,
			StorageTargetID: record.StorageTargetID,
			Status:          status,
			ErrorMessage:    errorMessage,
		},
	}
}

func buildRestoreNotificationEvent(instance model.BackupInstance, record model.RestoreRecord, status, errorMessage string, occurredAt time.Time) notify.NotifyEvent {
	return notify.NotifyEvent{
		Type:       restoreNotificationEventType(status),
		Instance:   instance.Name,
		Message:    restoreNotificationMessage(instance.Name, status, errorMessage),
		OccurredAt: occurredAt,
		Detail: restoreNotificationDetail{
			InstanceID:      instance.ID,
			RestoreRecordID: record.ID,
			BackupRecordID:  record.BackupRecordID,
			TriggeredBy:     record.TriggeredBy,
			Status:          status,
			ErrorMessage:    errorMessage,
		},
	}
}

func backupNotificationEventType(status string) string {
	if strings.TrimSpace(status) == model.BackupStatusSuccess {
		return "backup_success"
	}

	return "backup_failed"
}

func restoreNotificationEventType(status string) string {
	if strings.TrimSpace(status) == RestoreStatusSuccess {
		return "restore_complete"
	}

	return "restore_failed"
}

func backupNotificationMessage(instanceName, strategyName, status, errorMessage string) string {
	if strings.TrimSpace(status) == model.BackupStatusSuccess {
		if strings.TrimSpace(strategyName) != "" {
			return fmt.Sprintf("Backup strategy %s for instance %s completed successfully.", strategyName, instanceName)
		}
		return fmt.Sprintf("Backup for instance %s completed successfully.", instanceName)
	}
	if strings.TrimSpace(errorMessage) != "" {
		return fmt.Sprintf("Backup for instance %s failed: %s", instanceName, errorMessage)
	}
	return fmt.Sprintf("Backup for instance %s failed.", instanceName)
}

func restoreNotificationMessage(instanceName, status, errorMessage string) string {
	if strings.TrimSpace(status) == RestoreStatusSuccess {
		return fmt.Sprintf("Restore for instance %s completed successfully.", instanceName)
	}
	if strings.TrimSpace(errorMessage) != "" {
		return fmt.Sprintf("Restore for instance %s failed: %s", instanceName, errorMessage)
	}
	return fmt.Sprintf("Restore for instance %s failed.", instanceName)
}