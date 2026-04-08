package engine

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

func TestRiskDetectorBackupFailureLifecycleAndCacheInvalidation(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	disasterRecovery := service.NewDisasterRecoveryService(db)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	disasterRecovery.SetClock(func() time.Time { return now })
	if _, err := disasterRecovery.GetScore(context.Background(), instance.ID); err != nil {
		t.Fatalf("GetScore() error = %v", err)
	}
	if _, ok := disasterRecovery.Cache().Get(instance.ID); !ok {
		t.Fatal("cache miss before risk detection, want populated cache")
	}

	detector := NewRiskDetector(db, disasterRecovery.Cache(), audit.NewLogger(db))
	detector.SetClock(func() time.Time { return now })

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-3*time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(first) error = %v", err)
	}
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed, model.RiskSeverityWarning)
	if _, ok := disasterRecovery.Cache().Get(instance.ID); ok {
		t.Fatal("cache still populated after risk creation, want invalidated")
	}

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-2*time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(second) error = %v", err)
	}
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed, model.RiskSeverityWarning)

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(third) error = %v", err)
	}
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed, model.RiskSeverityCritical)

	createRiskTestBackup(t, db, instance.ID, policy.ID, "success", now, "")
	if err := detector.OnBackupSuccess(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupSuccess() error = %v", err)
	}
	assertNoActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupFailed)
}

func TestRiskDetectorBackupFailureDetectsCredentialError(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	detector := NewRiskDetector(db, nil, audit.NewLogger(db))

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC), "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed() error = %v", err)
	}

	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceCredentialError, model.RiskSeverityCritical)
}

func TestRiskDetectorBackupSuccessResolvesOverdueRisk(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	now := policy.CreatedAt.Add(4 * time.Hour)
	detector := NewRiskDetector(db, nil, audit.NewLogger(db))
	detector.SetClock(func() time.Time { return now })

	if err := detector.PeriodicCheck(context.Background()); err != nil {
		t.Fatalf("PeriodicCheck(create overdue) error = %v", err)
	}
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupOverdue, model.RiskSeverityCritical)

	createRiskTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-30*time.Minute), "")
	if err := detector.OnBackupSuccess(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupSuccess() error = %v", err)
	}
	assertNoActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceBackupOverdue)
}

func TestRiskDetectorPeriodicCheckDetectsColdBackupMissingAndResolves(t *testing.T) {
	db := newRollingTestDB(t)
	instance, _, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	detector := NewRiskDetector(db, nil, audit.NewLogger(db))

	if err := detector.PeriodicCheck(context.Background()); err != nil {
		t.Fatalf("PeriodicCheck(create cold missing) error = %v", err)
	}
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceColdBackupMissing, model.RiskSeverityInfo)

	coldTarget := &model.BackupTarget{
		Name:          "cold-target",
		BackupType:    "cold",
		StorageType:   "local",
		StoragePath:   t.TempDir(),
		HealthStatus:  "healthy",
		HealthMessage: "ok",
	}
	if err := db.CreateBackupTarget(coldTarget); err != nil {
		t.Fatalf("CreateBackupTarget(cold) error = %v", err)
	}
	coldPolicy := &model.Policy{
		InstanceID:     instance.ID,
		Name:           "daily-cold",
		Type:           "cold",
		TargetID:       coldTarget.ID,
		ScheduleType:   "interval",
		ScheduleValue:  "86400",
		Enabled:        true,
		RetentionType:  "count",
		RetentionValue: 3,
	}
	if err := db.CreatePolicy(coldPolicy); err != nil {
		t.Fatalf("CreatePolicy(cold) error = %v", err)
	}

	if err := detector.PeriodicCheck(context.Background()); err != nil {
		t.Fatalf("PeriodicCheck(resolve cold missing) error = %v", err)
	}
	assertNoActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceColdBackupMissing)
}

func TestRiskDetectorHealthAndCapacityLifecycle(t *testing.T) {
	db := newRollingTestDB(t)
	instance, _, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	disasterRecovery := service.NewDisasterRecoveryService(db)
	if _, err := disasterRecovery.GetScore(context.Background(), instance.ID); err != nil {
		t.Fatalf("GetScore() error = %v", err)
	}
	detector := NewRiskDetector(db, disasterRecovery.Cache(), audit.NewLogger(db))

	if err := db.UpdateHealthStatus(target.ID, "unreachable", "local path does not exist", nil, nil); err != nil {
		t.Fatalf("UpdateHealthStatus(unreachable) error = %v", err)
	}
	if err := detector.OnHealthCheckComplete(context.Background(), target.ID, "unreachable"); err != nil {
		t.Fatalf("OnHealthCheckComplete(unreachable) error = %v", err)
	}
	assertActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetUnreachable, model.RiskSeverityCritical)
	if _, ok := disasterRecovery.Cache().Get(instance.ID); ok {
		t.Fatal("cache still populated after target risk change, want invalidated")
	}

	total := int64(100)
	usedWarning := int64(85)
	if err := db.UpdateHealthStatus(target.ID, "healthy", "local target is healthy", &total, &usedWarning); err != nil {
		t.Fatalf("UpdateHealthStatus(warning) error = %v", err)
	}
	if err := detector.OnHealthCheckComplete(context.Background(), target.ID, "healthy"); err != nil {
		t.Fatalf("OnHealthCheckComplete(warning) error = %v", err)
	}
	assertNoActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetUnreachable)
	assertActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetCapacityLow, model.RiskSeverityWarning)

	usedCritical := int64(97)
	if err := db.UpdateHealthStatus(target.ID, "healthy", "local target is healthy", &total, &usedCritical); err != nil {
		t.Fatalf("UpdateHealthStatus(critical) error = %v", err)
	}
	if err := detector.OnHealthCheckComplete(context.Background(), target.ID, "healthy"); err != nil {
		t.Fatalf("OnHealthCheckComplete(critical) error = %v", err)
	}
	assertActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetCapacityLow, model.RiskSeverityCritical)

	usedRecovered := int64(60)
	if err := db.UpdateHealthStatus(target.ID, "healthy", "local target is healthy", &total, &usedRecovered); err != nil {
		t.Fatalf("UpdateHealthStatus(recovered) error = %v", err)
	}
	if err := detector.OnHealthCheckComplete(context.Background(), target.ID, "healthy"); err != nil {
		t.Fatalf("OnHealthCheckComplete(recovered) error = %v", err)
	}
	assertNoActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetCapacityLow)
}

