// Package srs implements the SuperMemo-2 (SM-2) spaced repetition algorithm
// used during Phase1 of the English-Learning project.
//
// Reference: https://www.supermemo.com/en/blog/application-of-a-computer-to-improve-the-results-obtained-in-working-with-the-supermemo-method
// Additional project-specific constraints come from docs/design.md §1.4 and §4.4:
//   - ease floor is 1.3
//   - lapse (quality < 3) resets reps to 0 and sets interval to 1 day
//   - latency_ms can be used to derive an SM-2 quality value (0..5)
package srs

import (
	"math"
	"time"
)

// Card represents the persistent SRS state for a single (user, word) pair.
// Field defaults for a brand-new card: Ease=2.5, IntervalDays=0, Reps=0, Lapses=0.
type Card struct {
	Ease         float64   // initial 2.5, floor 1.3
	IntervalDays int       // days until the next review
	Reps         int       // number of consecutive successful reviews
	Lapses       int       // total number of lapses (quality < 3)
	LastReview   time.Time // timestamp of the most recent review
}

// Review represents a single grading event.
type Review struct {
	Quality    int       // SM-2 grade in [0,5]
	ReviewedAt time.Time
}

// EaseFloor is the minimum allowed ease factor (per design.md §4.4).
const EaseFloor = 1.3

// DefaultEase is the initial ease factor for a fresh card.
const DefaultEase = 2.5

// Next applies a single SM-2 update step and returns the next Card state.
//
// Behaviour:
//   - quality < 3 (lapse): reps=0, interval=1, lapses+=1, ease=max(EaseFloor, ease-0.2)
//   - quality >= 3 (success):
//       reps==0 -> interval=1
//       reps==1 -> interval=6
//       reps>=2 -> interval=round(prev_interval * ease)
//     reps += 1, then
//     ease += 0.1 - (5-q)*(0.08 + (5-q)*0.02), floored at EaseFloor.
//
// LastReview is always replaced by r.ReviewedAt.
func Next(c Card, r Review) Card {
	q := clampQuality(r.Quality)

	// Initialise ease for brand-new cards that have not been seeded.
	if c.Ease == 0 {
		c.Ease = DefaultEase
	}

	out := c
	out.LastReview = r.ReviewedAt

	if q < 3 {
		out.Reps = 0
		out.IntervalDays = 1
		out.Lapses = c.Lapses + 1
		newEase := c.Ease - 0.2
		if newEase < EaseFloor {
			newEase = EaseFloor
		}
		out.Ease = newEase
		return out
	}

	// Successful recall.
	switch {
	case c.Reps == 0:
		out.IntervalDays = 1
	case c.Reps == 1:
		out.IntervalDays = 6
	default:
		out.IntervalDays = int(math.Round(float64(c.IntervalDays) * c.Ease))
		if out.IntervalDays < 1 {
			out.IntervalDays = 1
		}
	}
	out.Reps = c.Reps + 1

	// Standard SM-2 ease update.
	delta := 0.1 - float64(5-q)*(0.08+float64(5-q)*0.02)
	newEase := c.Ease + delta
	if newEase < EaseFloor {
		newEase = EaseFloor
	}
	out.Ease = newEase

	return out
}

// NextReviewAt returns the timestamp at which the card is next due.
// Callers may persist this value or recompute on read.
func NextReviewAt(c Card) time.Time {
	if c.LastReview.IsZero() {
		return time.Time{}
	}
	return c.LastReview.AddDate(0, 0, c.IntervalDays)
}

// Latency thresholds for QualityFromLatency. The defaults come from the
// task spec (2000 ms / 5000 ms) and may be tightened once design.md fixes
// concrete numbers.
const (
	fastLatencyMs = 2000
	slowLatencyMs = 5000
)

// QualityFromLatency maps an (correct, latency_ms) pair to an SM-2 quality
// value in [0,5]:
//
//   correct == true:
//     latency <= fastLatencyMs   -> 5 (perfect, immediate)
//     latency <= slowLatencyMs   -> 4 (correct, some hesitation)
//     latency >  slowLatencyMs   -> 3 (correct, but very slow / low confidence)
//
//   correct == false:
//     latency >  slowLatencyMs   -> 0 (no recall at all)
//     latency >  fastLatencyMs   -> 1 (wrong, but a real attempt)
//     latency <= fastLatencyMs   -> 2 (wrong, but quick / close)
//
// Negative latencies are treated as 0.
func QualityFromLatency(correct bool, latencyMs int) int {
	if latencyMs < 0 {
		latencyMs = 0
	}
	if correct {
		switch {
		case latencyMs <= fastLatencyMs:
			return 5
		case latencyMs <= slowLatencyMs:
			return 4
		default:
			return 3
		}
	}
	switch {
	case latencyMs > slowLatencyMs:
		return 0
	case latencyMs > fastLatencyMs:
		return 1
	default:
		return 2
	}
}

func clampQuality(q int) int {
	if q < 0 {
		return 0
	}
	if q > 5 {
		return 5
	}
	return q
}
