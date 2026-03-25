package api

import (
	"context"
	"sync"
	"time"
)

// RateLimiter enforces a minimum interval between calls.
type RateLimiter struct {
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewRateLimiter creates a rate limiter with the given minimum interval.
func NewRateLimiter(interval time.Duration) *RateLimiter {
	return &RateLimiter{interval: interval}
}

// Wait blocks until the minimum interval has elapsed since the last call.
// It returns early with ctx.Err() if the context is cancelled while waiting.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	if rl.interval == 0 || rl.lastCall.IsZero() {
		rl.lastCall = time.Now()
		rl.mu.Unlock()
		return nil
	}
	elapsed := time.Since(rl.lastCall)
	if elapsed >= rl.interval {
		rl.lastCall = time.Now()
		rl.mu.Unlock()
		return nil
	}
	// Reserve this slot before releasing the lock.  If we unlocked first,
	// a concurrent goroutine would read the same lastCall and compute the
	// same remaining — both would then wake at the same time, violating
	// the rate limit.
	rl.lastCall = rl.lastCall.Add(rl.interval)
	remaining := rl.interval - elapsed
	rl.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(remaining):
		return nil
	}
}
