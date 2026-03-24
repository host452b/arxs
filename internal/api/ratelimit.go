package api

import (
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
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if !rl.lastCall.IsZero() {
		elapsed := time.Since(rl.lastCall)
		if elapsed < rl.interval {
			time.Sleep(rl.interval - elapsed)
		}
	}
	rl.lastCall = time.Now()
}
