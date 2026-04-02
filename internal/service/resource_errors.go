package service

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrBackupRecordNotFound     = errors.New("backup record not found")
	ErrBackupRecordNotRestorable = errors.New("backup record is not restorable")
	ErrAuditLogQueryInvalid     = errors.New("invalid audit log query")
	ErrBasePathRequired          = errors.New("base path is required")
	ErrHostRequired             = errors.New("host is required")
	ErrInvalidBackupType        = errors.New("invalid backup type")
	ErrInvalidColdVolumeSize    = errors.New("invalid cold volume size")
	ErrInvalidNotificationChannelConfig = errors.New("invalid notification channel config")
	ErrInvalidNotificationChannelType = errors.New("invalid notification channel type")
	ErrInvalidNotificationEvent = errors.New("invalid notification event")
	ErrInvalidNotificationSubscriptionConfig = errors.New("invalid notification subscription config")
	ErrInvalidPort              = errors.New("invalid port")
	ErrInvalidRestoreTargetPath = errors.New("invalid restore target path")
	ErrInvalidRetention         = errors.New("invalid retention configuration")
	ErrInvalidSchedule          = errors.New("invalid schedule configuration")
	ErrInvalidSourceType        = errors.New("invalid source type")
	ErrInvalidSSHKey            = errors.New("invalid ssh private key")
	ErrInvalidSSHKeyPermissions = errors.New("ssh private key permissions must be 0600")
	ErrInvalidStorageTargetType = errors.New("invalid storage target type")
	ErrInvalidMaxExecution      = errors.New("invalid max execution seconds")
	ErrDuplicateStorageTargetLocation = errors.New("storage target location already exists")
	ErrRollingTargetConflict    = errors.New("rolling storage target conflict")
	ErrNameRequired             = errors.New("name is required")
	ErrNotificationChannelNotFound = errors.New("notification channel not found")
	ErrNotificationSubscriptionNotFound = errors.New("notification subscription not found")
	ErrPrivateKeyPathRequired   = errors.New("private key path is required")
	ErrRestoreTargetExists      = errors.New("restore target path already exists")
	ErrRestoreTargetPathRequired = errors.New("restore target path is required")
	ErrResourceInUse            = errors.New("resource is still in use")
	ErrScheduleRequired         = errors.New("either cron_expr or interval_seconds is required")
	ErrSSHKeyNotFound           = errors.New("ssh key not found")
	ErrSSHKeyRequired           = errors.New("ssh key is required")
	ErrSourcePathRequired       = errors.New("source path is required")
	ErrStorageTargetNotFound    = errors.New("storage target not found")
	ErrStorageTargetsRequired   = errors.New("at least one storage target is required")
	ErrStrategyNotFound         = errors.New("strategy not found")
	ErrUserRequired             = errors.New("user is required")
	ErrUnexpectedSSHFields      = errors.New("ssh-specific fields are not allowed for local resources")
)

func IsValidationError(err error) bool {
	switch {
	case errors.Is(err, ErrBasePathRequired),
		errors.Is(err, ErrAuditLogQueryInvalid),
		errors.Is(err, ErrHostRequired),
		errors.Is(err, ErrInvalidBackupType),
		errors.Is(err, ErrInvalidColdVolumeSize),
		errors.Is(err, ErrInvalidMaxExecution),
		errors.Is(err, ErrInvalidNotificationChannelConfig),
		errors.Is(err, ErrInvalidNotificationChannelType),
		errors.Is(err, ErrInvalidNotificationEvent),
		errors.Is(err, ErrInvalidNotificationSubscriptionConfig),
		errors.Is(err, ErrInvalidPort),
		errors.Is(err, ErrInvalidRestoreTargetPath),
		errors.Is(err, ErrInvalidRetention),
		errors.Is(err, ErrInvalidSchedule),
		errors.Is(err, ErrInvalidSourceType),
		errors.Is(err, ErrInvalidSSHKey),
		errors.Is(err, ErrInvalidSSHKeyPermissions),
		errors.Is(err, ErrInvalidStorageTargetType),
		errors.Is(err, ErrDuplicateStorageTargetLocation),
		errors.Is(err, ErrRollingTargetConflict),
		errors.Is(err, ErrNameRequired),
		errors.Is(err, ErrPrivateKeyPathRequired),
		errors.Is(err, ErrRestoreTargetPathRequired),
		errors.Is(err, ErrScheduleRequired),
		errors.Is(err, ErrSSHKeyRequired),
		errors.Is(err, ErrSourcePathRequired),
		errors.Is(err, ErrStorageTargetsRequired),
		errors.Is(err, ErrUnexpectedSSHFields),
		errors.Is(err, ErrUserRequired):
		return true
	default:
		return false
	}
}

func normalizeSSHRuntimeError(err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	switch {
	case strings.Contains(message, "ssh private key permissions must be 0600"):
		return fmt.Errorf("%w: ssh private key permissions must be 0600", ErrInvalidSSHKeyPermissions)
	case strings.Contains(message, "ssh private key is invalid"):
		return ErrInvalidSSHKey
	default:
		return err
	}
}