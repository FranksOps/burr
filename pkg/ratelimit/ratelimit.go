package ratelimit

import (
	"context"
	"math/rand"
	"time"
)

// Limiter controls the rate and timing of operations, incorporating optional jitter.
// It is safe for concurrent use by multiple goroutines.
type Limiter struct {
	ticker   *time.Ticker
	jitter   float64 // 0.0 to 1.0
	interval time.Duration
	ch       <-chan time.Time
}

// NewLimiter creates a new limiter with the given requests per second (rps)
// and jitter factor. Jitter must be between 0.0 and 1.0.
// If rps is <= 0, the limiter does not block.
func NewLimiter(rps float64, jitter float64) *Limiter {
	if rps <= 0 {
		return &Limiter{
			jitter: jitter,
		}
	}

	if jitter < 0 {
		jitter = 0
	} else if jitter > 1 {
		jitter = 1
	}

	interval := time.Duration(float64(time.Second) / rps)
	ticker := time.NewTicker(interval)

	return &Limiter{
		ticker:   ticker,
		jitter:   jitter,
		interval: interval,
		ch:       ticker.C,
	}
}

// Wait blocks until it is time to perform the next operation, or until the
// context is canceled. It applies jitter to the sleep time if configured.
func (l *Limiter) Wait(ctx context.Context) error {
	if l.ch == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.ch:
		if l.jitter > 0 {
			// Calculate random jitter between +/- (jitter * interval)
			jitterFactor := (rand.Float64() * 2) - 1.0 // -1.0 to 1.0
			jitterDuration := time.Duration(float64(l.interval) * l.jitter * jitterFactor)

			// If jitter duration is positive, sleep for the extra time
			if jitterDuration > 0 {
				select {
				case <-time.After(jitterDuration):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			// If jitter duration is negative, we could ideally run earlier,
			// but a Ticker enforces minimum wait time natively. So negative
			// jitter just means "run immediately when ticker ticks". This gives
			// a slight bias toward exactly interval or later, but achieves randomization.
		}
	}
	return nil
}

// Stop releases any resources associated with the limiter.
func (l *Limiter) Stop() {
	if l.ticker != nil {
		l.ticker.Stop()
	}
}
