package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_NoBlockWhenZeroRPS(t *testing.T) {
	limiter := NewLimiter(0, 0.5)

	start := time.Now()
	err := limiter.Wait(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if time.Since(start) > 10*time.Millisecond {
		t.Errorf("limiter with 0 RPS should not block")
	}
}

func TestLimiter_Wait(t *testing.T) {
	rps := 10.0 // 100ms interval
	limiter := NewLimiter(rps, 0)
	defer limiter.Stop()

	ctx := context.Background()

	// Throw away the first tick because time.NewTicker starts immediately counting
	_ = limiter.Wait(ctx)

	start := time.Now()
	err := limiter.Wait(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	duration := time.Since(start)

	// It should take roughly 100ms
	if duration < 50*time.Millisecond || duration > 150*time.Millisecond {
		t.Errorf("expected wait around 100ms, took %v", duration)
	}
}

func TestLimiter_ContextCancellation(t *testing.T) {
	limiter := NewLimiter(1, 0) // 1 second interval
	defer limiter.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := limiter.Wait(ctx)
	if err == nil {
		t.Fatalf("expected context canceled error")
	}
}

func TestLimiter_Jitter(t *testing.T) {
	rps := 10.0                     // 100ms interval
	limiter := NewLimiter(rps, 0.5) // +/- 50ms jitter
	defer limiter.Stop()

	ctx := context.Background()

	_ = limiter.Wait(ctx)

	start := time.Now()
	_ = limiter.Wait(ctx)

	duration := time.Since(start)

	// Interval is 100ms. Jitter is +/- 50ms.
	// Negative jitter just returns immediately, so min wait is the ticker interval (approx 100ms).
	// Positive jitter adds up to 50ms, so max wait is approx 150ms.
	// Allow some slack for goroutine scheduling.
	if duration < 50*time.Millisecond || duration > 300*time.Millisecond {
		t.Errorf("expected jittered wait to be roughly between 100ms and 150ms, took %v", duration)
	}
}
