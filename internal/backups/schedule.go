package backups

import (
	"context"
	"time"
)

func RunDaily(ctx context.Context, schedule time.Duration, now Clock, run func(context.Context) error) error {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	for {
		current := now().UTC()
		next := current.Truncate(24 * time.Hour).Add(schedule)
		if !next.After(current) {
			next = next.Add(24 * time.Hour)
		}
		timer := time.NewTimer(next.Sub(current))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
			if err := run(ctx); err != nil {
				return err
			}
		}
	}
}
