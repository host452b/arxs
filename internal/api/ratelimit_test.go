package api

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterEnforcesMinInterval(t *testing.T) {
	rl := NewRateLimiter(100 * time.Millisecond) // Use short interval for testing

	// First call should not block
	start := time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	firstDuration := time.Since(start)
	if firstDuration > 50*time.Millisecond {
		t.Errorf("first call took %v, expected near-instant", firstDuration)
	}

	// Second call should block ~100ms
	start = time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	secondDuration := time.Since(start)
	if secondDuration < 80*time.Millisecond {
		t.Errorf("second call took %v, expected >= 80ms", secondDuration)
	}
}

// TestRateLimiterConcurrentCallsEnforceInterval verifies that two concurrent
// goroutines cannot both proceed within the rate window (C5 fix).
// Without the fix both goroutines read the same elapsed, compute the same
// remaining, and wake up at the same time — violating the rate limit.
func TestRateLimiterConcurrentCallsEnforceInterval(t *testing.T) {
	const interval = 150 * time.Millisecond
	rl := NewRateLimiter(interval)

	// Establish a recent lastCall so subsequent calls must wait.
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("first Wait error: %v", err)
	}

	// Launch 2 goroutines that both enter Wait simultaneously.
	completedAt := make([]time.Time, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := rl.Wait(context.Background()); err != nil {
				t.Errorf("goroutine %d Wait error: %v", idx, err)
				return
			}
			completedAt[idx] = time.Now()
		}(i)
	}
	wg.Wait()

	// The two completions must be at least interval apart.
	diff := completedAt[0].Sub(completedAt[1])
	if diff < 0 {
		diff = -diff
	}
	if diff < 100*time.Millisecond {
		t.Errorf("concurrent calls completed only %v apart — rate limit violated (expected >= 100ms gap)", diff)
	}
}

func TestRateLimiterNoBlockAfterInterval(t *testing.T) {
	rl := NewRateLimiter(50 * time.Millisecond)

	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(60 * time.Millisecond) // Wait longer than interval

	start := time.Now()
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	duration := time.Since(start)
	if duration > 20*time.Millisecond {
		t.Errorf("call after interval took %v, expected near-instant", duration)
	}
}
