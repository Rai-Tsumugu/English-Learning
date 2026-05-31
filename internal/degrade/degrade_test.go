package degrade

import (
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

func observe429(rl *llm.RateLimiter, n int) {
	for i := 0; i < n; i++ {
		rl.Observe(429)
	}
}

func TestDecider_LevelTransitions(t *testing.T) {
	// threshold=10 → 50% = 5 events, 80% = 8 events, 100% = 10 events
	rl := llm.NewRateLimiter(5*time.Minute, 10)
	d := NewDecider(rl)

	// L1: no events.
	if got := d.Current(); got != L1 {
		t.Fatalf("L1: want %d, got %d", L1, got)
	}
	if !d.AllowGenerator() || !d.AllowReviewer() || d.AllowOnlyCache() || d.UseStaticPool() {
		t.Fatalf("L1 flags wrong: gen=%v rev=%v cacheOnly=%v static=%v",
			d.AllowGenerator(), d.AllowReviewer(), d.AllowOnlyCache(), d.UseStaticPool())
	}

	// L2: 50% (5/10).
	observe429(rl, 5)
	if got := d.Current(); got != L2 {
		t.Fatalf("L2: want %d, got %d", L2, got)
	}
	if !d.AllowGenerator() || d.AllowReviewer() {
		t.Fatalf("L2 flags wrong: gen=%v rev=%v", d.AllowGenerator(), d.AllowReviewer())
	}

	// L3: 80% (8/10).
	observe429(rl, 3) // total 8
	if got := d.Current(); got != L3 {
		t.Fatalf("L3: want %d, got %d", L3, got)
	}
	if d.AllowGenerator() || !d.AllowOnlyCache() || d.UseStaticPool() {
		t.Fatalf("L3 flags wrong: gen=%v cacheOnly=%v static=%v",
			d.AllowGenerator(), d.AllowOnlyCache(), d.UseStaticPool())
	}

	// L4: 100% (10/10).
	observe429(rl, 2) // total 10
	if got := d.Current(); got != L4 {
		t.Fatalf("L4: want %d, got %d", L4, got)
	}
	if !d.UseStaticPool() || d.AllowGenerator() || d.AllowReviewer() {
		t.Fatalf("L4 flags wrong: static=%v gen=%v rev=%v",
			d.UseStaticPool(), d.AllowGenerator(), d.AllowReviewer())
	}
}

func TestDecider_NilRateLimiter(t *testing.T) {
	d := NewDecider(nil)
	if d.Current() != L1 {
		t.Fatalf("nil rl should yield L1, got %d", d.Current())
	}
}

func TestPool_LoadAndPick(t *testing.T) {
	path := filepath.Join("..", "..", "data", "seed", "static_pool.json")
	p, err := LoadPool(path)
	if err != nil {
		t.Fatalf("LoadPool: %v", err)
	}
	if len(p.Items) == 0 {
		t.Fatal("pool is empty")
	}
	p.SetRand(rand.New(rand.NewSource(42)))

	got := p.PickFor("B1", 3)
	if len(got) != 3 {
		t.Fatalf("PickFor B1 3: want 3, got %d", len(got))
	}
	for _, it := range got {
		if len(it.Choices) == 0 || it.Prompt == "" {
			t.Fatalf("malformed item: %+v", it)
		}
		if it.AnswerIndex < 0 || it.AnswerIndex >= len(it.Choices) {
			t.Fatalf("answer index out of range: %+v", it)
		}
	}

	all := p.PickFor("A1", len(p.Items)+5)
	if len(all) != len(p.Items) {
		t.Fatalf("PickFor over-pool: want %d, got %d", len(p.Items), len(all))
	}

	if out := p.PickFor("A1", 0); out != nil {
		t.Fatalf("PickFor 0: want nil, got %v", out)
	}
}

func TestPool_LoadMissing(t *testing.T) {
	if _, err := LoadPool("nonexistent.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
