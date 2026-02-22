package useragent

import (
	"sync"
	"testing"
)

func TestPool_GetSequential(t *testing.T) {
	uas := []string{"A", "B", "C"}
	p := NewPool(uas)

	// Should round robin
	if got := p.GetSequential(); got != "A" {
		t.Errorf("expected A, got %s", got)
	}
	if got := p.GetSequential(); got != "B" {
		t.Errorf("expected B, got %s", got)
	}
	if got := p.GetSequential(); got != "C" {
		t.Errorf("expected C, got %s", got)
	}
	if got := p.GetSequential(); got != "A" {
		t.Errorf("expected A, got %s", got)
	}
}

func TestPool_Default(t *testing.T) {
	// Passing empty slice falls back to default
	p := NewPool(nil)
	if len(p.GetAll()) != len(DefaultPool) {
		t.Errorf("expected pool length %d, got %d", len(DefaultPool), len(p.GetAll()))
	}
	if got := p.GetSequential(); got != DefaultPool[0] {
		t.Errorf("expected %s, got %s", DefaultPool[0], got)
	}
}

func TestPool_GetRandom(t *testing.T) {
	uas := []string{"A", "B"}
	p := NewPool(uas)

	seenA := false
	seenB := false

	// Try 100 times, highly likely we see both A and B
	for i := 0; i < 100; i++ {
		got := p.GetRandom()
		if got == "A" {
			seenA = true
		} else if got == "B" {
			seenB = true
		} else {
			t.Fatalf("unexpected UA: %s", got)
		}
	}

	if !seenA || !seenB {
		t.Errorf("expected to see both A and B randomly, seenA: %v, seenB: %v", seenA, seenB)
	}
}

func TestPool_Concurrent(t *testing.T) {
	uas := []string{"X", "Y", "Z"}
	p := NewPool(uas)

	var wg sync.WaitGroup
	const routines = 100
	const iterations = 1000

	results := make(chan string, routines*iterations)

	for i := 0; i < routines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				results <- p.GetSequential()
			}
		}()
	}

	wg.Wait()
	close(results)

	counts := map[string]int{"X": 0, "Y": 0, "Z": 0}
	for r := range results {
		counts[r]++
	}

	// Total operations is routines * iterations. We expect an even distribution.
	expectedBase := (routines * iterations) / len(uas)
	remainder := (routines * iterations) % len(uas)

	for k, count := range counts {
		if count < expectedBase || count > expectedBase+remainder {
			t.Errorf("expected between %d and %d hits for %s, got %d", expectedBase, expectedBase+remainder, k, count)
		}
	}
}

func TestPool_Empty(t *testing.T) {
	// Internal struct bypass (NewPool handles nil -> DefaultPool)
	p := &Pool{uas: []string{}}

	if got := p.GetSequential(); got != "" {
		t.Errorf("expected empty string on empty sequential, got %s", got)
	}
	if got := p.GetRandom(); got != "" {
		t.Errorf("expected empty string on empty random, got %s", got)
	}
}
