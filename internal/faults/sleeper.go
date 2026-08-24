package faults

import (
	"context"
	"time"
)

type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

type TimerSleeper struct{}

func (TimerSleeper) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
