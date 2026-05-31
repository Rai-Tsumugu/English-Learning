// Package degrade implements the rate-driven degradation table (L1-L4)
// used to gracefully reduce the work done by the agent pipeline when the
// ChatGPT subscription side starts throttling us (429s).
//
//	L1 (normal):       Curriculum -> Generator -> Reviewer -> cache
//	L2 (no reviewer):  >= 50% threshold 429s, skip Reviewer
//	L3 (cache only):   >= 80% threshold 429s, only serve cache hits, no new generation
//	L4 (static pool):  >= 100% threshold 429s, fall back to a static item pool
package degrade

import "github.com/Rai-Tsumugu/English-Learning/internal/llm"

// Level is the current degradation level. Values are 1..4 (L1..L4).
type Level int

const (
	L1 Level = 1
	L2 Level = 2
	L3 Level = 3
	L4 Level = 4
)

// DefaultThresholds are the default 429-count ratios (count / threshold)
// that trigger each degradation level. Index i corresponds to Level i+1.
// L1 is always allowed (threshold 0.0).
var DefaultThresholds = [4]float64{0.0, 0.50, 0.80, 1.0}

// Decider computes the current degradation level from a RateLimiter.
type Decider struct {
	rl         *llm.RateLimiter
	thresholds [4]float64
}

// NewDecider builds a Decider using DefaultThresholds.
func NewDecider(rl *llm.RateLimiter) *Decider {
	return &Decider{rl: rl, thresholds: DefaultThresholds}
}

// NewDeciderWithThresholds builds a Decider with custom thresholds.
// Thresholds must be monotonically non-decreasing.
func NewDeciderWithThresholds(rl *llm.RateLimiter, thresholds [4]float64) *Decider {
	return &Decider{rl: rl, thresholds: thresholds}
}

// usage returns count / threshold from the RateLimiter snapshot. When rl is
// nil, 0 is returned (i.e. always L1).
func (d *Decider) usage() float64 {
	if d == nil || d.rl == nil {
		return 0
	}
	s := d.rl.Snapshot()
	if s.Threshold <= 0 {
		return 0
	}
	return float64(s.Count) / float64(s.Threshold)
}

// Current returns the current degradation level.
func (d *Decider) Current() Level {
	r := d.usage()
	lvl := L1
	for i, th := range d.thresholds {
		if r >= th {
			lvl = Level(i + 1)
		}
	}
	return lvl
}

// AllowGenerator reports whether new items may be generated. Disallowed at
// L3 and L4 (cache-only / static pool).
func (d *Decider) AllowGenerator() bool {
	return d.Current() <= L2
}

// AllowReviewer reports whether the Reviewer agent should run. Disallowed
// from L2 onwards.
func (d *Decider) AllowReviewer() bool {
	return d.Current() <= L1
}

// AllowOnlyCache reports whether only cache hits may be served (no new
// generation, but the static pool is not yet engaged).
func (d *Decider) AllowOnlyCache() bool {
	return d.Current() == L3
}

// UseStaticPool reports whether the static item pool must be used.
func (d *Decider) UseStaticPool() bool {
	return d.Current() >= L4
}
