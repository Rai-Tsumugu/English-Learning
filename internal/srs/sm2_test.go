package srs

import (
	"math"
	"testing"
	"time"
)

func approxEqual(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func TestNext_FirstSuccess(t *testing.T) {
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	c := Card{Ease: DefaultEase}
	out := Next(c, Review{Quality: 4, ReviewedAt: now})

	if out.Reps != 1 {
		t.Errorf("reps = %d, want 1", out.Reps)
	}
	if out.IntervalDays != 1 {
		t.Errorf("interval = %d, want 1", out.IntervalDays)
	}
	if out.Lapses != 0 {
		t.Errorf("lapses = %d, want 0", out.Lapses)
	}
	// q=4: delta = 0.1 - 1*(0.08 + 1*0.02) = 0.1 - 0.1 = 0 → ease unchanged
	if !approxEqual(out.Ease, 2.5, 1e-9) {
		t.Errorf("ease = %v, want 2.5", out.Ease)
	}
	if !out.LastReview.Equal(now) {
		t.Errorf("LastReview = %v, want %v", out.LastReview, now)
	}
}

func TestNext_SecondSuccess(t *testing.T) {
	c := Card{Ease: DefaultEase, Reps: 1, IntervalDays: 1}
	out := Next(c, Review{Quality: 5, ReviewedAt: time.Now()})

	if out.Reps != 2 {
		t.Errorf("reps = %d, want 2", out.Reps)
	}
	if out.IntervalDays != 6 {
		t.Errorf("interval = %d, want 6", out.IntervalDays)
	}
	// q=5: delta = 0.1
	if !approxEqual(out.Ease, 2.6, 1e-9) {
		t.Errorf("ease = %v, want 2.6", out.Ease)
	}
}

func TestNext_ThirdSuccess(t *testing.T) {
	c := Card{Ease: 2.5, Reps: 2, IntervalDays: 6}
	out := Next(c, Review{Quality: 4, ReviewedAt: time.Now()})

	if out.Reps != 3 {
		t.Errorf("reps = %d, want 3", out.Reps)
	}
	if out.IntervalDays != 15 {
		t.Errorf("interval = %d, want 15 (round(6*2.5))", out.IntervalDays)
	}
	if !approxEqual(out.Ease, 2.5, 1e-9) {
		t.Errorf("ease = %v, want 2.5", out.Ease)
	}
}

func TestNext_Lapse(t *testing.T) {
	c := Card{Ease: 2.5, Reps: 5, IntervalDays: 30, Lapses: 0}
	out := Next(c, Review{Quality: 1, ReviewedAt: time.Now()})

	if out.Reps != 0 {
		t.Errorf("reps = %d, want 0", out.Reps)
	}
	if out.IntervalDays != 1 {
		t.Errorf("interval = %d, want 1", out.IntervalDays)
	}
	if out.Lapses != 1 {
		t.Errorf("lapses = %d, want 1", out.Lapses)
	}
	if !approxEqual(out.Ease, 2.3, 1e-9) {
		t.Errorf("ease = %v, want 2.3", out.Ease)
	}
}

func TestNext_EaseFloor_OnLapse(t *testing.T) {
	c := Card{Ease: EaseFloor, Reps: 3, IntervalDays: 10}
	for i := 0; i < 5; i++ {
		c = Next(c, Review{Quality: 0, ReviewedAt: time.Now()})
		if c.Ease < EaseFloor-1e-9 {
			t.Fatalf("iter %d: ease %v dropped below floor %v", i, c.Ease, EaseFloor)
		}
	}
	if !approxEqual(c.Ease, EaseFloor, 1e-9) {
		t.Errorf("final ease = %v, want %v", c.Ease, EaseFloor)
	}
	if c.Lapses != 5 {
		t.Errorf("lapses = %d, want 5", c.Lapses)
	}
}

func TestNext_EaseFloor_OnLowSuccess(t *testing.T) {
	// Even q=3 reduces ease (delta = -0.14). Confirm floor still holds.
	c := Card{Ease: EaseFloor, Reps: 2, IntervalDays: 6}
	out := Next(c, Review{Quality: 3, ReviewedAt: time.Now()})
	if out.Ease < EaseFloor-1e-9 {
		t.Errorf("ease = %v dropped below floor", out.Ease)
	}
}

func TestNext_DefaultEaseWhenZero(t *testing.T) {
	out := Next(Card{}, Review{Quality: 4, ReviewedAt: time.Now()})
	if !approxEqual(out.Ease, 2.5, 1e-9) {
		t.Errorf("ease for fresh card = %v, want 2.5", out.Ease)
	}
}

func TestNextReviewAt(t *testing.T) {
	base := time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)
	c := Card{LastReview: base, IntervalDays: 6}
	got := NextReviewAt(c)
	want := base.AddDate(0, 0, 6)
	if !got.Equal(want) {
		t.Errorf("NextReviewAt = %v, want %v", got, want)
	}

	if !NextReviewAt(Card{}).IsZero() {
		t.Errorf("NextReviewAt on zero card should be zero time")
	}
}

func TestQualityFromLatency_Correct(t *testing.T) {
	cases := []struct {
		name    string
		latency int
		want    int
	}{
		{"instant", 100, 5},
		{"fast boundary", 2000, 5},
		{"medium", 3500, 4},
		{"slow boundary", 5000, 4},
		{"very slow", 8000, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QualityFromLatency(true, tc.latency); got != tc.want {
				t.Errorf("QualityFromLatency(true, %d) = %d, want %d", tc.latency, got, tc.want)
			}
		})
	}
}

func TestQualityFromLatency_Incorrect(t *testing.T) {
	cases := []struct {
		name    string
		latency int
		want    int
	}{
		{"quick wrong (close miss)", 500, 2},
		{"fast boundary", 2000, 2},
		{"medium wrong", 3500, 1},
		{"slow boundary", 5000, 1},
		{"very slow wrong (blackout)", 9000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := QualityFromLatency(false, tc.latency); got != tc.want {
				t.Errorf("QualityFromLatency(false, %d) = %d, want %d", tc.latency, got, tc.want)
			}
		})
	}
}

func TestQualityFromLatency_NegativeLatency(t *testing.T) {
	if got := QualityFromLatency(true, -100); got != 5 {
		t.Errorf("negative latency correct = %d, want 5", got)
	}
	if got := QualityFromLatency(false, -100); got != 2 {
		t.Errorf("negative latency incorrect = %d, want 2", got)
	}
}

func TestClampQuality(t *testing.T) {
	out := Next(Card{Ease: 2.5, Reps: 1, IntervalDays: 1}, Review{Quality: 99, ReviewedAt: time.Now()})
	// q clamped to 5: success path with reps=1 → interval=6
	if out.IntervalDays != 6 {
		t.Errorf("interval = %d, want 6 (quality clamped to 5)", out.IntervalDays)
	}
	out2 := Next(Card{Ease: 2.5, Reps: 3, IntervalDays: 10}, Review{Quality: -3, ReviewedAt: time.Now()})
	// q clamped to 0: lapse path
	if out2.Reps != 0 || out2.IntervalDays != 1 || out2.Lapses != 1 {
		t.Errorf("negative quality should lapse: %+v", out2)
	}
}
