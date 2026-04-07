package service

import (
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestDRCalculatorCalculateNoPolicies(t *testing.T) {
	db := newDRTestDB(t)
	calculator := NewDRCalculator(db)
	calculator.now = func() time.Time { return time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC) }

	instance := createDRTestInstance(t, db, "no-policy")
	score, err := calculator.Calculate(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if score.Total != 0 {
		t.Fatalf("Total = %v, want 0", score.Total)
	}
	if score.Level != "danger" {
		t.Fatalf("Level = %q, want danger", score.Level)
	}
}

func TestDRCalculatorCalculateFreshness(t *testing.T) {
	db := newDRTestDB(t)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	calculator := NewDRCalculator(db)
	calculator.now = func() time.Time { return now }

	instance := createDRTestInstance(t, db, "freshness")
	target := createDRTestTarget(t, db, "freshness-target", "local", "healthy")
	policy := createDRTestPolicy(t, db, instance.ID, target.ID, "hourly", "interval", "3600", "count", 3)
	createDRTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-90*time.Minute))

	score, reasons, err := calculator.calculateFreshness(instance.ID, []model.Policy{*policy}, now)
	if err != nil {
		t.Fatalf("calculateFreshness() error = %v", err)
	}
	if math.Abs(score-80) > 0.01 {
		t.Fatalf("Freshness = %v, want 80", score)
	}
	if len(reasons) != 0 {
		t.Fatalf("Freshness reasons = %v, want empty", reasons)
	}
}

func TestDRCalculatorCalculateRecoveryPoints(t *testing.T) {
	db := newDRTestDB(t)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	calculator := NewDRCalculator(db)
	calculator.now = func() time.Time { return now }

	instance := createDRTestInstance(t, db, "recovery")
	target := createDRTestTarget(t, db, "recovery-target", "local", "healthy")
	policy := createDRTestPolicy(t, db, instance.ID, target.ID, "count-retention", "interval", "3600", "count", 3)
	createDRTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-30*time.Minute))
	createDRTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-2*time.Hour))

	score, reasons, err := calculator.calculateRecoveryPoints([]model.Policy{*policy}, now)
	if err != nil {
		t.Fatalf("calculateRecoveryPoints() error = %v", err)
	}
	if math.Abs(score-90) > 0.01 {
		t.Fatalf("RecoveryPoints = %v, want 90", score)
	}
	if len(reasons) != 0 {
		t.Fatalf("Recovery reasons = %v, want empty", reasons)
	}
}

func TestDRCalculatorCalculateRedundancy(t *testing.T) {
	db := newDRTestDB(t)
	calculator := NewDRCalculator(db)

	instance := createDRTestInstance(t, db, "redundancy")
	localTarget := createDRTestTarget(t, db, "local-target", "local", "healthy")
	localOnlyPolicy := createDRTestPolicy(t, db, instance.ID, localTarget.ID, "local-only", "interval", "3600", "count", 3)

	localScore, _, _, err := calculator.calculateRedundancy([]model.Policy{*localOnlyPolicy})
	if err != nil {
		t.Fatalf("calculateRedundancy(local) error = %v", err)
	}
	if math.Abs(localScore-40) > 0.01 {
		t.Fatalf("local redundancy score = %v, want 40", localScore)
	}

	sshTarget := createDRTestTarget(t, db, "ssh-target", "ssh", "healthy")
	sshPolicy := createDRTestPolicy(t, db, instance.ID, sshTarget.ID, "remote", "interval", "7200", "count", 3)
	remoteScore, _, _, err := calculator.calculateRedundancy([]model.Policy{*localOnlyPolicy, *sshPolicy})
	if err != nil {
		t.Fatalf("calculateRedundancy(remote) error = %v", err)
	}
	if remoteScore <= localScore {
		t.Fatalf("remote redundancy score = %v, want greater than %v", remoteScore, localScore)
	}
	if math.Abs(remoteScore-100) > 0.01 {
		t.Fatalf("remote redundancy score = %v, want 100", remoteScore)
	}
}

