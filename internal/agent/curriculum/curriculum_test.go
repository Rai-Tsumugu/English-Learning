package curriculum

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// fakeChat is a minimal llm.Chat implementation used by curriculum tests.
type fakeChat struct {
	fn func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

func (f *fakeChat) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f.fn(ctx, req)
}

// planChat returns a Chat that always replies with the given plan serialized
// as JSON in the response Content.
func planChat(t *testing.T, plan Plan) llm.Chat {
	t.Helper()
	return &fakeChat{fn: func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		content, err := json.Marshal(plan)
		if err != nil {
			t.Fatalf("marshal plan: %v", err)
		}
		return &llm.ChatResponse{Content: string(content)}, nil
	}}
}

func makeTargets(n int, prefix string) []TargetWord {
	out := make([]TargetWord, n)
	for i := 0; i < n; i++ {
		out[i] = TargetWord{
			Lemma:  fmt.Sprintf("%s%d", prefix, i),
			CEFR:   "B1",
			Reason: "test",
		}
	}
	return out
}

func newAgent(t *testing.T, plan Plan) *Agent {
	t.Helper()
	return New(planChat(t, plan), "gpt-test")
}

func TestPlan_Normal12Targets(t *testing.T) {
	want := Plan{Targets: makeTargets(12, "w"), Strategy: StrategyBalanced}
	a := newAgent(t, want)
	got, err := a.Plan(context.Background(), Input{
		UserCEFR:             "B1",
		DueWords:             []DueWord{{Lemma: "a", CEFR: "B1"}},
		DaysSinceLastSession: 1,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(got.Targets) != 12 {
		t.Fatalf("want 12 targets, got %d", len(got.Targets))
	}
	if got.Strategy != StrategyBalanced {
		t.Fatalf("want strategy balanced, got %q", got.Strategy)
	}
}

func TestPlan_CircuitBreakForcesEight(t *testing.T) {
	llmOut := Plan{Targets: makeTargets(12, "w"), Strategy: StrategyBalanced}
	a := newAgent(t, llmOut)
	got, err := a.Plan(context.Background(), Input{
		UserCEFR:             "B1",
		DaysSinceLastSession: 5,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if got.Strategy != StrategyCircuitBreak {
		t.Fatalf("want circuit_break, got %q", got.Strategy)
	}
	if len(got.Targets) != CircuitBreakTargets {
		t.Fatalf("want exactly %d targets, got %d", CircuitBreakTargets, len(got.Targets))
	}
}

func TestPlan_TrimsTo15(t *testing.T) {
	llmOut := Plan{Targets: makeTargets(20, "w"), Strategy: StrategyBalanced}
	a := newAgent(t, llmOut)
	got, err := a.Plan(context.Background(), Input{
		UserCEFR:             "B1",
		DaysSinceLastSession: 1,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(got.Targets) != MaxTargets {
		t.Fatalf("want %d targets, got %d", MaxTargets, len(got.Targets))
	}
}

func TestPlan_ExcludesRecentlyShown(t *testing.T) {
	targets := makeTargets(12, "w")
	targets[3].Lemma = "banned"
	llmOut := Plan{Targets: targets, Strategy: StrategyBalanced}
	a := newAgent(t, llmOut)
	got, err := a.Plan(context.Background(), Input{
		UserCEFR:             "B1",
		RecentlyShownLemmas:  []string{"banned"},
		DaysSinceLastSession: 1,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, tg := range got.Targets {
		if tg.Lemma == "banned" {
			t.Fatalf("banned lemma leaked into plan: %+v", got.Targets)
		}
	}
	if len(got.Targets) != 11 {
		t.Fatalf("want 11 targets after filter, got %d", len(got.Targets))
	}
}
