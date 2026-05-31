package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

type staticToken struct{ tok string }

func (s staticToken) Token(_ context.Context) (string, error) { return s.tok, nil }

func newTestClient(t *testing.T, srv *httptest.Server, rl *llm.RateLimiter) *Client {
	t.Helper()
	c := New(staticToken{tok: "tok-abc"}, rl)
	WithBaseURL(c, srv.URL)
	WithHTTPClient(c, srv.Client())
	return c
}

func TestClient_Chat_Success(t *testing.T) {
	var gotReq responsesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != responsesPath {
			t.Errorf("bad path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-abc" {
			t.Errorf("auth header=%q", got)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": `{"k":"v"}`},
					},
				},
			},
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv, llm.NewRateLimiter(time.Minute, 3))
	resp, err := c.Chat(context.Background(), llm.ChatRequest{
		Model: "gpt-4o-mini",
		Messages: []llm.Message{
			{Role: "system", Content: "be helpful"},
			{Role: "user", Content: "hi"},
		},
		Schema:     map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}, "required": []any{}},
		SchemaName: "thing",
		MaxTokens:  256,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Content != `{"k":"v"}` {
		t.Fatalf("content=%q", resp.Content)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("usage=%+v", resp.Usage)
	}
	if gotReq.Instructions != "be helpful" {
		t.Fatalf("instructions=%q", gotReq.Instructions)
	}
	if len(gotReq.Input) != 1 || gotReq.Input[0].Role != "user" {
		t.Fatalf("input=%+v", gotReq.Input)
	}
	if gotReq.Input[0].Content[0].Type != "input_text" || gotReq.Input[0].Content[0].Text != "hi" {
		t.Fatalf("content=%+v", gotReq.Input[0].Content)
	}
	if gotReq.Text == nil || gotReq.Text.Format.Type != "json_schema" || !gotReq.Text.Format.Strict || gotReq.Text.Format.Name != "thing" {
		t.Fatalf("text format=%+v", gotReq.Text)
	}
	if gotReq.MaxOutputTokens != 256 {
		t.Fatalf("max_output_tokens=%d", gotReq.MaxOutputTokens)
	}
}

func TestClient_Chat_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if !errors.Is(err, llm.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

func TestClient_Chat_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"slow down"}`))
	}))
	defer srv.Close()
	rl := llm.NewRateLimiter(time.Minute, 2)
	c := newTestClient(t, srv, rl)
	for i := 0; i < 2; i++ {
		_, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
		if !errors.Is(err, llm.ErrRateLimited) {
			t.Fatalf("want ErrRateLimited, got %v", err)
		}
	}
	if !rl.Tripped() {
		t.Fatalf("rate limiter should be tripped after 2x 429")
	}
}

func TestClient_Chat_Refusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output": []map[string]any{
				{
					"type": "message",
					"role": "assistant",
					"content": []map[string]any{
						{"type": "refusal", "text": "cannot help"},
					},
				},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if !errors.Is(err, llm.ErrRefusal) {
		t.Fatalf("want ErrRefusal, got %v", err)
	}
}

func TestClient_Chat_Truncated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "incomplete",
			"output": []map[string]any{
				{
					"type":   "message",
					"status": "incomplete",
					"role":   "assistant",
					"content": []map[string]any{
						{"type": "output_text", "text": "partial"},
					},
				},
			},
		})
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	_, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if !errors.Is(err, llm.ErrTruncated) {
		t.Fatalf("want ErrTruncated, got %v", err)
	}
}

func TestClient_Chat_NoToken(t *testing.T) {
	c := New(staticToken{tok: ""}, nil)
	_, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m"})
	if !errors.Is(err, llm.ErrUnauthenticated) {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

func TestClient_Chat_OutputTextShortcut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"output_text": `{"ok":true}`})
	}))
	defer srv.Close()
	c := newTestClient(t, srv, nil)
	resp, err := c.Chat(context.Background(), llm.ChatRequest{Model: "m", Messages: []llm.Message{{Role: "user", Content: "x"}}})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(resp.Content, "ok") {
		t.Fatalf("content=%q", resp.Content)
	}
}
