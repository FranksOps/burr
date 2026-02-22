package ratelimit

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestLimiter_ConcurrentStress tests that the limiter is safe for concurrent use.
func TestLimiter_ConcurrentStress(t *testing.T) {
	// Create limiter with moderate rate
	limiter := NewLimiter(1000, 0.1) // 1000 rps = 1ms interval
	defer limiter.Stop()

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Spawn many goroutines calling Wait
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if err := limiter.Wait(ctx); err != nil {
						// Expected when context times out
						return
					}
				}
			}
		}()
	}

	wg.Wait()
}
