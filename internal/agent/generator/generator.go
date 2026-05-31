// Package generator implements the Generator Agent that produces English
// learning items (MCQ / cloze) for a set of target words via an LLM with
// strict JSON-schema Structured Outputs.
package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// TargetWord is a single word the agent should generate an item for.
type TargetWord struct {
	Lemma   string `json:"lemma"`
	CEFR    string `json:"cefr"`
	GlossJA string `json:"gloss_ja"`
}

// Input is the request payload for Generate.
type Input struct {
	Words     []TargetWord `json:"words"`
	UserCEFR  string       `json:"user_cefr"`
	PromptVer string       `json:"prompt_ver"`
}

// Item is a single generated learning item.
type Item struct {
	QuestionType string   `json:"question_type"`
	Prompt       string   `json:"prompt"`
	Choices      []string `json:"choices"`
	AnswerIndex  int      `json:"answer_index"`
	AnswerSpan   string   `json:"answer_span"`
	CEFREvidence string   `json:"cefr_evidence"`
	TargetLemma  string   `json:"target_lemma"`
}

// Output is the strict schema response wrapper.
type Output struct {
	Items []Item `json:"items"`
}

// DefaultModel is the model used when none is provided in Input.
const DefaultModel = "gpt-4o-mini"

// maxRetries is the number of attempts (1 initial + N retries).
const maxRetries = 2

// Generate calls the LLM and returns a structured Output.
//
// Retry policy: on llm.ErrTruncated the request is retried with max_tokens
// expanded by 1.5x (up to maxRetries times). On llm.ErrRefusal the request
// is retried once; subsequent refusals are surfaced to the caller.
func Generate(ctx context.Context, c llm.Chat, model string, in Input) (Output, error) {
	if c == nil {
		return Output{}, errors.New("generator: nil chat")
	}
	if len(in.Words) == 0 {
		return Output{}, errors.New("generator: no target words")
	}
	if model == "" {
		model = DefaultModel
	}
	sys := buildSystemPrompt(in)
	user := buildUserPrompt(in)
	msgs := []llm.Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: user},
	}

	maxTokens := 1024
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		out, err := chatStructuredWithTokens(ctx, c, model, msgs, "generator_output", maxTokens)
		if err == nil {
			return out, nil
		}
		lastErr = err
		switch {
		case errors.Is(err, llm.ErrTruncated):
			maxTokens = int(float64(maxTokens) * 1.5)
			continue
		case errors.Is(err, llm.ErrRefusal):
			// retry once; if it fails again loop will exit at maxRetries.
			continue
		default:
			return Output{}, err
		}
	}
	return Output{}, fmt.Errorf("generator: exhausted retries: %w", lastErr)
}

// chatStructuredWithTokens builds a strict-schema chat request with an
// explicit MaxTokens and decodes the response into Output.
func chatStructuredWithTokens(ctx context.Context, c llm.Chat, model string, msgs []llm.Message, schemaName string, maxTokens int) (Output, error) {
	schema := llm.SchemaFor[Output]()
	resp, err := c.Chat(ctx, llm.ChatRequest{
		Model:      model,
		Messages:   msgs,
		Schema:     schema,
		SchemaName: schemaName,
		MaxTokens:  maxTokens,
	})
	if err != nil {
		return Output{}, err
	}
	if resp == nil || resp.Content == "" {
		return Output{}, fmt.Errorf("generator: empty content")
	}
	var out Output
	if err := json.Unmarshal([]byte(resp.Content), &out); err != nil {
		return Output{}, fmt.Errorf("generator: unmarshal: %w", err)
	}
	return out, nil
}

func buildSystemPrompt(in Input) string {
	var b strings.Builder
	b.WriteString("You are an English language item generator.\n")
	b.WriteString("Produce one item per target word.\n")
	b.WriteString("Each item is either question_type=\"mcq\" (multiple choice) or \"cloze\" (fill-in-the-blank).\n")
	b.WriteString("MCQ items must have 4 choices with exactly one correct answer at answer_index.\n")
	b.WriteString("Cloze items: blank the target word in prompt; choices may be empty; answer_span is the target word.\n")
	b.WriteString("answer_span must be the lemma surface form. cefr_evidence must justify the chosen CEFR level briefly.\n")
	b.WriteString("Difficulty MUST be appropriate for the user's CEFR.\n")
	b.WriteString("Prompt version: ")
	b.WriteString(in.PromptVer)
	b.WriteString("\n")
	return b.String()
}

func buildUserPrompt(in Input) string {
	var b strings.Builder
	fmt.Fprintf(&b, "User CEFR: %s\nTarget words:\n", in.UserCEFR)
	for _, w := range in.Words {
		fmt.Fprintf(&b, "- lemma=%q cefr=%s gloss_ja=%q\n", w.Lemma, w.CEFR, w.GlossJA)
	}
	b.WriteString("\nReturn JSON matching the schema.")
	return b.String()
}
