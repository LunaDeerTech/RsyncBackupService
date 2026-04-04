package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/robfig/cron/v3"
)

var cronParser = cron.NewParser(
	cron.SecondOptional |
		cron.Minute |
		cron.Hour |
		cron.Dom |
		cron.Month |
		cron.Dow |
		cron.Descriptor,
)

type Scheduler struct {
	mu       sync.Mutex
	cron     *cron.Cron
	registry *registry
	logger   *log.Logger
	now      func() time.Time
	stopOnce sync.Once
}

func NewScheduler() *Scheduler {
	return newScheduler(time.Now)
}

func newScheduler(now func() time.Time) *Scheduler {
	if now == nil {
		now = time.Now
	}

	instance := &Scheduler{
		cron:     cron.New(cron.WithParser(cronParser)),
		registry: newRegistry(),
		logger:   log.Default(),
		now:      now,
	}
	instance.cron.Start()

	return instance
}

func ParseCronExpression(expr string) error {
	trimmedExpr := strings.TrimSpace(expr)
	if trimmedExpr == "" {
		return fmt.Errorf("cron expression is empty")
	}

	if _, err := cronParser.Parse(trimmedExpr); err != nil {
		return err
	}

	return nil
}

func (s *Scheduler) RegisterStrategy(strategy model.Strategy, run func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmedCronExpr := normalizeOptionalString(strategy.CronExpr)
	if trimmedCronExpr != nil && strategy.IntervalSeconds > 0 {
		return fmt.Errorf("cron_expr and interval_seconds are mutually exclusive")
	}
	if trimmedCronExpr == nil && strategy.IntervalSeconds == 0 {
		return fmt.Errorf("either cron_expr or interval_seconds is required")
	}
	if strategy.IntervalSeconds < 0 {
		return fmt.Errorf("interval_seconds must be >= 0")
	}
	if trimmedCronExpr != nil {
		if err := ParseCronExpression(*trimmedCronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}
	if !strategy.Enabled {
		return s.removeStrategyLocked(strategy.ID)
	}
	if err := s.removeStrategyLocked(strategy.ID); err != nil {
		return err
	}

	entry, err := s.buildRegistration(strategy.ID, trimmedCronExpr, strategy.IntervalSeconds, run)
	if err != nil {
		return err
	}

	s.registry.Replace(entry)

	return nil
}

func (s *Scheduler) RemoveStrategy(strategyID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removeStrategyLocked(strategyID)
}

func (s *Scheduler) removeStrategyLocked(strategyID uint) error {
	entry, exists := s.registry.Remove(strategyID)
	if !exists {
		return nil
	}

	if entry.stop != nil {
		entry.stop()
	}

	return nil
}

func (s *Scheduler) UpcomingRuns(strategyID uint, limit int, now time.Time) []time.Time {
	if limit <= 0 {
		return nil
	}

	entry, exists := s.registry.Get(strategyID)
	if !exists {
		return nil
	}

	referenceTime := now.UTC()
	if referenceTime.IsZero() {
		referenceTime = s.now().UTC()
	}

	switch entry.mode {
	case scheduleModeCron:
		if entry.schedule == nil || entry.nextRunAt.IsZero() {
			return nil
		}

		runs := make([]time.Time, 0, limit)
		nextRunAt := entry.nextRunAt.UTC()
		for !nextRunAt.After(referenceTime) {
			nextRunAt = entry.schedule.Next(nextRunAt)
		}

		for len(runs) < limit {
			if nextRunAt.IsZero() {
				break
			}
			runs = append(runs, nextRunAt.UTC())
			nextRunAt = entry.schedule.Next(nextRunAt)
		}

		return runs
	case scheduleModeInterval:
		if entry.interval <= 0 || entry.nextRunAt.IsZero() {
			return nil
		}

		nextRunAt := entry.nextRunAt.UTC()
		for !nextRunAt.After(referenceTime) {
			nextRunAt = nextRunAt.Add(entry.interval)
		}

		runs := make([]time.Time, 0, limit)
		for len(runs) < limit {
			runs = append(runs, nextRunAt)
			nextRunAt = nextRunAt.Add(entry.interval)
		}

		return runs
	default:
		return nil
	}
}

func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		entries := s.registry.Drain()
		for _, entry := range entries {
			if entry.stop != nil {
				entry.stop()
			}
		}

		if s.cron != nil {
			ctx := s.cron.Stop()
			<-ctx.Done()
		}
	})
}

func (s *Scheduler) buildRegistration(strategyID uint, cronExpr *string, intervalSeconds int, run func(context.Context) error) (registration, error) {
	if cronExpr != nil {
		schedule, err := cronParser.Parse(strings.TrimSpace(*cronExpr))
		if err != nil {
			return registration{}, fmt.Errorf("register cron strategy: %w", err)
		}

		nextRunAt := schedule.Next(s.now().UTC())
		upcoming := nextRunAt
		entryID, err := s.cron.AddFunc(*cronExpr, func() {
			upcoming = schedule.Next(upcoming)
			s.registry.UpdateNextRun(strategyID, upcoming)
			s.execute(run)
		})
		if err != nil {
			return registration{}, fmt.Errorf("register cron strategy: %w", err)
		}

		return newRegistration(strategyID, scheduleModeCron, *cronExpr, schedule, 0, nextRunAt, func() {
			s.cron.Remove(entryID)
		}), nil
	}

	interval := time.Duration(intervalSeconds) * time.Second
	if interval <= 0 {
		return registration{}, fmt.Errorf("interval_seconds must be greater than zero")
	}

	ctx, cancel := context.WithCancel(context.Background())
	var stopRequested uint32
	done := make(chan struct{})
	nextRunAt := s.now().UTC().Add(interval)
	go func() {
		defer close(done)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		upcoming := nextRunAt

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if atomic.LoadUint32(&stopRequested) != 0 {
					return
				}
				upcoming = upcoming.Add(interval)
				s.registry.UpdateNextRun(strategyID, upcoming)
				s.execute(run)
			}
		}
	}()

	return newRegistration(strategyID, scheduleModeInterval, interval.String(), nil, interval, nextRunAt, func() {
		atomic.StoreUint32(&stopRequested, 1)
		cancel()
		<-done
	}), nil
}

func (s *Scheduler) execute(run func(context.Context) error) {
	if run == nil {
		return
	}

	if err := run(context.Background()); err != nil {
		s.logger.Printf("scheduler run failed: %v", err)
	}
}

func newRegistration(strategyID uint, mode scheduleMode, spec string, schedule cron.Schedule, interval time.Duration, nextRunAt time.Time, stopFn func()) registration {
	stopped := make(chan struct{})
	var once sync.Once

	return registration{
		strategyID: strategyID,
		mode:       mode,
		spec:       spec,
		schedule:   schedule,
		interval:   interval,
		nextRunAt:  nextRunAt,
		stopped:    stopped,
		stop: func() {
			once.Do(func() {
				if stopFn != nil {
					stopFn()
				}
				close(stopped)
			})
		},
	}
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmedValue := strings.TrimSpace(*value)
	if trimmedValue == "" {
		return nil
	}

	return &trimmedValue
}