func TestDRCalculatorCalculateStability(t *testing.T) {
	t.Run("success rate", func(t *testing.T) {
		db := newDRTestDB(t)
		calculator := NewDRCalculator(db)
		instance := createDRTestInstance(t, db, "stability-success")
		target := createDRTestTarget(t, db, "stability-target", "local", "healthy")
		policy := createDRTestPolicy(t, db, instance.ID, target.ID, "stability", "interval", "3600", "count", 5)
		createDRTestBackup(t, db, instance.ID, policy.ID, "success", time.Date(2026, 4, 7, 11, 55, 0, 0, time.UTC))
		createDRTestBackup(t, db, instance.ID, policy.ID, "success", time.Date(2026, 4, 7, 11, 45, 0, 0, time.UTC))
		createDRTestBackup(t, db, instance.ID, policy.ID, "failed", time.Date(2026, 4, 7, 11, 35, 0, 0, time.UTC))
		createDRTestBackup(t, db, instance.ID, policy.ID, "success", time.Date(2026, 4, 7, 11, 25, 0, 0, time.UTC))
		createDRTestBackup(t, db, instance.ID, policy.ID, "success", time.Date(2026, 4, 7, 11, 15, 0, 0, time.UTC))

		score, reasons, err := calculator.calculateStability(instance.ID, map[int64]*model.BackupTarget{target.ID: target})
		if err != nil {
			t.Fatalf("calculateStability() error = %v", err)
		}
		if math.Abs(score-84) > 0.01 {
			t.Fatalf("Stability = %v, want 84", score)
		}
		if len(reasons) != 0 {
			t.Fatalf("Stability reasons = %v, want empty", reasons)
		}
	})

	t.Run("blocking risk", func(t *testing.T) {
		db := newDRTestDB(t)
		calculator := NewDRCalculator(db)
		instance := createDRTestInstance(t, db, "stability-risk")
		target := createDRTestTarget(t, db, "risk-target", "ssh", "unreachable")
		target.HealthMessage = "remote path is unavailable or not writable"
		if err := db.UpdateBackupTarget(target); err != nil {
			t.Fatalf("UpdateBackupTarget() error = %v", err)
		}

		score, reasons, err := calculator.calculateStability(instance.ID, map[int64]*model.BackupTarget{target.ID: target})
		if err != nil {
			t.Fatalf("calculateStability() error = %v", err)
		}
		if score != 0 {
			t.Fatalf("Stability = %v, want 0", score)
		}
		if len(reasons) == 0 {
			t.Fatal("Stability reasons = empty, want blocking risk reason")
		}
	})
}

func TestDisasterRecoveryServiceCache(t *testing.T) {
	db := newDRTestDB(t)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	service := NewDisasterRecoveryService(db)
	service.SetClock(func() time.Time { return now })

	instance := createDRTestInstance(t, db, "cache")
	target := createDRTestTarget(t, db, "cache-target", "local", "healthy")
	policy := createDRTestPolicy(t, db, instance.ID, target.ID, "cache-policy", "interval", "3600", "count", 3)
	createDRTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-30*time.Minute))

	first, err := service.GetScore(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetScore(first) error = %v", err)
	}

	service.SetClock(func() time.Time { return now.Add(2 * time.Minute) })
	second, err := service.GetScore(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetScore(second) error = %v", err)
	}
	if !first.CalculatedAt.Equal(second.CalculatedAt) {
		t.Fatalf("CalculatedAt changed within cache TTL: first=%s second=%s", first.CalculatedAt, second.CalculatedAt)
	}

	service.SetClock(func() time.Time { return now.Add(6 * time.Minute) })
	third, err := service.GetScore(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("GetScore(third) error = %v", err)
	}
	if !third.CalculatedAt.After(second.CalculatedAt) {
		t.Fatalf("CalculatedAt after TTL = %s, want after %s", third.CalculatedAt, second.CalculatedAt)
	}

	service.Invalidate(instance.ID)
	if _, ok := service.cache.Get(instance.ID); ok {
		t.Fatal("cache still contains instance after Invalidate()")
	}
}

