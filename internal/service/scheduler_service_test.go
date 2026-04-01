package service

import (
	"context"
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

func (s *schedulerRefresherSpy) RefreshStrategy(strategy model.Strategy) error {
	s.refreshed = append(s.refreshed, strategy)
	return nil
}

func (s *schedulerRefresherSpy) RemoveStrategy(strategyID uint) error {
	s.removed = append(s.removed, strategyID)
	return nil
}
