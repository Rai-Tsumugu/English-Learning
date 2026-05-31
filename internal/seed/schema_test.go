package seed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// repoRoot walks up from this package directory to locate the repo root
// (identified by go.mod). Tests run with cwd set to the package dir.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate repo root")
	return ""
}

var allowedCEFR = map[string]bool{
	"A1": true, "A2": true, "B1": true, "B2": true, "C1": true, "C2": true,
}

// TestPlacementItems_Schema validates the on-disk placement_items.json
// envelope: each item has the required fields, CEFR is one of A1..C2, and
// answer_index is within the choices range. (This overlaps with placement
// loader tests by design — Issue #31 explicitly allows duplication.)
func TestPlacementItems_Schema(t *testing.T) {
	path := filepath.Join(repoRoot(t), "data", "seed", "placement_items.json")
	items, err := Load(path)
	if err != nil {
		t.Fatalf("load placement: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected non-empty placement items")
	}
	for _, it := range items {
		if it.ID == "" || it.Prompt == "" {
			t.Errorf("item %q: missing id/prompt", it.ID)
		}
		if len(it.Choices) < 2 {
			t.Errorf("item %s: need >=2 choices, got %d", it.ID, len(it.Choices))
		}
		if it.AnswerIndex < 0 || it.AnswerIndex >= len(it.Choices) {
			t.Errorf("item %s: answer_index %d out of range", it.ID, it.AnswerIndex)
		}
		if !allowedCEFR[it.CEFR] {
			t.Errorf("item %s: cefr %q not in A1..C2", it.ID, it.CEFR)
		}
	}
}

// wordEntry mirrors one entry in words.sample.json.
type wordEntry struct {
	Lemma    string `json:"lemma"`
	CEFR     string `json:"cefr"`
	FreqRank int    `json:"freq_rank"`
	POS      string `json:"pos"`
	GlossJA  string `json:"gloss_ja"`
}

type wordsFile struct {
	Version string      `json:"version"`
	Items   []wordEntry `json:"items"`
}

// lemmaRe enforces lowercase ASCII letters only (no spaces, no digits,
// no diacritics) so that NGSL本投入時に lemma 正規化処理を入れる前提が崩れない
// ことを保証する。
var lemmaRe = regexp.MustCompile(`^[a-z]+$`)

// TestWordsSample_LemmaShape verifies the sample word list:
//   - lemma is non-empty lowercase ASCII letters only
//   - CEFR is A1..C2
//
// NGSL 上位500語の本格的ゴールデンテストは Phase 後半で
// data/source/NGSL を取り込んでから実装する。本テストはサンプル20語の
// スキーマ整合性で代替する。
func TestWordsSample_LemmaShape(t *testing.T) {
	path := filepath.Join(repoRoot(t), "data", "seed", "words.sample.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read words: %v", err)
	}
	var wf wordsFile
	if err := json.Unmarshal(b, &wf); err != nil {
		t.Fatalf("unmarshal words: %v", err)
	}
	if len(wf.Items) == 0 {
		t.Fatal("expected non-empty words list")
	}
	seen := make(map[string]bool, len(wf.Items))
	for _, w := range wf.Items {
		if w.Lemma == "" {
			t.Error("empty lemma found")
			continue
		}
		if !lemmaRe.MatchString(w.Lemma) {
			t.Errorf("lemma %q: must be lowercase ASCII letters only", w.Lemma)
		}
		if seen[w.Lemma] {
			t.Errorf("duplicate lemma %q", w.Lemma)
		}
		seen[w.Lemma] = true
		if !allowedCEFR[w.CEFR] {
			t.Errorf("lemma %q: cefr %q not in A1..C2", w.Lemma, w.CEFR)
		}
	}
}
