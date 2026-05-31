package seed

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/db"
)

func seedPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	// internal/seed/placement_test.go -> repo root
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "data", "seed", "placement_items.json")
}

func TestLoad_ReturnsThirtyItems(t *testing.T) {
	items, err := Load(seedPath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(items), 30; got != want {
		t.Fatalf("item count = %d, want %d", got, want)
	}
}

func TestLoad_UniqueIDsAndValidAnswerIndex(t *testing.T) {
	items, err := Load(seedPath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	seen := make(map[string]struct{})
	for _, it := range items {
		if _, dup := seen[it.ID]; dup {
			t.Errorf("duplicate id %s", it.ID)
		}
		seen[it.ID] = struct{}{}
		if it.AnswerIndex < 0 || it.AnswerIndex >= len(it.Choices) {
			t.Errorf("item %s: answer_index %d out of range (len choices=%d)", it.ID, it.AnswerIndex, len(it.Choices))
		}
		if it.CEFR == "" {
			t.Errorf("item %s: missing cefr", it.ID)
		}
	}
}

func TestLoad_CEFRDistribution(t *testing.T) {
	items, err := Load(seedPath(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := map[string]int{"A1": 5, "A2": 6, "B1": 7, "B2": 6, "C1": 4, "C2": 2}
	got := make(map[string]int)
	for _, it := range items {
		got[it.CEFR]++
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("CEFR %s: got %d, want %d", k, got[k], v)
		}
	}
}

func TestLoadInto_BulkInsert(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer sqlDB.Close()

	if err := LoadInto(ctx, sqlDB, seedPath(t)); err != nil {
		t.Fatalf("LoadInto: %v", err)
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_items`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 30 {
		t.Fatalf("placement_items rows = %d, want 30", n)
	}
}
