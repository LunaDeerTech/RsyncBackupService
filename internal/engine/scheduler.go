package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

type Scheduler struct {
	db     *store.DB
	queue  *TaskQueue
	mu     sync.Mutex
	timers map[int64]*time.Timer
	stopCh chan struct{}

	generations map[int64]uint64
	stopped     bool
	now         func() time.Time
}

func NewScheduler(db *store.DB, queue *TaskQueue) *Scheduler {
	return &Scheduler{
		db:          db,
		queue:       queue,
		timers:      make(map[int64]*time.Timer),
		stopCh:      make(chan struct{}),
		generations: make(map[int64]uint64),
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("scheduler is nil")
	}
	if s.db == nil {
		return fmt.Errorf("database unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	policies, err := s.db.ListEnabledPolicies()
	if err != nil {
		return err
	}
	for index := range policies {
		if err := s.reloadPolicy(&policies[index], s.now()); err != nil {
			return err
		}
	}

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

	return nil
}

func (s *Scheduler) Stop() {
	if s == nil {
		return
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stopCh)
	for policyID, timer := range s.timers {
		if timer != nil {
			timer.Stop()
		}
		delete(s.timers, policyID)
	}
	s.mu.Unlock()
}

func (s *Scheduler) ReloadPolicy(policyID int64) {
	if s == nil || policyID <= 0 {
		return
	}

	if err := s.reloadPolicyByID(policyID, s.now()); err != nil {
		slog.Error("reload policy schedule failed", "policy_id", policyID, "error", err)
	}
}

func (s *Scheduler) RemovePolicy(policyID int64) {
	if s == nil || policyID <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.removePolicyLocked(policyID)
}

func (s *Scheduler) reloadPolicyByID(policyID int64, now time.Time) error {
	policy, err := s.db.GetPolicyByID(policyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.RemovePolicy(policyID)
			return nil
		}
		return err
	}

	return s.reloadPolicy(policy, now)
}

func (s *Scheduler) reloadPolicy(policy *model.Policy, now time.Time) error {
	if policy == nil {
		return fmt.Errorf("policy is nil")
	}
	if !policy.Enabled {
		s.RemovePolicy(policy.ID)
		return nil
	}

	nextRun, shouldSchedule, err := s.nextTriggerTime(policy, now)
	if err != nil {
		s.RemovePolicy(policy.ID)
		return err
	}
	if !shouldSchedule {
		s.RemovePolicy(policy.ID)
		return nil
	}

	delay := nextRun.Sub(now)
	if delay < 0 {
		delay = 0
	}

	s.scheduleTimer(policy.ID, delay)
	return nil
}

func (s *Scheduler) nextTriggerTime(policy *model.Policy, now time.Time) (time.Time, bool, error) {
	if policy == nil {
		return time.Time{}, false, fmt.Errorf("policy is nil")
	}

	switch strings.ToLower(strings.TrimSpace(policy.ScheduleType)) {
	case "interval":
		return s.nextIntervalTrigger(policy, now)
	case "cron":
		expr, err := ParseCron(policy.ScheduleValue)
		if err != nil {
			return time.Time{}, false, err
		}
		return expr.Next(now), true, nil
	default:
		return time.Time{}, false, fmt.Errorf("unsupported schedule type %q", policy.ScheduleType)
	}
}

func (s *Scheduler) nextIntervalTrigger(policy *model.Policy, now time.Time) (time.Time, bool, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(policy.ScheduleValue))
	if err != nil || seconds <= 0 {
		return time.Time{}, false, fmt.Errorf("invalid interval schedule value %q", policy.ScheduleValue)
	}

	hasActiveRun, err := s.db.HasActivePolicyRunBySource(policy.ID, model.BackupTriggerSourceScheduled)
	if err != nil {
		return time.Time{}, false, err
	}
	if hasActiveRun {
		return time.Time{}, false, nil
	}

	lastBackup, err := s.db.GetLatestCompletedBackupByPolicyAndSource(policy.ID, model.BackupTriggerSourceScheduled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return now.Add(time.Duration(seconds) * time.Second), true, nil
		}
		return time.Time{}, false, err
	}

	completedAt := now
	if lastBackup.CompletedAt != nil {
		completedAt = lastBackup.CompletedAt.UTC()
	}
	nextRun := completedAt.Add(time.Duration(seconds) * time.Second)
	if !nextRun.After(now) {
		return now, true, nil
	}

	return nextRun, true, nil
}

func (s *Scheduler) scheduleTimer(policyID int64, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}

	s.removePolicyTimerLocked(policyID)
	s.generations[policyID]++
	generation := s.generations[policyID]
	timer := time.AfterFunc(delay, func() {
		s.handleTimer(policyID, generation)
	})
	s.timers[policyID] = timer
}

func (s *Scheduler) handleTimer(policyID int64, generation uint64) {
	s.mu.Lock()
	if s.stopped || s.generations[policyID] != generation {
		s.mu.Unlock()
		return
	}
	delete(s.timers, policyID)
	s.mu.Unlock()

	select {
	case <-s.stopCh:
		return
	default:
	}

	policy, err := s.db.GetPolicyByID(policyID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("load scheduled policy failed", "policy_id", policyID, "error", err)
		}
		s.RemovePolicy(policyID)
		return
	}
	if !policy.Enabled {
		s.RemovePolicy(policyID)
		return
	}
	if s.queue == nil {
		slog.Error("skip scheduled policy trigger because task queue is unavailable", "policy_id", policyID)
		s.ReloadPolicy(policyID)
		return
	}

	_, task, err := s.db.CreatePendingPolicyRunWithSource(policy, model.BackupTriggerSourceScheduled)
	if err != nil {
		slog.Error("create scheduled policy run failed", "policy_id", policyID, "error", err)
		s.ReloadPolicy(policyID)
		return
	}
	if err := s.queue.Enqueue(task); err != nil {
		slog.Error("enqueue scheduled task failed", "policy_id", policyID, "task_id", task.ID, "error", err)
		s.ReloadPolicy(policyID)
		return
	}

	if strings.EqualFold(policy.ScheduleType, "cron") {
		if err := s.reloadPolicy(policy, s.now()); err != nil {
			slog.Error("reload cron policy after trigger failed", "policy_id", policyID, "error", err)
		}
	}
}

func (s *Scheduler) removePolicyLocked(policyID int64) {
	s.removePolicyTimerLocked(policyID)
	s.generations[policyID]++
}

func (s *Scheduler) removePolicyTimerLocked(policyID int64) {
	if timer, exists := s.timers[policyID]; exists {
		if timer != nil {
			timer.Stop()
		}
		delete(s.timers, policyID)
	}
}
