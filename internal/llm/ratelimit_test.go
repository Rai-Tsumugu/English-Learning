package llm

import (
	"testing"
	"time"
)

func TestRateLimiter_TripsOnThreshold(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 3)
	now := time.Now()
	rl.now = func() time.Time { return now }

	rl.Observe(200) // ignored
	rl.Observe(429)
	rl.Observe(503)
	if rl.Tripped() {
		t.Fatalf("should not be tripped at 2 events")
	}
	rl.Observe(429)
	if !rl.Tripped() {
		t.Fatalf("should be tripped at 3 events")
	}
	if got := rl.Snapshot(); got.Count != 3 || !got.Tripped {
		t.Fatalf("snapshot=%+v", got)
	}
}

func TestRateLimiter_WindowEviction(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 2)
	base := time.Now()
	rl.now = func() time.Time { return base }
	rl.Observe(429)
	rl.Observe(429)
	if !rl.Tripped() {
		t.Fatalf("expected tripped")
	}
	// Advance past the window.
	rl.now = func() time.Time { return base.Add(2 * time.Minute) }
	if rl.Tripped() {
		t.Fatalf("events should have aged out")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 1)
	rl.Observe(429)
	if !rl.Tripped() {
		t.Fatal("expected tripped")
	}
	rl.Reset()
	if rl.Tripped() {
		t.Fatal("expected reset")
	}
}

func TestRateLimiter_IgnoresSuccess(t *testing.T) {
	rl := NewRateLimiter(time.Minute, 1)
	rl.Observe(200)
	rl.Observe(404)
	if rl.Tripped() {
		t.Fatal("non-rate codes must not trip")
	}
}
