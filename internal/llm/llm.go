// Package llm defines a provider-agnostic chat LLM interface used by the
// English-Learning agents. The default implementation is provided by the
// internal/chatgpt package which talks to ChatGPT (Codex) via OAuth.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Sentinel errors returned by Chat implementations.
var (
	// ErrRefusal indicates the model refused to produce content (safety,
	// content_filter, etc.).
	ErrRefusal = errors.New("llm: refusal")
	// ErrTruncated indicates the response was cut off (max tokens / length).
	ErrTruncated = errors.New("llm: response truncated")
	// ErrUnauthenticated indicates the user must (re-)run `app login`.
	ErrUnauthenticated = errors.New("llm: not authenticated, run `app login`")
	// ErrRateLimited indicates the provider returned 429.
	ErrRateLimited = errors.New("llm: rate limited")
)

// Message is a single chat message.
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// ChatRequest is a provider-agnostic chat completion request.
type ChatRequest struct {
	Model      string
	Messages   []Message
	Schema     map[string]any // JSON Schema (strict). May be nil for free-form.
	SchemaName string
	MaxTokens  int
}

// Usage records token counters returned by the provider (best-effort).
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// ChatResponse is the parsed result of a chat call. Content is the JSON
// string produced by a strict json_schema response (or free text otherwise).
type ChatResponse struct {
	Content string
	Usage   Usage
}

// Chat is the minimal interface required by agents.
type Chat interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ChatStructured is a generics helper that builds a strict JSON-schema
// request from T, calls c.Chat, and unmarshals the response Content into T.
func ChatStructured[T any](ctx context.Context, c Chat, model string, msgs []Message, schemaName string) (T, error) {
	var zero T
	schema := SchemaFor[T]()
	resp, err := c.Chat(ctx, ChatRequest{
		Model:      model,
		Messages:   msgs,
		Schema:     schema,
		SchemaName: schemaName,
	})
	if err != nil {
		return zero, err
	}
	if resp == nil || resp.Content == "" {
		return zero, fmt.Errorf("llm: empty content")
	}
	var out T
	if err := json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return zero, fmt.Errorf("llm: unmarshal structured content: %w", err)
	}
	return out, nil
}
