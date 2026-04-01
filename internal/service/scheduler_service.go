package service

import (
	"context"
	"fmt"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
)

type StrategyScheduler interface {
	RegisterStrategy(strategy model.Strategy, run func(context.Context) error) error
	RemoveStrategy(strategyID uint) error
}

type StrategyScheduleRefresher interface {
	RefreshStrategy(strategy model.Strategy) error
	RemoveStrategy(strategyID uint) error
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
