package model

import "time"

const (
	RiskSeverityInfo     = "info"
	RiskSeverityWarning  = "warning"
	RiskSeverityCritical = "critical"

	RiskSourceBackupFailed      = "backup_failed"
	RiskSourceBackupOverdue     = "backup_overdue"
	RiskSourceColdBackupMissing = "cold_backup_missing"
	RiskSourceTargetUnreachable = "target_unreachable"
	RiskSourceTargetCapacityLow = "target_capacity_low"
	RiskSourceRestoreFailed     = "restore_failed"
	RiskSourceCredentialError   = "credential_error"
)

type RiskEvent struct {
	ID         int64      `json:"id"`
	InstanceID *int64     `json:"instance_id,omitempty"`
	TargetID   *int64     `json:"target_id,omitempty"`
	Severity   string     `json:"severity"`
	Source     string     `json:"source"`
	Message    string     `json:"message"`
	Resolved   bool       `json:"resolved"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}
