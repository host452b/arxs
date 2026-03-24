package api

import (
	"testing"
	"time"
)

func TestRateLimiterEnforcesMinInterval(t *testing.T) {
	rl := NewRateLimiter(100 * time.Millisecond) // Use short interval for testing

	// First call should not block
	start := time.Now()
	rl.Wait()
	firstDuration := time.Since(start)
	if firstDuration > 50*time.Millisecond {
		t.Errorf("first call took %v, expected near-instant", firstDuration)
	}

	// Second call should block ~100ms
	start = time.Now()
	rl.Wait()
	secondDuration := time.Since(start)
	if secondDuration < 80*time.Millisecond {
		t.Errorf("second call took %v, expected >= 80ms", secondDuration)
	}
}

func TestRateLimiterNoBlockAfterInterval(t *testing.T) {
	rl := NewRateLimiter(50 * time.Millisecond)

	rl.Wait()
	time.Sleep(60 * time.Millisecond) // Wait longer than interval

	start := time.Now()
	rl.Wait()
	duration := time.Since(start)
	if duration > 20*time.Millisecond {
		t.Errorf("call after interval took %v, expected near-instant", duration)
	}
}
