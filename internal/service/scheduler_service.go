package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"gorm.io/gorm"
)

type StrategyScheduler interface {
	RegisterStrategy(strategy model.Strategy, run func(context.Context) error) error
	RemoveStrategy(strategyID uint) error
	UpcomingRuns(strategyID uint, limit int, now time.Time) []time.Time
}

type StrategyScheduleRefresher interface {
	RefreshStrategy(strategy model.Strategy) error
	RemoveStrategy(strategyID uint) error
}

type StrategySchedulePreviewer interface {
	UpcomingRuns(strategyID uint, limit int, now time.Time) []time.Time
}

type StrategyScheduleCoordinator interface {
	StrategyScheduleRefresher
	StrategySchedulePreviewer
}

type StrategyRunner func(context.Context, model.Strategy) error

type SchedulerService struct {
	scheduler StrategyScheduler
	runner    StrategyRunner
}

func NewSchedulerService(scheduler StrategyScheduler, runner StrategyRunner) *SchedulerService {
	if runner == nil {
		runner = func(context.Context, model.Strategy) error {
			return nil
		}
	}

	return &SchedulerService{
		scheduler: scheduler,
		runner:    runner,
	}
}

func (s *SchedulerService) RefreshStrategy(strategy model.Strategy) error {
	if s == nil || s.scheduler == nil {
		return nil
	}
	if strings.TrimSpace(strategy.BackupType) != BackupTypeRolling {
		return s.RemoveStrategy(strategy.ID)
	}
	if !strategy.Enabled || (strategy.CronExpr == nil && strategy.IntervalSeconds == 0) {
		return s.RemoveStrategy(strategy.ID)
	}

	strategyCopy := strategy
	if err := s.scheduler.RegisterStrategy(strategyCopy, func(ctx context.Context) error {
		return s.runner(ctx, strategyCopy)
	}); err != nil {
		return fmt.Errorf("register strategy schedule: %w", err)
	}

	return nil
}

func (s *SchedulerService) RemoveStrategy(strategyID uint) error {
	if s == nil || s.scheduler == nil {
		return nil
	}

	if err := s.scheduler.RemoveStrategy(strategyID); err != nil {
		return fmt.Errorf("remove strategy schedule: %w", err)
	}

	return nil
}

func (s *SchedulerService) UpcomingRuns(strategyID uint, limit int, now time.Time) []time.Time {
	if s == nil || s.scheduler == nil {
		return nil
	}

	return s.scheduler.UpcomingRuns(strategyID, limit, now)
}

func BootstrapSchedules(ctx context.Context, db *gorm.DB, refresher StrategyScheduleRefresher) error {
	if db == nil || refresher == nil {
		return nil
	}

	var strategies []model.Strategy
	if err := db.WithContext(ctx).
		Preload("StorageTargets").
		Order("id ASC").
		Find(&strategies).Error; err != nil {
		return fmt.Errorf("list persisted strategies for bootstrap: %w", err)
	}
	if err := validatePersistedRollingScheduleIsolation(strategies); err != nil {
		return err
	}

	for _, strategy := range strategies {
		if !strategy.Enabled {
			continue
		}
		if err := refresher.RefreshStrategy(strategy); err != nil {
			return fmt.Errorf("refresh persisted strategy schedule: %w", err)
		}
	}

	return nil
}

func validatePersistedRollingScheduleIsolation(strategies []model.Strategy) error {
	seenLocations := make(map[string]uint)
	conflicts := make([]string, 0)

	for _, strategy := range strategies {
		if strings.TrimSpace(strategy.BackupType) != BackupTypeRolling {
			continue
		}
		for _, target := range strategy.StorageTargets {
			locationKey := fmt.Sprintf("%d|%s", strategy.InstanceID, storageTargetLocationKey(target))
			if existingStrategyID, exists := seenLocations[locationKey]; exists {
				conflicts = append(conflicts, fmt.Sprintf("strategy %d conflicts with strategy %d", existingStrategyID, strategy.ID))
				continue
			}
			seenLocations[locationKey] = strategy.ID
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("persisted rolling schedule isolation conflict: %s", strings.Join(conflicts, "; "))
	}

	return nil
}
