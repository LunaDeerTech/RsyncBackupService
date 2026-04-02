package service

import (
	"context"
	"testing"

	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/notify"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
)

func TestExecutorServiceRunStrategyDispatchesCompletionNotification(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	strategy := model.Strategy{
		InstanceID:      fixture.instance.ID,
		Name:            "daily",
		BackupType:      BackupTypeRolling,
		IntervalSeconds: 3600,
		RetentionCount:  1,
		Enabled:         true,
	}
	if err := fixture.db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}
	if err := fixture.db.Model(&strategy).Association("StorageTargets").Replace([]model.StorageTarget{target}); err != nil {
		t.Fatalf("bind strategy storage target: %v", err)
	}
	strategy.StorageTargets = []model.StorageTarget{target}

	spy := &notificationDispatcherSpy{}
	service := NewExecutorService(fixture.db, config.Config{DataDir: t.TempDir()}, successfulRunner{}, executorpkg.NewTaskManager(), spy)
	service.retentionService = retentionCleanerFunc(func(context.Context, model.Strategy, model.StorageTarget) error { return nil })

	if err := service.RunStrategy(context.Background(), strategy); err != nil {
		t.Fatalf("run strategy: %v", err)
	}
	if len(spy.events) != 1 {
		t.Fatalf("expected one completion notification, got %d", len(spy.events))
	}
	if spy.events[0].Type != "backup_success" || spy.events[0].Instance != fixture.instance.Name {
		t.Fatalf("unexpected backup notification event: %+v", spy.events[0])
	}
}

func TestRestoreServiceStartDispatchesCompletionNotification(t *testing.T) {
	fixture := newRestoreServiceTestFixture(t)
	spy := &notificationDispatcherSpy{}
	fixture.service.notificationDispatcher = spy

	if _, err := fixture.service.Start(context.Background(), RestoreRequest{
		InstanceID:        fixture.instance.ID,
		BackupRecordID:    fixture.backupRecord.ID,
		RestoreTargetPath: t.TempDir(),
		Overwrite:         false,
		VerifyToken:       fixture.issueVerifyToken(t),
		TriggeredBy:       fixture.admin.ID,
	}); err != nil {
		t.Fatalf("start restore: %v", err)
	}
	if len(spy.events) != 1 {
		t.Fatalf("expected one restore completion notification, got %d", len(spy.events))
	}
	if spy.events[0].Type != "restore_complete" || spy.events[0].Instance != fixture.instance.Name {
		t.Fatalf("unexpected restore notification event: %+v", spy.events[0])
	}
}

type notificationDispatcherSpy struct {
	events []notify.NotifyEvent
}

func (s *notificationDispatcherSpy) Notify(ctx context.Context, event notify.NotifyEvent) error {
	_ = ctx
	s.events = append(s.events, event)
	return nil
}