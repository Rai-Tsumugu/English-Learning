// Package curriculum builds a daily learning plan (target words) from a
// user's SRS state using an LLM with Structured Outputs.
package curriculum

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// Strategy values returned by the agent.
const (
	StrategyReviewHeavy  = "review_heavy"
	StrategyBalanced     = "balanced"
	StrategyNewHeavy     = "new_heavy"
	StrategyCircuitBreak = "circuit_break"
)

// Plan size limits.
const (
	MinTargets             = 10
	MaxTargets             = 15
	CircuitBreakTargets    = 8
	CircuitBreakDaysCutoff = 3
)

// DueWord describes either an SRS-due word or a new-word candidate.
type DueWord struct {
	Lemma           string `json:"lemma"`
	CEFR            string `json:"cefr"`
	LastQuality     int    `json:"last_quality"`
	DaysSinceReview int    `json:"days_since_review"`
}

// Input is the full state handed to the curriculum agent.
type Input struct {
	UserCEFR             string    `json:"user_cefr"`
	DueWords             []DueWord `json:"due_words"`
	NewWordCandidates    []DueWord `json:"new_word_candidates"`
	RecentlyShownLemmas  []string  `json:"recently_shown_lemmas"`
	DaysSinceLastSession int       `json:"days_since_last_session"`
}

// TargetWord is a single word selected for today's session.
type TargetWord struct {
	Lemma  string `json:"lemma"`
	CEFR   string `json:"cefr"`
	Reason string `json:"reason"`
}

// Plan is the structured output produced by the LLM.
type Plan struct {
	Targets  []TargetWord `json:"targets"`
	Strategy string       `json:"strategy"`
}

const systemPrompt = `You are an English vocabulary learning curriculum planner.
Given a learner's CEFR level, SRS-due words, new word candidates, recently shown
lemmas (last 24h) and days since the last session, build a daily plan.

Rules:
- Produce 10 to 15 target words total.
- Default Review:New ratio is 70:30 (review = words from due_words, new = words from new_word_candidates).
- Never include any lemma that appears in recently_shown_lemmas.
- Avoid putting near-synonyms / semantically very close lemmas together in the same plan.
- Interleave CEFR levels (e.g. alternate A2 and B1) when possible.
- If days_since_last_session >= 3, set strategy to "circuit_break" and return only 8 light targets (mostly easy reviews).
- Strategy must be one of: "review_heavy", "balanced", "new_heavy", "circuit_break".
- Each target must include a short reason in English (why this word today).`

// Agent plans a daily session via an LLM.
type Agent struct {
	chat  llm.Chat
	model string
}

// New constructs an Agent.
func New(chat llm.Chat, model string) *Agent {
	return &Agent{chat: chat, model: model}
}

// Plan calls the LLM and returns a post-processed plan that respects the
// hard constraints (recently-shown exclusion, size limits, circuit-break).
func (a *Agent) Plan(ctx context.Context, in Input) (Plan, error) {
	userJSON, err := marshalUser(in)
	if err != nil {
		return Plan{}, err
	}
	msgs := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userJSON},
	}
	p, err := llm.ChatStructured[Plan](ctx, a.chat, a.model, msgs, "curriculum_plan")
	if err != nil {
		return Plan{}, err
	}
	return postProcess(p, in), nil
}

// postProcess enforces hard constraints regardless of LLM output.
func postProcess(p Plan, in Input) Plan {
	// 1. Exclude recently-shown lemmas.
	if len(in.RecentlyShownLemmas) > 0 {
		recent := make(map[string]struct{}, len(in.RecentlyShownLemmas))
		for _, l := range in.RecentlyShownLemmas {
			recent[l] = struct{}{}
		}
		filtered := p.Targets[:0]
		for _, t := range p.Targets {
			if _, bad := recent[t.Lemma]; bad {
				continue
			}
			filtered = append(filtered, t)
		}
		p.Targets = filtered
	}

	// 2. Deduplicate by lemma (preserve first occurrence).
	seen := make(map[string]struct{}, len(p.Targets))
	dedup := p.Targets[:0]
	for _, t := range p.Targets {
		if _, ok := seen[t.Lemma]; ok {
			continue
		}
		seen[t.Lemma] = struct{}{}
		dedup = append(dedup, t)
	}
	p.Targets = dedup

	// 3. Circuit-break: force strategy and trim to 8 if user has been away.
	if in.DaysSinceLastSession >= CircuitBreakDaysCutoff {
		p.Strategy = StrategyCircuitBreak
		if len(p.Targets) > CircuitBreakTargets {
			p.Targets = p.Targets[:CircuitBreakTargets]
		}
		return p
	}

	// 4. Trim to MaxTargets.
	if len(p.Targets) > MaxTargets {
		p.Targets = p.Targets[:MaxTargets]
	}
	return p
}

func marshalUser(in Input) (string, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return "", fmt.Errorf("curriculum: marshal input: %w", err)
	}
	return string(b), nil
}
