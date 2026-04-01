package scheduler

import (
	"context"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

func TestSchedulerRegisterStrategyReplacesExistingRegistration(t *testing.T) {
	scheduler := NewScheduler()
	defer scheduler.Stop()

	first := model.Strategy{
		ID:              1,
		InstanceID:      10,
		IntervalSeconds: 60,
		Enabled:         true,
	}
	if err := scheduler.RegisterStrategy(first, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("register interval strategy: %v", err)
	}

	firstRegistration, ok := scheduler.registry.Get(first.ID)
	if !ok {
		t.Fatal("expected first registration to be stored")
	}
	if firstRegistration.mode != scheduleModeInterval {
		t.Fatalf("expected interval registration mode, got %q", firstRegistration.mode)
	}

	updated := model.Strategy{
		ID:         first.ID,
		InstanceID: first.InstanceID,
		CronExpr:   stringPtr("*/5 * * * * *"),
		Enabled:    true,
	}
	if err := scheduler.RegisterStrategy(updated, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("refresh strategy registration: %v", err)
	}

	updatedRegistration, ok := scheduler.registry.Get(updated.ID)
	if !ok {
		t.Fatal("expected updated registration to exist")
	}
	if updatedRegistration.mode != scheduleModeCron {
		t.Fatalf("expected cron registration mode, got %q", updatedRegistration.mode)
	}
	if updatedRegistration.spec != "*/5 * * * * *" {
		t.Fatalf("expected cron spec to be refreshed, got %q", updatedRegistration.spec)
	}
	if scheduler.registry.Count() != 1 {
		t.Fatalf("expected a single active registration, got %d", scheduler.registry.Count())
	}

	select {
	case <-firstRegistration.stopped:
	default:
		t.Fatal("expected previous registration to be stopped during refresh")
	}
}

func stringPtr(value string) *string {
	return &value
}
