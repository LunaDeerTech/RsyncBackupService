package engine

import (
	"context"
	"time"
)

func managedRetryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	return time.Duration(attempt) * 5 * time.Second
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		if ctx == nil {
			return nil
		}
		return ctx.Err()
	}
	if ctx == nil {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
