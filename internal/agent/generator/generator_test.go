package generator

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

type fakeResp struct {
	err     error
	content Output
}

type fakeChat struct {
	responses []fakeResp
	n         int32
	t         *testing.T
}

func (f *fakeChat) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	i := int(atomic.AddInt32(&f.n, 1)) - 1
	if i >= len(f.responses) {
		f.t.Fatalf("unexpected extra call %d", i)
	}
	r := f.responses[i]
	if r.err != nil {
		return nil, r.err
	}
	body, _ := json.Marshal(r.content)
	return &llm.ChatResponse{Content: string(body)}, nil
}

func newChat(t *testing.T, responses []fakeResp) *fakeChat {
	return &fakeChat{responses: responses, t: t}
}

func sampleInput() Input {
	return Input{
		Words: []TargetWord{
			{Lemma: "ubiquitous", CEFR: "C1", GlossJA: "至る所にある"},
		},
		UserCEFR:  "B2",
		PromptVer: "v1",
	}
}

func sampleOutput() Output {
	return Output{Items: []Item{{
		QuestionType: "mcq",
		Prompt:       "Smartphones are ___ in modern life.",
		Choices:      []string{"ubiquitous", "rare", "old", "broken"},
		AnswerIndex:  0,
		AnswerSpan:   "ubiquitous",
		CEFREvidence: "Common C1 vocab",
		TargetLemma:  "ubiquitous",
	}}}
}

func TestGenerate_Success(t *testing.T) {
	c := newChat(t, []fakeResp{{content: sampleOutput()}})
	out, err := Generate(context.Background(), c, "", sampleInput())
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].TargetLemma != "ubiquitous" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if c.n != 1 {
		t.Fatalf("expected 1 call, got %d", c.n)
	}
}

func TestGenerate_TruncatedThenSuccess(t *testing.T) {
	c := newChat(t, []fakeResp{
		{err: llm.ErrTruncated},
		{content: sampleOutput()},
	})
	out, err := Generate(context.Background(), c, "", sampleInput())
	if err != nil {
		t.Fatalf("Generate err: %v", err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(out.Items))
	}
	if c.n != 2 {
		t.Fatalf("expected 2 calls, got %d", c.n)
	}
}

func TestGenerate_RefusalExhausted(t *testing.T) {
	c := newChat(t, []fakeResp{
		{err: llm.ErrRefusal},
		{err: llm.ErrRefusal},
		{err: llm.ErrRefusal},
	})
	_, err := Generate(context.Background(), c, "", sampleInput())
	if err == nil {
		t.Fatal("expected error after exhausting refusals")
	}
	if !errors.Is(err, llm.ErrRefusal) {
		t.Fatalf("expected ErrRefusal, got %v", err)
	}
}

func TestGenerate_NilClient(t *testing.T) {
	if _, err := Generate(context.Background(), nil, "", sampleInput()); err == nil {
		t.Fatal("expected error for nil chat")
	}
}

func TestGenerate_NoWords(t *testing.T) {
	c := newChat(t, nil)
	if _, err := Generate(context.Background(), c, "", Input{}); err == nil {
		t.Fatal("expected error for empty words")
	}
}
