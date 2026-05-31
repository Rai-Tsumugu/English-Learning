// Package chatgpt implements an llm.Chat backed by ChatGPT's Codex backend
// Responses API, authenticated with an OAuth access token (no API key).
package chatgpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// DefaultBaseURL is the ChatGPT backend host hosting the Codex Responses API.
const DefaultBaseURL = "https://chatgpt.com"

// responsesPath is the Codex Responses endpoint path under the backend host.
const responsesPath = "/backend-api/codex/responses"

// TokenProvider returns a fresh OAuth access token. Callers usually wrap
// the oauth.Flow / Store from internal/oauth.
type TokenProvider interface {
	Token(ctx context.Context) (accessToken string, err error)
}

// Client is a thin HTTP client around the Codex Responses endpoint.
type Client struct {
	http    *http.Client
	auth    TokenProvider
	baseURL string
	rl      *llm.RateLimiter
}

// New constructs a Client. If rl is nil a no-trip default is created.
func New(auth TokenProvider, rl *llm.RateLimiter) *Client {
	if rl == nil {
		rl = llm.NewRateLimiter(0, 0)
	}
	return &Client{
		http:    &http.Client{Timeout: 90 * time.Second},
		auth:    auth,
		baseURL: DefaultBaseURL,
		rl:      rl,
	}
}

// WithBaseURL overrides the base URL (intended for tests).
func WithBaseURL(c *Client, url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

// WithHTTPClient overrides the inner *http.Client (intended for tests).
func WithHTTPClient(c *Client, hc *http.Client) {
	if hc != nil {
		c.http = hc
	}
}

// --- Wire types --------------------------------------------------------------

type respContentPart struct {
	Type string `json:"type"` // "input_text"
	Text string `json:"text"`
}

type respInputMessage struct {
	Role    string            `json:"role"`
	Content []respContentPart `json:"content"`
}

type respTextFormat struct {
	Type   string         `json:"type"`             // "json_schema"
	Name   string         `json:"name,omitempty"`
	Strict bool           `json:"strict,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
}

type respTextField struct {
	Format respTextFormat `json:"format"`
}

type responsesRequest struct {
	Model           string             `json:"model"`
	Instructions    string             `json:"instructions,omitempty"`
	Input           []respInputMessage `json:"input"`
	Text            *respTextField     `json:"text,omitempty"`
	MaxOutputTokens int                `json:"max_output_tokens,omitempty"`
}

// responsesResponse models the subset of the Codex Responses API output we
// rely on. The actual wire format is not finalised and may contain extra
// fields; we therefore keep this lenient (unknown fields are ignored by
// encoding/json) and tolerate either the top-level convenience field
// `output_text` or the structured `output[].content[].text` shape.
//
// TODO(phase-B): confirm exact JSON shape against a live Codex backend
// response and adjust if the structure differs. Keep tolerant for now.
type responsesResponse struct {
	OutputText string                 `json:"output_text,omitempty"`
	Output     []responsesOutputItem  `json:"output,omitempty"`
	Status     string                 `json:"status,omitempty"`
	Refusal    string                 `json:"refusal,omitempty"`
	Usage      responsesUsage         `json:"usage,omitempty"`
	Error      *responsesErrorPayload `json:"error,omitempty"`
}

type responsesOutputItem struct {
	Type    string                   `json:"type,omitempty"`
	Status  string                   `json:"status,omitempty"`
	Role    string                   `json:"role,omitempty"`
	Content []responsesOutputContent `json:"content,omitempty"`
}

type responsesOutputContent struct {
	Type    string `json:"type,omitempty"` // "output_text" | "refusal"
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

type responsesErrorPayload struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
}

// --- Chat --------------------------------------------------------------------

// Chat implements llm.Chat.
func (c *Client) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	token := ""
	if c.auth != nil {
		t, err := c.auth.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", llm.ErrUnauthenticated, err)
		}
		token = t
	}
	if token == "" {
		return nil, llm.ErrUnauthenticated
	}

	body := buildRequest(req)
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("chatgpt: marshal request: %w", err)
	}

	url := c.baseURL + responsesPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	// TODO(phase-B): confirm the required beta / version header(s) for the
	// Codex Responses API. The value below mirrors the public Responses API
	// header convention and may need adjustment after live verification.
	httpReq.Header.Set("OpenAI-Beta", "responses-2024-12-17")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		c.rl.Observe(resp.StatusCode)
		return nil, llm.ErrUnauthenticated
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		c.rl.Observe(resp.StatusCode)
		return nil, llm.ErrRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.rl.Observe(resp.StatusCode)
		return nil, fmt.Errorf("chatgpt: http %d: %s", resp.StatusCode, llm.Mask(string(data)))
	}

	var parsed responsesResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("chatgpt: decode: %w", err)
	}

	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("chatgpt: api error: %s", parsed.Error.Message)
	}
	if parsed.Refusal != "" {
		return nil, fmt.Errorf("%w: %s", llm.ErrRefusal, parsed.Refusal)
	}
	if parsed.Status == "incomplete" {
		return nil, llm.ErrTruncated
	}

	content, refusal, truncated := extractText(parsed)
	if refusal != "" {
		return nil, fmt.Errorf("%w: %s", llm.ErrRefusal, refusal)
	}
	if truncated {
		return nil, llm.ErrTruncated
	}
	if content == "" {
		return nil, fmt.Errorf("chatgpt: empty response content")
	}

	return &llm.ChatResponse{
		Content: content,
		Usage: llm.Usage{
			InputTokens:  parsed.Usage.InputTokens,
			OutputTokens: parsed.Usage.OutputTokens,
		},
	}, nil
}

func buildRequest(req llm.ChatRequest) responsesRequest {
	out := responsesRequest{
		Model:           req.Model,
		MaxOutputTokens: req.MaxTokens,
	}

	// Collect system messages into the instructions field; the rest become
	// input messages (user/assistant).
	var instructions []string
	for _, m := range req.Messages {
		if m.Role == "system" {
			instructions = append(instructions, m.Content)
			continue
		}
		role := m.Role
		if role == "" {
			role = "user"
		}
		out.Input = append(out.Input, respInputMessage{
			Role: role,
			Content: []respContentPart{{
				Type: "input_text",
				Text: m.Content,
			}},
		})
	}
	out.Instructions = strings.Join(instructions, "\n\n")

	if req.Schema != nil {
		name := req.SchemaName
		if name == "" {
			name = "response"
		}
		out.Text = &respTextField{Format: respTextFormat{
			Type:   "json_schema",
			Name:   name,
			Strict: true,
			Schema: req.Schema,
		}}
	}
	return out
}

func extractText(r responsesResponse) (text string, refusal string, truncated bool) {
	if r.OutputText != "" {
		return r.OutputText, "", false
	}
	for _, item := range r.Output {
		if item.Status == "incomplete" {
			truncated = true
		}
		for _, p := range item.Content {
			if p.Refusal != "" {
				refusal = p.Refusal
			}
			if p.Type == "refusal" && p.Text != "" {
				refusal = p.Text
			}
			if p.Text != "" && p.Type != "refusal" {
				text = p.Text
			}
		}
	}
	return text, refusal, truncated
}
