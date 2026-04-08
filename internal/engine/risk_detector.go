package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

type DRCache = service.DRCache

type emailDispatcher interface {
	Send(to, subject, body string)
}

type RiskDetector struct {
	db      *store.DB
	drCache *DRCache
	audit   *audit.Logger
	now     func() time.Time
	email   emailDispatcher
}

func NewRiskDetector(db *store.DB, drCache *DRCache, auditLogger *audit.Logger) *RiskDetector {
	return &RiskDetector{
		db:      db,
		drCache: drCache,
		audit:   auditLogger,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (rd *RiskDetector) SetClock(now func() time.Time) {
	if rd == nil || now == nil {
		return
	}
	rd.now = func() time.Time { return now().UTC() }
}

func (rd *RiskDetector) SetEmailSender(sender emailDispatcher) {
	if rd == nil {
		return
	}
	rd.email = sender
}

func (rd *RiskDetector) OnBackupFailed(ctx context.Context, instanceID int64, policyID int64) error {
	if err := rd.validate(); err != nil {
		return err
	}

	failures, err := rd.db.CountConsecutiveFailures(instanceID, policyID)
	if err != nil {
		return err
	}
	severity := model.RiskSeverityWarning
	if failures >= 3 {
		severity = model.RiskSeverityCritical
	}

	if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceBackupFailed, severity, "实例最近备份执行失败", map[string]any{
		"instance_id":          instanceID,
		"policy_id":            policyID,
		"consecutive_failures": failures,
	}); err != nil {
		return err
	}

	message, err := rd.latestBackupFailureMessage(instanceID, policyID)
	if err != nil {
		return err
	}
	if isCredentialError(message) {
		if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceCredentialError, model.RiskSeverityCritical, "检测到 SSH 凭证异常，请检查远程配置或密钥", map[string]any{
			"instance_id": instanceID,
			"policy_id":   policyID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (rd *RiskDetector) OnBackupSuccess(ctx context.Context, instanceID int64, policyID int64) error {
	if err := rd.validate(); err != nil {
		return err
	}

	hasFailures, err := rd.instanceHasActiveBackupFailure(instanceID)
	if err != nil {
		return err
	}
	if !hasFailures {
		if err := rd.resolveRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceBackupFailed, map[string]any{
			"instance_id": instanceID,
			"policy_id":   policyID,
		}); err != nil {
			return err
		}
	}

	hasOverdue, err := rd.instanceHasOverduePolicy(instanceID)
	if err != nil {
		return err
	}
	if !hasOverdue {
		if err := rd.resolveRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceBackupOverdue, map[string]any{
			"instance_id": instanceID,
			"policy_id":   policyID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (rd *RiskDetector) OnHealthCheckComplete(ctx context.Context, targetID int64, status string) error {
	if err := rd.validate(); err != nil {
		return err
	}

	target, err := rd.db.GetBackupTargetByID(targetID)
	if err != nil {
		return err
	}

	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	switch normalizedStatus {
	case "unreachable":
		if err := rd.ensureRiskEvent(ctx, nil, int64Ptr(targetID), model.RiskSourceTargetUnreachable, model.RiskSeverityCritical, "备份目标当前不可达", map[string]any{
			"target_id": targetID,
			"status":    normalizedStatus,
		}); err != nil {
			return err
		}
	case "healthy":
		if err := rd.resolveRiskEvent(ctx, nil, int64Ptr(targetID), model.RiskSourceTargetUnreachable, map[string]any{
			"target_id": targetID,
			"status":    normalizedStatus,
		}); err != nil {
			return err
		}
	}

	severity, ok := capacitySeverity(target)
	if ok {
		if err := rd.ensureRiskEvent(ctx, nil, int64Ptr(targetID), model.RiskSourceTargetCapacityLow, severity, "备份目标剩余容量不足", map[string]any{
			"target_id": targetID,
			"status":    normalizedStatus,
		}); err != nil {
			return err
		}
	} else {
		if err := rd.resolveRiskEvent(ctx, nil, int64Ptr(targetID), model.RiskSourceTargetCapacityLow, map[string]any{
			"target_id": targetID,
			"status":    normalizedStatus,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (rd *RiskDetector) OnRestoreFailed(ctx context.Context, instanceID int64) error {
	if err := rd.validate(); err != nil {
		return err
	}

	if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceRestoreFailed, model.RiskSeverityCritical, "实例最近恢复任务失败", map[string]any{
		"instance_id": instanceID,
	}); err != nil {
		return err
	}

	message, err := rd.latestRestoreFailureMessage(instanceID)
	if err != nil {
		return err
	}
	if isCredentialError(message) {
		if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceCredentialError, model.RiskSeverityCritical, "检测到 SSH 凭证异常，请检查远程配置或密钥", map[string]any{
			"instance_id": instanceID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (rd *RiskDetector) PeriodicCheck(ctx context.Context) error {
	if err := rd.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	overdueByInstance, err := rd.collectOverdueRisks(ctx)
	if err != nil {
		return err
	}
	for instanceID, severity := range overdueByInstance {
		if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceBackupOverdue, severity, "存在超期未完成的备份策略", map[string]any{
			"instance_id": instanceID,
		}); err != nil {
			return err
		}
	}
	if err := rd.resolveMissingInstanceRisks(ctx, model.RiskSourceBackupOverdue, func(instanceID int64) bool {
		_, ok := overdueByInstance[instanceID]
		return ok
	}); err != nil {
		return err
	}

	missingColdByInstance, err := rd.collectColdBackupMissingRisks(ctx)
	if err != nil {
		return err
	}
	for instanceID := range missingColdByInstance {
		if err := rd.ensureRiskEvent(ctx, int64Ptr(instanceID), nil, model.RiskSourceColdBackupMissing, model.RiskSeverityInfo, "实例仅配置了滚动备份，缺少冷备份策略", map[string]any{
			"instance_id": instanceID,
		}); err != nil {
			return err
		}
	}
	if err := rd.resolveMissingInstanceRisks(ctx, model.RiskSourceColdBackupMissing, func(instanceID int64) bool {
		_, ok := missingColdByInstance[instanceID]
		return ok
	}); err != nil {
		return err
	}

	return nil
}

func (rd *RiskDetector) validate() error {
	if rd == nil {
		return fmt.Errorf("risk detector is nil")
	}
	if rd.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if rd.now == nil {
		rd.now = func() time.Time { return time.Now().UTC() }
	}
	return nil
}

func (rd *RiskDetector) collectOverdueRisks(ctx context.Context) (map[int64]string, error) {
	enabledPolicies, err := rd.db.ListEnabledPolicies()
	if err != nil {
		return nil, err
	}

	now := rd.now().UTC()
	overdueByInstance := make(map[int64]string)
	for index := range enabledPolicies {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		policy := enabledPolicies[index]
		period, err := riskSchedulePeriod(policy, now)
		if err != nil {
			slog.Warn("skip backup overdue risk because policy schedule is invalid", "policy_id", policy.ID, "error", err)
			continue
		}

		referenceTime := policy.CreatedAt.UTC()
		backup, err := rd.db.GetLatestSuccessfulBackup(policy.InstanceID, policy.ID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		} else {
			referenceTime = riskBackupCompletedAt(backup)
		}

		severity := overdueSeverity(now.Sub(referenceTime), period)
		if severity == "" {
			continue
		}
		if riskSeverityRank(severity) > riskSeverityRank(overdueByInstance[policy.InstanceID]) {
			overdueByInstance[policy.InstanceID] = severity
		}
	}

	return overdueByInstance, nil
}

func (rd *RiskDetector) collectColdBackupMissingRisks(ctx context.Context) (map[int64]struct{}, error) {
	instances, err := rd.db.ListInstances()
	if err != nil {
		return nil, err
	}

	missing := make(map[int64]struct{})
	for index := range instances {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		policies, err := rd.db.ListPoliciesByInstance(instances[index].ID)
		if err != nil {
			return nil, err
		}

		hasRolling := false
		hasCold := false
		for _, policy := range policies {
			if !policy.Enabled {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(policy.Type)) {
			case "rolling":
				hasRolling = true
			case "cold":
				hasCold = true
			}
		}
		if hasRolling && !hasCold {
			missing[instances[index].ID] = struct{}{}
		}
	}

	return missing, nil
}

func (rd *RiskDetector) resolveMissingInstanceRisks(ctx context.Context, source string, keep func(int64) bool) error {
	resolved := false
	events, _, err := rd.db.ListRiskEvents(store.RiskEventQuery{
		Source:   source,
		Resolved: &resolved,
	})
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.InstanceID == nil {
			continue
		}
		if keep != nil && keep(*event.InstanceID) {
			continue
		}
		if err := rd.resolveRiskEvent(ctx, event.InstanceID, nil, source, map[string]any{
			"instance_id": *event.InstanceID,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (rd *RiskDetector) ensureRiskEvent(ctx context.Context, instanceID *int64, targetID *int64, source string, severity string, message string, detail map[string]any) error {
	event, err := rd.db.GetActiveRiskEvent(instanceID, targetID, source)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		created := &model.RiskEvent{
			InstanceID: cloneInt64(instanceID),
			TargetID:   cloneInt64(targetID),
			Severity:   severity,
			Source:     source,
			Message:    message,
			Resolved:   false,
		}
		if err := rd.db.CreateRiskEvent(created); err != nil {
			return err
		}
		rd.writeAudit(ctx, audit.ActionRiskCreate, created, mergeAuditDetail(detail, map[string]any{
			"risk_event_id": created.ID,
			"source":        source,
			"severity":      severity,
			"message":       message,
		}))
		rd.notify(ctx, created)
		rd.invalidateRelated(created.InstanceID, created.TargetID)
		return nil
	}

	if riskSeverityRank(severity) <= riskSeverityRank(event.Severity) {
		return nil
	}
	if err := rd.db.UpdateRiskEventSeverity(event.ID, severity); err != nil {
		return err
	}
	rd.writeAudit(ctx, audit.ActionRiskEscalate, event, mergeAuditDetail(detail, map[string]any{
		"risk_event_id": event.ID,
		"source":        source,
		"from_severity": event.Severity,
		"to_severity":   severity,
	}))
	event.Severity = severity
	if severity == model.RiskSeverityCritical {
		rd.notify(ctx, event)
	}
	rd.invalidateRelated(event.InstanceID, event.TargetID)
	return nil
}

func (rd *RiskDetector) resolveRiskEvent(ctx context.Context, instanceID *int64, targetID *int64, source string, detail map[string]any) error {
	event, err := rd.db.GetActiveRiskEvent(instanceID, targetID, source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if err := rd.db.ResolveRiskEvent(event.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	rd.writeAudit(ctx, audit.ActionRiskResolve, event, mergeAuditDetail(detail, map[string]any{
		"risk_event_id": event.ID,
		"source":        source,
		"severity":      event.Severity,
	}))
	rd.invalidateRelated(event.InstanceID, event.TargetID)
	return nil
}

func (rd *RiskDetector) writeAudit(ctx context.Context, action string, event *model.RiskEvent, detail map[string]any) {
	if rd == nil || rd.audit == nil {
		return
	}
	instanceID := int64(0)
	if event != nil && event.InstanceID != nil {
		instanceID = *event.InstanceID
	}
	if err := rd.audit.LogAction(ctx, instanceID, 0, action, detail); err != nil {
		slog.Error("write risk audit log failed", "action", action, "instance_id", instanceID, "error", err)
	}
}

func (rd *RiskDetector) invalidateRelated(instanceID *int64, targetID *int64) {
	if rd == nil || rd.drCache == nil {
		return
	}
	seen := make(map[int64]struct{})
	if instanceID != nil && *instanceID > 0 {
		seen[*instanceID] = struct{}{}
	}
	if targetID != nil && *targetID > 0 && rd.db != nil {
		instanceIDs, err := rd.db.ListInstanceIDsByTargetID(*targetID)
		if err == nil {
			for _, id := range instanceIDs {
				seen[id] = struct{}{}
			}
		}
	}
	for id := range seen {
		rd.drCache.Invalidate(id)
	}
}

func (rd *RiskDetector) notify(ctx context.Context, event *model.RiskEvent) {
	if rd == nil || rd.email == nil || rd.db == nil || event == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	instanceIDs, err := rd.notificationInstanceIDs(event)
	if err != nil {
		slog.Error("resolve risk notification instances failed", "risk_event_id", event.ID, "error", err)
		return
	}
	if len(instanceIDs) == 0 {
		return
	}

	var target *model.BackupTarget
	if event.TargetID != nil {
		target, err = rd.db.GetBackupTargetByID(*event.TargetID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Warn("load backup target for risk notification failed", "target_id", *event.TargetID, "error", err)
		}
	}

	for _, instanceID := range instanceIDs {
		instance, err := rd.db.GetInstanceByID(instanceID)
		if err != nil {
			slog.Warn("load instance for risk notification failed", "instance_id", instanceID, "error", err)
			continue
		}
		subscribers, err := rd.db.ListSubscribersByInstance(instanceID)
		if err != nil {
			slog.Warn("load subscribers for risk notification failed", "instance_id", instanceID, "error", err)
			continue
		}
		if len(subscribers) == 0 {
			continue
		}
		subject := buildRiskNotificationSubject(event, instance, target)
		body := buildRiskNotificationBody(event, instance, target)
		for _, user := range subscribers {
			rd.email.Send(user.Email, subject, body)
		}
	}
}

func (rd *RiskDetector) notificationInstanceIDs(event *model.RiskEvent) ([]int64, error) {
	if event == nil {
		return nil, nil
	}
	if event.InstanceID != nil && *event.InstanceID > 0 {
		return []int64{*event.InstanceID}, nil
	}
	if event.TargetID != nil && *event.TargetID > 0 {
		return rd.db.ListInstanceIDsByTargetID(*event.TargetID)
	}
	return nil, nil
}

func (rd *RiskDetector) latestBackupFailureMessage(instanceID int64, policyID int64) (string, error) {
	backups, err := rd.db.ListRecentBackupsByInstanceAllStatuses(instanceID, 20)
	if err != nil {
		return "", err
	}
	for _, backup := range backups {
		if backup.PolicyID != policyID || !strings.EqualFold(backup.Status, "failed") {
			continue
		}
		return strings.TrimSpace(backup.ErrorMessage), nil
	}
	return "", nil
}

func (rd *RiskDetector) latestRestoreFailureMessage(instanceID int64) (string, error) {
	tasks, err := rd.db.ListTasksByInstance(instanceID)
	if err != nil {
		return "", err
	}
	for _, task := range tasks {
		if !strings.EqualFold(task.Type, "restore") || !strings.EqualFold(task.Status, "failed") {
			continue
		}
		return strings.TrimSpace(task.ErrorMessage), nil
	}
	return "", nil
}

func (rd *RiskDetector) instanceHasActiveBackupFailure(instanceID int64) (bool, error) {
	policies, err := rd.db.ListPoliciesByInstance(instanceID)
	if err != nil {
		return false, err
	}
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		count, err := rd.db.CountConsecutiveFailures(instanceID, policy.ID)
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (rd *RiskDetector) instanceHasOverduePolicy(instanceID int64) (bool, error) {
	policies, err := rd.db.ListPoliciesByInstance(instanceID)
	if err != nil {
		return false, err
	}
	now := rd.now().UTC()
	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}
		period, err := riskSchedulePeriod(policy, now)
		if err != nil {
			continue
		}
		referenceTime := policy.CreatedAt.UTC()
		backup, err := rd.db.GetLatestSuccessfulBackup(instanceID, policy.ID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return false, err
			}
		} else {
			referenceTime = riskBackupCompletedAt(backup)
		}
		if overdueSeverity(now.Sub(referenceTime), period) != "" {
			return true, nil
		}
	}
	return false, nil
}

func riskSchedulePeriod(policy model.Policy, now time.Time) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(policy.ScheduleType)) {
	case "interval":
		seconds, err := strconv.Atoi(strings.TrimSpace(policy.ScheduleValue))
		if err != nil || seconds <= 0 {
			return 0, fmt.Errorf("invalid interval %q", policy.ScheduleValue)
		}
		return time.Duration(seconds) * time.Second, nil
	case "cron":
		expr, err := ParseCron(policy.ScheduleValue)
		if err != nil {
			return 0, err
		}
		next := expr.Next(now)
		period := next.Sub(now)
		if period <= 0 {
			return 0, fmt.Errorf("cron expression %q produced non-positive period", policy.ScheduleValue)
		}
		return period, nil
	default:
		return 0, fmt.Errorf("unsupported schedule type %q", policy.ScheduleType)
	}
}

func riskBackupCompletedAt(backup *model.Backup) time.Time {
	if backup == nil {
		return time.Time{}
	}
	if backup.CompletedAt != nil {
		return backup.CompletedAt.UTC()
	}
	if backup.StartedAt != nil {
		return backup.StartedAt.UTC()
	}
	return backup.CreatedAt.UTC()
}

func overdueSeverity(elapsed time.Duration, period time.Duration) string {
	if period <= 0 || elapsed < 2*period {
		return ""
	}
	if elapsed >= 3*period {
		return model.RiskSeverityCritical
	}
	return model.RiskSeverityWarning
}

func capacitySeverity(target *model.BackupTarget) (string, bool) {
	if target == nil || target.TotalCapacityBytes == nil || target.UsedCapacityBytes == nil {
		return "", false
	}
	total := *target.TotalCapacityBytes
	used := *target.UsedCapacityBytes
	if total <= 0 {
		return "", false
	}
	remainingRatio := float64(total-used) / float64(total)
	if remainingRatio < 0.05 {
		return model.RiskSeverityCritical, true
	}
	if remainingRatio < 0.20 {
		return model.RiskSeverityWarning, true
	}
	return "", false
}

func isCredentialError(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	if lower == "" {
		return false
	}
	keywords := []string{
		"unable to authenticate",
		"authentication failed",
		"permission denied",
		"publickey",
		"private key",
		"ssh: handshake failed",
		"no supported methods remain",
		"auth fail",
		"credential",
		"凭证",
		"认证",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func buildRiskNotificationSubject(event *model.RiskEvent, instance *model.Instance, target *model.BackupTarget) string {
	instanceName := "未知实例"
	if instance != nil && strings.TrimSpace(instance.Name) != "" {
		instanceName = instance.Name
	}
	label := riskSourceLabel(event.Source)
	if target != nil && strings.TrimSpace(target.Name) != "" {
		return fmt.Sprintf("[RBS 预警] 实例 %s 关联的备份目标 %s 出现%s风险", instanceName, target.Name, label)
	}
	return fmt.Sprintf("[RBS 预警] 实例 %s 出现%s风险", instanceName, label)
}

func buildRiskNotificationBody(event *model.RiskEvent, instance *model.Instance, target *model.BackupTarget) string {
	instanceName := "未知实例"
	if instance != nil && strings.TrimSpace(instance.Name) != "" {
		instanceName = instance.Name
	}
	targetName := ""
	if target != nil && strings.TrimSpace(target.Name) != "" {
		targetName = target.Name
	}
	createdAt := time.Now().UTC()
	if event != nil && !event.CreatedAt.IsZero() {
		createdAt = event.CreatedAt.UTC()
	}

	lines := []string{
		fmt.Sprintf("实例: %s", instanceName),
		fmt.Sprintf("风险类型: %s", riskSourceLabel(event.Source)),
		fmt.Sprintf("风险等级: %s", riskSeverityLabel(event.Severity)),
		fmt.Sprintf("风险描述: %s", event.Message),
		fmt.Sprintf("时间: %s", createdAt.Format(time.RFC3339)),
	}
	if targetName != "" {
		lines = append(lines, fmt.Sprintf("备份目标: %s", targetName))
	}
	return strings.Join(lines, "\n")
}

func riskSourceLabel(source string) string {
	switch source {
	case model.RiskSourceBackupFailed:
		return "备份失败"
	case model.RiskSourceBackupOverdue:
		return "备份超期"
	case model.RiskSourceColdBackupMissing:
		return "缺少冷备份"
	case model.RiskSourceTargetUnreachable:
		return "目标不可达"
	case model.RiskSourceTargetCapacityLow:
		return "目标容量不足"
	case model.RiskSourceRestoreFailed:
		return "恢复失败"
	case model.RiskSourceCredentialError:
		return "凭证异常"
	default:
		return "系统"
	}
}

func riskSeverityLabel(severity string) string {
	switch severity {
	case model.RiskSeverityCritical:
		return "critical"
	case model.RiskSeverityWarning:
		return "warning"
	case model.RiskSeverityInfo:
		return "info"
	default:
		return severity
	}
}

func riskSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case model.RiskSeverityCritical:
		return 3
	case model.RiskSeverityWarning:
		return 2
	case model.RiskSeverityInfo:
		return 1
	default:
		return 0
	}
}

func mergeAuditDetail(base map[string]any, extra map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func int64Ptr(value int64) *int64 {
	cloned := value
	return &cloned
}
