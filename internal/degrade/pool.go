package degrade

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"

	"github.com/Rai-Tsumugu/English-Learning/internal/agent/generator"
)

// PoolItem is a self-contained static learning item that does not depend on
// the generator package. ToGeneratorItem converts to generator.Item.
type PoolItem struct {
	QuestionType string   `json:"question_type"`
	Prompt       string   `json:"prompt"`
	Choices      []string `json:"choices"`
	AnswerIndex  int      `json:"answer_index"`
	AnswerSpan   string   `json:"answer_span"`
	CEFREvidence string   `json:"cefr_evidence"`
	TargetLemma  string   `json:"target_lemma"`
	CEFR         string   `json:"cefr"`
}

// ToGeneratorItem converts the PoolItem to a generator.Item.
func (p PoolItem) ToGeneratorItem() generator.Item {
	return generator.Item{
		QuestionType: p.QuestionType,
		Prompt:       p.Prompt,
		Choices:      append([]string(nil), p.Choices...),
		AnswerIndex:  p.AnswerIndex,
		AnswerSpan:   p.AnswerSpan,
		CEFREvidence: p.CEFREvidence,
		TargetLemma:  p.TargetLemma,
	}
}

// Pool is a static item pool used at L4 degradation.
type Pool struct {
	Items []PoolItem `json:"items"`
	rng   *rand.Rand
}

// LoadPool reads a static pool JSON file from disk.
func LoadPool(path string) (*Pool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("degrade: load pool: %w", err)
	}
	var p Pool
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("degrade: parse pool: %w", err)
	}
	if len(p.Items) == 0 {
		return nil, fmt.Errorf("degrade: empty pool at %s", path)
	}
	return &p, nil
}

// cefrDistance returns a small integer "distance" between two CEFR levels.
// Unknown CEFRs are treated as max-distance.
func cefrDistance(a, b string) int {
	order := map[string]int{"A1": 1, "A2": 2, "B1": 3, "B2": 4, "C1": 5, "C2": 6}
	ai, ok1 := order[a]
	bi, ok2 := order[b]
	if !ok1 || !ok2 {
		return 99
	}
	if ai > bi {
		return ai - bi
	}
	return bi - ai
}

// PickFor returns up to n generator.Items nearest to the requested CEFR.
// If n exceeds the pool size, all items are returned. The pool is sorted
// by CEFR proximity; ties are broken with the pool's RNG.
func (p *Pool) PickFor(cefr string, n int) []generator.Item {
	if p == nil || len(p.Items) == 0 || n <= 0 {
		return nil
	}
	items := make([]PoolItem, len(p.Items))
	copy(items, p.Items)

	// Stable, deterministic-ish shuffle for tie-break.
	r := p.rng
	if r == nil {
		r = rand.New(rand.NewSource(1))
	}
	r.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })

	sort.SliceStable(items, func(i, j int) bool {
		return cefrDistance(items[i].CEFR, cefr) < cefrDistance(items[j].CEFR, cefr)
	})

	if n > len(items) {
		n = len(items)
	}
	out := make([]generator.Item, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, items[i].ToGeneratorItem())
	}
	return out
}

// SetRand allows tests to inject a deterministic RNG.
func (p *Pool) SetRand(r *rand.Rand) { p.rng = r }