func TestRiskDetectorRestoreFailedCreatesRestoreAndCredentialRisks(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	backup := insertSuccessBackupWithTask(t, db, instance.ID, policy.ID, "rolling", t.TempDir(), time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC))
	task := createRestoreTask(t, db, instance.ID, backup.ID, "source", "")
	completedAt := time.Date(2026, 4, 7, 12, 10, 0, 0, time.UTC)
	task.Status = "failed"
	task.CompletedAt = &completedAt
	task.ErrorMessage = "ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"
	if err := db.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask(restore failed) error = %v", err)
	}

	detector := NewRiskDetector(db, nil, audit.NewLogger(db))
	if err := detector.OnRestoreFailed(context.Background(), instance.ID); err != nil {
		t.Fatalf("OnRestoreFailed() error = %v", err)
	}

	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceRestoreFailed, model.RiskSeverityCritical)
	assertActiveRiskEvent(t, db, &instance.ID, nil, model.RiskSourceCredentialError, model.RiskSeverityCritical)
}

func TestRiskDetectorSendsNotificationsOnCreateAndCriticalEscalation(t *testing.T) {
	db := newRollingTestDB(t)
	instance, policy, _, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	user := &model.User{Email: "viewer@example.com", Name: "Viewer", PasswordHash: "hash", Role: "viewer"}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(viewer) error = %v", err)
	}
	if err := db.UpdateSubscriptions(user.ID, []model.NotificationSubscription{{InstanceID: instance.ID, Enabled: true}}); err != nil {
		t.Fatalf("UpdateSubscriptions() error = %v", err)
	}

	sender := &recordingEmailSender{}
	detector := NewRiskDetector(db, nil, audit.NewLogger(db))
	detector.SetEmailSender(sender)
	now := time.Date(2026, 4, 8, 8, 0, 0, 0, time.UTC)

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-3*time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(first) error = %v", err)
	}
	if len(sender.jobs) != 1 {
		t.Fatalf("notification count after create = %d, want %d", len(sender.jobs), 1)
	}

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-2*time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(second) error = %v", err)
	}
	if len(sender.jobs) != 1 {
		t.Fatalf("notification count after second failure = %d, want %d", len(sender.jobs), 1)
	}

	createRiskTestBackup(t, db, instance.ID, policy.ID, "failed", now.Add(-time.Minute), "rsync failed")
	if err := detector.OnBackupFailed(context.Background(), instance.ID, policy.ID); err != nil {
		t.Fatalf("OnBackupFailed(third) error = %v", err)
	}
	if len(sender.jobs) != 2 {
		t.Fatalf("notification count after critical escalation = %d, want %d", len(sender.jobs), 2)
	}
	if sender.jobs[1].to != user.Email || !strings.Contains(sender.jobs[1].subject, "备份失败") || !strings.Contains(sender.jobs[1].body, "critical") {
		t.Fatalf("critical notification = %+v, want backup-failed critical email", sender.jobs[1])
	}
}

func createRiskTestBackup(t *testing.T, db *store.DB, instanceID, policyID int64, status string, completedAt time.Time, errorMessage string) *model.Backup {
	t.Helper()
	startedAt := completedAt.Add(-5 * time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policyID,
		TriggerSource:   model.BackupTriggerSourceScheduled,
		Type:            "rolling",
		Status:          status,
		SnapshotPath:    t.TempDir(),
		BackupSizeBytes: 100,
		ActualSizeBytes: 80,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: int64(completedAt.Sub(startedAt).Seconds()),
		ErrorMessage:    errorMessage,
		RsyncStats:      "{}",
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup(%s) error = %v", status, err)
	}
	return backup
}

func assertActiveRiskEvent(t *testing.T, db *store.DB, instanceID *int64, targetID *int64, source string, severity string) *model.RiskEvent {
	t.Helper()
	event, err := db.GetActiveRiskEvent(instanceID, targetID, source)
	if err != nil {
		t.Fatalf("GetActiveRiskEvent(%s) error = %v", source, err)
	}
	if event.Severity != severity {
		t.Fatalf("risk event %s severity = %q, want %q", source, event.Severity, severity)
	}
	return event
}

func assertNoActiveRiskEvent(t *testing.T, db *store.DB, instanceID *int64, targetID *int64, source string) {
	t.Helper()
	_, err := db.GetActiveRiskEvent(instanceID, targetID, source)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetActiveRiskEvent(%s) error = %v, want sql.ErrNoRows", source, err)
	}
}

type recordingEmailSender struct {
	jobs []recordedEmailJob
}

type recordedEmailJob struct {
	to      string
	subject string
	body    string
}

func (s *recordingEmailSender) Send(to, subject, body string) {
	s.jobs = append(s.jobs, recordedEmailJob{to: to, subject: subject, body: body})
}
