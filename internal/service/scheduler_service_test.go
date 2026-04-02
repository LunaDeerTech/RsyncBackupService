package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"gorm.io/gorm"
)

func TestStrategyServiceCreateRefreshesSchedule(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", "rolling_local")
	refresher := &schedulerRefresherSpy{}
	strategyService := NewStrategyService(fixture.db, refresher)

	strategy, err := strategyService.Create(context.Background(), fixture.actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "hourly",
		BackupType:       BackupTypeRolling,
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	if len(refresher.refreshed) != 1 {
		t.Fatalf("expected one refresh call, got %d", len(refresher.refreshed))
	}
	if refresher.refreshed[0].ID != strategy.ID {
		t.Fatalf("expected refreshed strategy id %d, got %d", strategy.ID, refresher.refreshed[0].ID)
	}
}

func TestStrategyServiceDeleteRemovesSchedule(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", "rolling_local")
	baseService := NewStrategyService(fixture.db)
	strategy, err := baseService.Create(context.Background(), fixture.actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "nightly",
		BackupType:       BackupTypeRolling,
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("seed strategy: %v", err)
	}

	refresher := &schedulerRefresherSpy{}
	strategyService := NewStrategyService(fixture.db, refresher)

	if err := strategyService.Delete(context.Background(), fixture.actor, strategy.ID); err != nil {
		t.Fatalf("delete strategy: %v", err)
	}

	if len(refresher.removed) != 1 {
		t.Fatalf("expected one remove call, got %d", len(refresher.removed))
	}
	if refresher.removed[0] != strategy.ID {
		t.Fatalf("expected removed strategy id %d, got %d", strategy.ID, refresher.removed[0])
	}
}

func TestSchedulerServiceRefreshStrategySkipsColdStrategies(t *testing.T) {
	scheduler := &registerSchedulerSpy{}
	schedulerService := NewSchedulerService(scheduler, nil)
	strategy := model.Strategy{
		ID:              9,
		BackupType:      BackupTypeCold,
		IntervalSeconds: 3600,
		Enabled:         true,
	}

	if err := schedulerService.RefreshStrategy(strategy); err != nil {
		t.Fatalf("refresh strategy: %v", err)
	}
	if len(scheduler.registered) != 0 {
		t.Fatalf("expected cold strategy not to be registered, got %d registrations", len(scheduler.registered))
	}
	if len(scheduler.removed) != 1 || scheduler.removed[0] != strategy.ID {
		t.Fatalf("expected cold strategy removal for id %d, got %v", strategy.ID, scheduler.removed)
	}
}

func TestStorageTargetServiceUpdateRefreshesBoundStrategies(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	target := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target", StorageTargetTypeRollingLocal)
	strategyService := NewStrategyService(fixture.db)
	strategy, err := strategyService.Create(context.Background(), fixture.actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "hourly",
		BackupType:       BackupTypeRolling,
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{target.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create strategy: %v", err)
	}

	refresher := &schedulerRefresherSpy{}
	storageTargetService := NewStorageTargetService(fixture.db, refresher)
	updatedBasePath := filepath.Join(t.TempDir(), "rolling-target-updated")

	updatedTarget, err := storageTargetService.Update(context.Background(), target.ID, UpdateStorageTargetRequest{
		Name:     target.Name,
		Type:     target.Type,
		BasePath: updatedBasePath,
	})
	if err != nil {
		t.Fatalf("update storage target: %v", err)
	}

	if updatedTarget.BasePath != updatedBasePath {
		t.Fatalf("expected updated base path %q, got %q", updatedBasePath, updatedTarget.BasePath)
	}
	if len(refresher.refreshed) != 1 {
		t.Fatalf("expected one schedule refresh, got %d", len(refresher.refreshed))
	}
	if refresher.refreshed[0].ID != strategy.ID {
		t.Fatalf("expected refreshed strategy id %d, got %d", strategy.ID, refresher.refreshed[0].ID)
	}
	if len(refresher.refreshed[0].StorageTargets) != 1 {
		t.Fatalf("expected refreshed strategy to preload one storage target, got %d", len(refresher.refreshed[0].StorageTargets))
	}
	if refresher.refreshed[0].StorageTargets[0].BasePath != updatedBasePath {
		t.Fatalf("expected refreshed schedule to use updated base path %q, got %q", updatedBasePath, refresher.refreshed[0].StorageTargets[0].BasePath)
	}
}

func TestBootstrapSchedulesRefreshesPersistedEnabledStrategies(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	targetA := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target-a", StorageTargetTypeRollingLocal)
	targetB := createStrategyServiceTestStorageTarget(t, fixture.db, "rolling-target-b", StorageTargetTypeRollingLocal)
	strategyService := NewStrategyService(fixture.db)

	enabledStrategy, err := strategyService.Create(context.Background(), fixture.actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "enabled-hourly",
		BackupType:       BackupTypeRolling,
		IntervalSeconds:  3600,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{targetA.ID},
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("create enabled strategy: %v", err)
	}
	if _, err := strategyService.Create(context.Background(), fixture.actor, fixture.instance.ID, CreateStrategyRequest{
		Name:             "disabled-hourly",
		BackupType:       BackupTypeRolling,
		IntervalSeconds:  7200,
		RetentionDays:    7,
		RetentionCount:   3,
		StorageTargetIDs: []uint{targetB.ID},
		Enabled:          false,
	}); err != nil {
		t.Fatalf("create disabled strategy: %v", err)
	}

	refresher := &schedulerRefresherSpy{}
	if err := BootstrapSchedules(context.Background(), fixture.db, refresher); err != nil {
		t.Fatalf("bootstrap schedules: %v", err)
	}

	if len(refresher.refreshed) != 1 {
		t.Fatalf("expected one persisted enabled strategy to be refreshed, got %d", len(refresher.refreshed))
	}
	if refresher.refreshed[0].ID != enabledStrategy.ID {
		t.Fatalf("expected refreshed persisted strategy id %d, got %d", enabledStrategy.ID, refresher.refreshed[0].ID)
	}
}

func TestBootstrapSchedulesRejectsPersistedRollingLocationConflicts(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	sharedBasePath := filepath.Join(t.TempDir(), "shared-rolling-root")
	targetA := model.StorageTarget{Name: "target-a", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	targetB := model.StorageTarget{Name: "target-b", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	if err := fixture.db.Create(&targetA).Error; err != nil {
		t.Fatalf("create target a: %v", err)
	}
	if err := fixture.db.Create(&targetB).Error; err != nil {
		t.Fatalf("create target b: %v", err)
	}

	strategyA := model.Strategy{InstanceID: fixture.instance.ID, Name: "legacy-a", BackupType: BackupTypeRolling, IntervalSeconds: 3600, Enabled: true}
	strategyB := model.Strategy{InstanceID: fixture.instance.ID, Name: "legacy-b", BackupType: BackupTypeRolling, IntervalSeconds: 7200, Enabled: true}
	if err := fixture.db.Create(&strategyA).Error; err != nil {
		t.Fatalf("create strategy a: %v", err)
	}
	if err := fixture.db.Create(&strategyB).Error; err != nil {
		t.Fatalf("create strategy b: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategyA.ID, StorageTargetID: targetA.ID}).Error; err != nil {
		t.Fatalf("create binding a: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategyB.ID, StorageTargetID: targetB.ID}).Error; err != nil {
		t.Fatalf("create binding b: %v", err)
	}

	err := BootstrapSchedules(context.Background(), fixture.db, &schedulerRefresherSpy{})
	if err == nil || !strings.Contains(err.Error(), "isolation conflict") {
		t.Fatalf("expected persisted rolling schedule isolation conflict, got %v", err)
	}
}

func TestBootstrapSchedulesRejectsDuplicateLocationsWithinSingleStrategy(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	sharedBasePath := filepath.Join(t.TempDir(), "single-strategy-shared-root")
	targetA := model.StorageTarget{Name: "target-a", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	targetB := model.StorageTarget{Name: "target-b", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	if err := fixture.db.Create(&targetA).Error; err != nil {
		t.Fatalf("create target a: %v", err)
	}
	if err := fixture.db.Create(&targetB).Error; err != nil {
		t.Fatalf("create target b: %v", err)
	}

	strategy := model.Strategy{InstanceID: fixture.instance.ID, Name: "legacy-duplicate-targets", BackupType: BackupTypeRolling, IntervalSeconds: 3600, Enabled: true}
	if err := fixture.db.Create(&strategy).Error; err != nil {
		t.Fatalf("create strategy: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategy.ID, StorageTargetID: targetA.ID}).Error; err != nil {
		t.Fatalf("create binding a: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategy.ID, StorageTargetID: targetB.ID}).Error; err != nil {
		t.Fatalf("create binding b: %v", err)
	}

	err := BootstrapSchedules(context.Background(), fixture.db, &schedulerRefresherSpy{})
	if err == nil || !strings.Contains(err.Error(), "isolation conflict") {
		t.Fatalf("expected single-strategy rolling schedule isolation conflict, got %v", err)
	}
}

func TestBootstrapSchedulesRejectsDisabledRollingLocationConflicts(t *testing.T) {
	fixture := newStrategyServiceTestFixture(t)
	sharedBasePath := filepath.Join(t.TempDir(), "disabled-shared-root")
	targetA := model.StorageTarget{Name: "target-a", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	targetB := model.StorageTarget{Name: "target-b", Type: StorageTargetTypeRollingLocal, BasePath: sharedBasePath}
	if err := fixture.db.Create(&targetA).Error; err != nil {
		t.Fatalf("create target a: %v", err)
	}
	if err := fixture.db.Create(&targetB).Error; err != nil {
		t.Fatalf("create target b: %v", err)
	}

	strategyA := model.Strategy{InstanceID: fixture.instance.ID, Name: "enabled-strategy", BackupType: BackupTypeRolling, IntervalSeconds: 3600, Enabled: true}
	strategyB := model.Strategy{InstanceID: fixture.instance.ID, Name: "disabled-strategy", BackupType: BackupTypeRolling, IntervalSeconds: 7200, Enabled: false}
	if err := fixture.db.Create(&strategyA).Error; err != nil {
		t.Fatalf("create strategy a: %v", err)
	}
	if err := fixture.db.Create(&strategyB).Error; err != nil {
		t.Fatalf("create strategy b: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategyA.ID, StorageTargetID: targetA.ID}).Error; err != nil {
		t.Fatalf("create binding a: %v", err)
	}
	if err := fixture.db.Create(&model.StrategyStorageBinding{StrategyID: strategyB.ID, StorageTargetID: targetB.ID}).Error; err != nil {
		t.Fatalf("create binding b: %v", err)
	}

	err := BootstrapSchedules(context.Background(), fixture.db, &schedulerRefresherSpy{})
	if err == nil || !strings.Contains(err.Error(), "isolation conflict") {
		t.Fatalf("expected disabled rolling schedule isolation conflict, got %v", err)
	}
}

type strategyServiceTestFixture struct {
	db       *gorm.DB
	actor    AuthIdentity
	instance model.BackupInstance
}

func newStrategyServiceTestFixture(t *testing.T) strategyServiceTestFixture {
	t.Helper()

	db, err := repository.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := repository.MigrateAndSeed(db, config.Config{AdminUser: "admin", AdminPassword: "secret"}); err != nil {
		t.Fatalf("migrate and seed: %v", err)
	}

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}

	instance := model.BackupInstance{
		Name:            "instance-a",
		SourceType:      "local",
		SourcePath:      "/srv/source",
		ExcludePatterns: "[]",
		Enabled:         true,
		CreatedBy:       admin.ID,
	}
	if err := db.Create(&instance).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	return strategyServiceTestFixture{
		db: db,
		actor: AuthIdentity{
			UserID:   admin.ID,
			Username: admin.Username,
			IsAdmin:  admin.IsAdmin,
		},
		instance: instance,
	}
}

func createStrategyServiceTestStorageTarget(t *testing.T, db *gorm.DB, name, storageType string) model.StorageTarget {
	t.Helper()

	target := model.StorageTarget{
		Name:     name,
		Type:     storageType,
		BasePath: t.TempDir(),
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create storage target: %v", err)
	}

	return target
}

type schedulerRefresherSpy struct {
	refreshed []model.Strategy
	removed   []uint
}

type registerSchedulerSpy struct {
	registered []model.Strategy
	removed    []uint
}

func (s *registerSchedulerSpy) RegisterStrategy(strategy model.Strategy, _ func(context.Context) error) error {
	s.registered = append(s.registered, strategy)
	return nil
}

func (s *registerSchedulerSpy) RemoveStrategy(strategyID uint) error {
	s.removed = append(s.removed, strategyID)
	return nil
}

func (s *schedulerRefresherSpy) RefreshStrategy(strategy model.Strategy) error {
	s.refreshed = append(s.refreshed, strategy)
	return nil
}

func (s *schedulerRefresherSpy) RemoveStrategy(strategyID uint) error {
	s.removed = append(s.removed, strategyID)
	return nil
}
