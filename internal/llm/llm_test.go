package llm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeChat struct {
	resp *ChatResponse
	err  error
	last ChatRequest
}

func (f *fakeChat) Chat(_ context.Context, req ChatRequest) (*ChatResponse, error) {
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type out struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestChatStructured_Happy(t *testing.T) {
	fc := &fakeChat{resp: &ChatResponse{Content: `{"name":"x","n":3}`}}
	got, err := ChatStructured[out](context.Background(), fc, "gpt-4o-mini",
		[]Message{{Role: "user", Content: "hi"}}, "out")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Name != "x" || got.N != 3 {
		t.Fatalf("got=%+v", got)
	}
	if fc.last.SchemaName != "out" || fc.last.Schema == nil {
		t.Fatalf("schema not propagated: %+v", fc.last)
	}
	// schema sanity: marshalable.
	if _, err := json.Marshal(fc.last.Schema); err != nil {
		t.Fatalf("schema unmarshalable: %v", err)
	}
}

func TestChatStructured_PropagatesError(t *testing.T) {
	fc := &fakeChat{err: ErrRateLimited}
	_, err := ChatStructured[out](context.Background(), fc, "m", nil, "o")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
}