func TestDRCalculatorCalculateWithSuccessfulBackupHasPositiveScore(t *testing.T) {
	db := newDRTestDB(t)
	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	calculator := NewDRCalculator(db)
	calculator.now = func() time.Time { return now }

	instance := createDRTestInstance(t, db, "positive")
	target := createDRTestTarget(t, db, "positive-target", "local", "healthy")
	policy := createDRTestPolicy(t, db, instance.ID, target.ID, "positive-policy", "interval", "3600", "count", 3)
	createDRTestBackup(t, db, instance.ID, policy.ID, "success", now.Add(-15*time.Minute))

	score, err := calculator.Calculate(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if score.Total <= 0 {
		t.Fatalf("Total = %v, want > 0", score.Total)
	}
	wantTotal := roundScore(0.35*score.Freshness + 0.30*score.RecoveryPoints + 0.20*score.Redundancy + 0.15*score.Stability)
	if score.Total != wantTotal {
		t.Fatalf("Total = %v, want %v from weighted formula", score.Total, wantTotal)
	}
}

func newDRTestDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("db.Close() error = %v", err)
		}
	})
	if err := db.Migrate(); err != nil {
		t.Fatalf("db.Migrate() error = %v", err)
	}
	return db
}

func createDRTestInstance(t *testing.T, db *store.DB, name string) *model.Instance {
	t.Helper()
	instance := &model.Instance{Name: name, SourceType: "local", SourcePath: "/srv/" + name, Status: "idle"}
	if err := db.CreateInstance(instance); err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}
	return instance
}

func createDRTestTarget(t *testing.T, db *store.DB, name, storageType, healthStatus string) *model.BackupTarget {
	t.Helper()
	target := &model.BackupTarget{
		Name:          name,
		BackupType:    "rolling",
		StorageType:   storageType,
		StoragePath:   "/backup/" + name,
		HealthStatus:  healthStatus,
		HealthMessage: "ok",
	}
	if healthStatus != "healthy" {
		target.HealthMessage = "target unreachable"
	}
	if err := db.CreateBackupTarget(target); err != nil {
		t.Fatalf("CreateBackupTarget() error = %v", err)
	}
	return target
}

func createDRTestPolicy(t *testing.T, db *store.DB, instanceID, targetID int64, name, scheduleType, scheduleValue, retentionType string, retentionValue int) *model.Policy {
	t.Helper()
	policy := &model.Policy{
		InstanceID:     instanceID,
		Name:           name,
		Type:           "rolling",
		TargetID:       targetID,
		ScheduleType:   scheduleType,
		ScheduleValue:  scheduleValue,
		Enabled:        true,
		RetentionType:  retentionType,
		RetentionValue: retentionValue,
	}
	if err := db.CreatePolicy(policy); err != nil {
		t.Fatalf("CreatePolicy() error = %v", err)
	}
	return policy
}

func createDRTestBackup(t *testing.T, db *store.DB, instanceID, policyID int64, status string, completedAt time.Time) *model.Backup {
	t.Helper()
	startedAt := completedAt.Add(-5 * time.Minute)
	backup := &model.Backup{
		InstanceID:      instanceID,
		PolicyID:        policyID,
		TriggerSource:   model.BackupTriggerSourceScheduled,
		Type:            "rolling",
		Status:          status,
		SnapshotPath:    "/backup/snapshots/" + strconv.FormatInt(policyID, 10),
		BackupSizeBytes: 128,
		ActualSizeBytes: 64,
		StartedAt:       &startedAt,
		CompletedAt:     &completedAt,
		DurationSeconds: int64(completedAt.Sub(startedAt).Seconds()),
		ErrorMessage:    "",
		RsyncStats:      "{}",
	}
	if status != "success" {
		backup.ErrorMessage = "backup failed"
	}
	if err := db.CreateBackup(backup); err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	return backup
}
