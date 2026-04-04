package scheduler

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type scheduleMode string

const (
	scheduleModeCron     scheduleMode = "cron"
	scheduleModeInterval scheduleMode = "interval"
)

type registration struct {
	strategyID uint
	mode       scheduleMode
	spec       string
	schedule   cron.Schedule
	interval   time.Duration
	nextRunAt  time.Time
	stop       func()
	stopped    chan struct{}
}

type registry struct {
	mu            sync.RWMutex
	registrations map[uint]registration
}

func newRegistry() *registry {
	return &registry{registrations: make(map[uint]registration)}
}

func (r *registry) Replace(entry registration) (registration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	previous, replaced := r.registrations[entry.strategyID]
	r.registrations[entry.strategyID] = entry

	return previous, replaced
}

func (r *registry) Remove(strategyID uint) (registration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.registrations[strategyID]
	if !exists {
		return registration{}, false
	}

	delete(r.registrations, strategyID)
	return entry, true
}

func (r *registry) Get(strategyID uint) (registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.registrations[strategyID]
	return entry, exists
}

func (r *registry) UpdateNextRun(strategyID uint, nextRunAt time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.registrations[strategyID]
	if !exists {
		return false
	}

	entry.nextRunAt = nextRunAt
	r.registrations[strategyID] = entry
	return true
}

func (r *registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.registrations)
}

func (r *registry) Drain() []registration {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries := make([]registration, 0, len(r.registrations))
	for _, entry := range r.registrations {
		entries = append(entries, entry)
	}
	r.registrations = make(map[uint]registration)

	return entries
}
