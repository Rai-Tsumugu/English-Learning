package reviewer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/agent/generator"
	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

type fakeChat struct {
	fn func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

func (f *fakeChat) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f.fn(ctx, req)
}

func reviewerChat(t *testing.T, score Score, suggestion string) llm.Chat {
	t.Helper()
	return &fakeChat{fn: func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		body, err := json.Marshal(reviewerResponse{Score: score, Suggestion: suggestion})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return &llm.ChatResponse{Content: string(body)}, nil
	}}
}

func sampleItem() generator.Item {
	return generator.Item{
		QuestionType: "mcq",
		Prompt:       "Smartphones are ___ in modern life.",
		Choices:      []string{"ubiquitous", "rare", "old", "broken"},
		AnswerIndex:  0,
		AnswerSpan:   "ubiquitous",
		CEFREvidence: "C1",
		TargetLemma:  "ubiquitous",
	}
}

func TestReviewOne_Pass(t *testing.T) {
	c := reviewerChat(t, Score{Correctness: 1, CEFRFit: 1, DistractorQuality: 1, Fluency: 1, Comment: "ok"}, "")
	d, err := ReviewOne(context.Background(), c, "", sampleItem())
	if err != nil {
		t.Fatalf("ReviewOne err: %v", err)
	}
	if !d.Pass || d.Total != 4 {
		t.Fatalf("expected pass=true total=4, got %+v", d)
	}
}

func TestReviewOne_Fail(t *testing.T) {
	c := reviewerChat(t, Score{Correctness: 0, CEFRFit: 1, DistractorQuality: 0, Fluency: 1, Comment: "ambiguous"}, "rewrite distractors")
	d, err := ReviewOne(context.Background(), c, "", sampleItem())
	if err != nil {
		t.Fatalf("ReviewOne err: %v", err)
	}
	if d.Pass {
		t.Fatalf("expected pass=false, got %+v", d)
	}
	if d.Total != 2 {
		t.Fatalf("expected total=2, got %d", d.Total)
	}
	if d.Suggestion == "" {
		t.Fatal("expected non-empty suggestion on fail")
	}
}

func TestReviewOne_NilClient(t *testing.T) {
	if _, err := ReviewOne(context.Background(), nil, "", sampleItem()); err == nil {
		t.Fatal("expected error for nil chat")
	}
}

func TestSampler_ApproximateRate(t *testing.T) {
	s := NewSampler(0.1, nil)
	s.SetSeed(42)
	it := sampleItem()
	hits := 0
	const N = 1000
	for i := 0; i < N; i++ {
		if s.ShouldReview(it) {
			hits++
		}
	}
	if hits < 60 || hits > 140 {
		t.Fatalf("rate 0.1 over %d trials: got %d hits (want ~100)", N, hits)
	}
}

func TestSampler_ForceNewFormats(t *testing.T) {
	s := NewSampler(0.0, []string{"matching", "ordering"})
	s.SetSeed(1)
	it := sampleItem()
	it.QuestionType = "matching"
	for i := 0; i < 100; i++ {
		if !s.ShouldReview(it) {
			t.Fatalf("forced format must always sample (i=%d)", i)
		}
	}
	it.QuestionType = "mcq"
	for i := 0; i < 100; i++ {
		if s.ShouldReview(it) {
			t.Fatalf("rate=0 non-forced sampled at i=%d", i)
		}
	}
}

func TestSampler_AutoPromote(t *testing.T) {
	s := NewSampler(0.1, nil)
	for i := 0; i < 100; i++ {
		s.Observe(true)
	}
	if s.Rate != 0.05 {
		t.Fatalf("expected Rate to drop to 0.05 after 100 passes, got %v", s.Rate)
	}
	for i := 0; i < 50; i++ {
		s.Observe(false)
	}
	if s.Rate != 0.05 {
		t.Fatalf("Rate must stay promoted, got %v", s.Rate)
	}
}

func TestSampler_NoPromoteBelowThreshold(t *testing.T) {
	s := NewSampler(0.1, nil)
	for i := 0; i < 100; i++ {
		s.Observe(i%10 != 0)
	}
	if s.Rate != 0.1 {
		t.Fatalf("Rate should not promote below threshold; got %v", s.Rate)
	}
}
