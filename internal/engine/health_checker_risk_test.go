package engine

import (
	"testing"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/model"
)

func TestHealthCheckerCheckAllCreatesTargetUnreachableRiskEvent(t *testing.T) {
	db := newRollingTestDB(t)
	_, _, target, _, _ := createRollingFixtures(t, db, t.TempDir(), t.TempDir())
	target.StoragePath = t.TempDir() + "/missing"
	if err := db.UpdateBackupTarget(target); err != nil {
		t.Fatalf("UpdateBackupTarget() error = %v", err)
	}

	checker := NewHealthChecker(db)
	checker.SetRiskDetector(NewRiskDetector(db, nil, audit.NewLogger(db)))
	checker.CheckAll()

	assertActiveRiskEvent(t, db, nil, &target.ID, model.RiskSourceTargetUnreachable, model.RiskSeverityCritical)
}
