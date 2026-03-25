package api

import (
	"context"
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
