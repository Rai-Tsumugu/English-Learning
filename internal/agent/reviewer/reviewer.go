// Package reviewer implements the Reviewer Agent that scores Generator
// outputs on a 4-axis rubric and decides whether each item should pass.
//
// The Reviewer is intended to be sampled (e.g. 10% of regular items, 100%
// of brand-new question formats) so that the cost of running a stronger
// model (gpt-4o by default) stays bounded. A simple in-memory auto-promotion
// mechanism reduces the sampling rate further once recent pass rate exceeds
// a threshold.
package reviewer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/Rai-Tsumugu/English-Learning/internal/agent/generator"
	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// DefaultModel is the model the Reviewer Agent uses by default.
const DefaultModel = "gpt-4o"

// Score is the per-axis 0/1 rubric score returned by the model.
type Score struct {
	Correctness       int    `json:"correctness"`
	CEFRFit           int    `json:"cefr_fit"`
	DistractorQuality int    `json:"distractor_quality"`
	Fluency           int    `json:"fluency"`
	Comment           string `json:"comment"`
}

// Decision is the Reviewer's verdict for a single item.
type Decision struct {
	Pass       bool   `json:"pass"`
	Total      int    `json:"total"`
	Score      Score  `json:"score"`
	Suggestion string `json:"suggestion,omitempty"`
}

// passThreshold is the minimum total score for an item to pass.
const passThreshold = 3

// ReviewOne sends a single Generator item to the Reviewer model and returns
// the resulting Decision. The model is asked for a strict-schema response
// matching Decision (without the computed Pass/Total fields, which the
// helper fills in from Score).
func ReviewOne(ctx context.Context, c llm.Chat, model string, item generator.Item) (Decision, error) {
	if c == nil {
		return Decision{}, errors.New("reviewer: nil chat")
	}
	if model == "" {
		model = DefaultModel
	}
	sys := buildSystemPrompt()
	user := buildUserPrompt(item)
	msgs := []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}
	raw, err := llm.ChatStructured[reviewerResponse](ctx, c, model, msgs, "reviewer_decision")
	if err != nil {
		return Decision{}, fmt.Errorf("reviewer: chat: %w", err)
	}
	total := raw.Score.Correctness + raw.Score.CEFRFit + raw.Score.DistractorQuality + raw.Score.Fluency
	return Decision{
		Pass:       total >= passThreshold,
		Total:      total,
		Score:      raw.Score,
		Suggestion: raw.Suggestion,
	}, nil
}

// reviewerResponse is the raw schema we ask the model to fill in. Pass and
// Total are derived locally from Score so the model only has to commit to
// the four 0/1 axes.
type reviewerResponse struct {
	Score      Score  `json:"score"`
	Suggestion string `json:"suggestion"`
}

func buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are an English-learning item reviewer.\n")
	b.WriteString("Score the given item on four axes, each 0 (fail) or 1 (pass):\n")
	b.WriteString("  correctness        — exactly one answer is correct and unambiguous.\n")
	b.WriteString("  cefr_fit           — the item's difficulty matches its declared CEFR.\n")
	b.WriteString("  distractor_quality — wrong choices are plausible but clearly wrong.\n")
	b.WriteString("  fluency            — the English prompt sounds natural.\n")
	b.WriteString("Also leave a short comment, and if any axis is 0, return a suggestion for fixing the item.\n")
	b.WriteString("Reply strictly in the requested JSON schema.\n")
	return b.String()
}

func buildUserPrompt(item generator.Item) string {
	b, _ := json.MarshalIndent(item, "", "  ")
	return "Item to review:\n" + string(b)
}

// ---------------------------------------------------------------------------
// Sampler
// ---------------------------------------------------------------------------

// Sampler decides which Generator items should be sent to the Reviewer.
//
// The base sampling rate is configurable; certain "new" question formats are
// always reviewed regardless. Recent pass-rate observations may auto-promote
// the sampler into a lower rate once the model proves stable.
type Sampler struct {
	// Rate is the base sampling probability for non-forced items (0..1).
	Rate float64
	// ForceNewFormats is a list of question_type values that bypass random
	// sampling and are always reviewed.
	ForceNewFormats []string

	// Internal state for auto-promotion.
	mu             sync.Mutex
	window         []bool // recent observations (true=pass)
	windowSize     int
	promoted       bool
	promoteAfter   int     // minimum observations before promotion check
	promoteRateLow float64 // rate to drop to once promoted
	passThreshold  float64 // pass-rate threshold for promotion
	rng            *rand.Rand
}

// NewSampler returns a Sampler with sensible defaults: window 100 obs,
// promotion when pass-rate > 0.95, low rate 0.05.
func NewSampler(rate float64, forceNewFormats []string) *Sampler {
	return &Sampler{
		Rate:            rate,
		ForceNewFormats: append([]string(nil), forceNewFormats...),
		windowSize:      100,
		promoteAfter:    100,
		promoteRateLow:  0.05,
		passThreshold:   0.95,
		rng:             rand.New(rand.NewSource(1)),
	}
}

// SetSeed makes the sampler deterministic for tests.
func (s *Sampler) SetSeed(seed int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rng = rand.New(rand.NewSource(seed))
}

// ShouldReview reports whether the given item should be sent to the Reviewer.
func (s *Sampler) ShouldReview(item generator.Item) bool {
	for _, qt := range s.ForceNewFormats {
		if qt == item.QuestionType {
			return true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rng == nil {
		s.rng = rand.New(rand.NewSource(1))
	}
	return s.rng.Float64() < s.Rate
}

// Observe records a Reviewer decision outcome and may auto-promote (lower
// the base Rate) once enough observations show a high pass rate.
func (s *Sampler) Observe(passed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.window = append(s.window, passed)
	if len(s.window) > s.windowSize {
		s.window = s.window[len(s.window)-s.windowSize:]
	}
	if s.promoted || len(s.window) < s.promoteAfter {
		return
	}
	pass := 0
	for _, ok := range s.window {
		if ok {
			pass++
		}
	}
	if float64(pass)/float64(len(s.window)) > s.passThreshold {
		s.Rate = s.promoteRateLow
		s.promoted = true
	}
}
