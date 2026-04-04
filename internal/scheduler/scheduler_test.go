package scheduler

import (
	"context"
	"testing"
	"time"

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

func TestSchedulerUpcomingRunsUsesRegisteredIntervalAnchor(t *testing.T) {
	fixedNow := time.Date(2026, time.April, 4, 12, 0, 0, 0, time.UTC)
	scheduler := newScheduler(func() time.Time { return fixedNow })
	defer scheduler.Stop()

	strategy := model.Strategy{
		ID:              11,
		InstanceID:      3,
		IntervalSeconds: 900,
		Enabled:         true,
	}
	if err := scheduler.RegisterStrategy(strategy, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("register interval strategy: %v", err)
	}

	upcoming := scheduler.UpcomingRuns(strategy.ID, 5, fixedNow)
	if len(upcoming) != 5 {
		t.Fatalf("expected 5 upcoming runs, got %d", len(upcoming))
	}

	for index, runAt := range upcoming {
		expected := fixedNow.Add(time.Duration(index+1) * 15 * time.Minute)
		if !runAt.Equal(expected) {
			t.Fatalf("expected run %d at %s, got %s", index, expected.Format(time.RFC3339), runAt.Format(time.RFC3339))
		}
	}
}

func TestSchedulerUpcomingRunsBuildsCronPreview(t *testing.T) {
	fixedNow := time.Date(2026, time.April, 4, 12, 7, 0, 0, time.UTC)
	scheduler := newScheduler(func() time.Time { return fixedNow })
	defer scheduler.Stop()

	strategy := model.Strategy{
		ID:         12,
		InstanceID: 3,
		CronExpr:   stringPtr("0 */15 * * * *"),
		Enabled:    true,
	}
	if err := scheduler.RegisterStrategy(strategy, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("register cron strategy: %v", err)
	}

	upcoming := scheduler.UpcomingRuns(strategy.ID, 3, fixedNow)
	if len(upcoming) != 3 {
		t.Fatalf("expected 3 upcoming runs, got %d", len(upcoming))
	}

	expected := []time.Time{
		time.Date(2026, time.April, 4, 12, 15, 0, 0, time.UTC),
		time.Date(2026, time.April, 4, 12, 30, 0, 0, time.UTC),
		time.Date(2026, time.April, 4, 12, 45, 0, 0, time.UTC),
	}

	for index, expectedRun := range expected {
		if !upcoming[index].Equal(expectedRun) {
			t.Fatalf("expected cron run %d at %s, got %s", index, expectedRun.Format(time.RFC3339), upcoming[index].Format(time.RFC3339))
		}
	}
}

func TestSchedulerUpcomingRunsKeepsDescriptorCronAnchoredToRegistration(t *testing.T) {
	fixedNow := time.Date(2026, time.April, 4, 12, 7, 0, 0, time.UTC)
	scheduler := newScheduler(func() time.Time { return fixedNow })
	defer scheduler.Stop()

	strategy := model.Strategy{
		ID:         13,
		InstanceID: 3,
		CronExpr:   stringPtr("@every 15m"),
		Enabled:    true,
	}
	if err := scheduler.RegisterStrategy(strategy, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("register descriptor cron strategy: %v", err)
	}

	upcoming := scheduler.UpcomingRuns(strategy.ID, 2, fixedNow.Add(5*time.Minute))
	expected := []time.Time{
		time.Date(2026, time.April, 4, 12, 22, 0, 0, time.UTC),
		time.Date(2026, time.April, 4, 12, 37, 0, 0, time.UTC),
	}

	for index, expectedRun := range expected {
		if !upcoming[index].Equal(expectedRun) {
			t.Fatalf("expected descriptor cron run %d at %s, got %s", index, expectedRun.Format(time.RFC3339), upcoming[index].Format(time.RFC3339))
		}
	}
}

func stringPtr(value string) *string {
	return &value
}
