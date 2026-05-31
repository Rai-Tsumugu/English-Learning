package llm

import (
	"sync"
	"time"
)

// RateLimiter tracks 429/5xx events from the provider in a sliding window
// so degrade.Decider can step down quality when the subscription side
// throttles us.
type RateLimiter struct {
	window    time.Duration
	threshold int

	mu     sync.Mutex
	events []time.Time
	now    func() time.Time
}

// RateLimitStats is a snapshot of the current window.
type RateLimitStats struct {
	Window    time.Duration
	Threshold int
	Count     int
	Tripped   bool
}

// NewRateLimiter creates a limiter with the given sliding window and
// trip threshold. Sensible defaults are 5 minutes / 5 events.
func NewRateLimiter(window time.Duration, threshold int) *RateLimiter {
	if window <= 0 {
		window = 5 * time.Minute
	}
	if threshold <= 0 {
		threshold = 5
	}
	return &RateLimiter{window: window, threshold: threshold, now: time.Now}
}

func (r *RateLimiter) clockNow() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Observe records an HTTP status code. 429 and 5xx are treated as rate
// signals; other codes are ignored.
func (r *RateLimiter) Observe(statusCode int) {
	if statusCode != 429 && (statusCode < 500 || statusCode >= 600) {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clockNow()
	r.events = append(r.events, now)
	r.gcLocked(now)
}

func (r *RateLimiter) gcLocked(now time.Time) {
	cutoff := now.Add(-r.window)
	keep := r.events[:0]
	for _, t := range r.events {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	r.events = keep
}

// Tripped returns true if the number of events within the window has
// reached the threshold.
func (r *RateLimiter) Tripped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gcLocked(r.clockNow())
	return len(r.events) >= r.threshold
}

// Reset clears all recorded events.
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

// Snapshot returns a copy of the current stats.
func (r *RateLimiter) Snapshot() RateLimitStats {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gcLocked(r.clockNow())
	return RateLimitStats{
		Window:    r.window,
		Threshold: r.threshold,
		Count:     len(r.events),
		Tripped:   len(r.events) >= r.threshold,
	}
}
