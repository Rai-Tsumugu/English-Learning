package cache

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
)

// memRepo is an in-memory ContentRepo for tests.
type memRepo struct {
	mu      sync.Mutex
	data    map[string][]byte
	meta    map[string]meta
	hits    map[string]int
	getErr  error
	putErr  error
}

type meta struct{ model, schemaVer, promptVer string }

func newMemRepo() *memRepo {
	return &memRepo{
		data: map[string][]byte{},
		meta: map[string]meta{},
		hits: map[string]int{},
	}
}

func (r *memRepo) Get(_ context.Context, key string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	v, ok := r.data[key]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return v, nil
}

func (r *memRepo) Put(_ context.Context, key string, payload []byte, model, schemaVer, promptVer string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.putErr != nil {
		return r.putErr
	}
	r.data[key] = append([]byte(nil), payload...)
	r.meta[key] = meta{model, schemaVer, promptVer}
	return nil
}

func (r *memRepo) IncHit(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hits[key]++
	return nil
}

type payload struct {
	N int    `json:"n"`
	S string `json:"s"`
}

func TestKey_DeterministicAndCanonical(t *testing.T) {
	a := Key("m", "1", "p", map[string]any{"a": 1, "b": []int{1, 2}})
	b := Key("m", "1", "p", map[string]any{"b": []int{1, 2}, "a": 1})
	if a != b {
		t.Fatalf("expected same key, got %s vs %s", a, b)
	}
	c := Key("m", "1", "p", map[string]any{"a": 2})
	if c == a {
		t.Fatal("expected different key for different inputs")
	}
	d := Key("m2", "1", "p", map[string]any{"a": 1, "b": []int{1, 2}})
	if d == a {
		t.Fatal("expected different key for different model")
	}
}

func TestWithCache_MissThenHit(t *testing.T) {
	repo := newMemRepo()
	ctx := context.Background()
	key := Key("m", "1", "p", "x")

	hitsBefore := GlobalStats().Hits
	missesBefore := GlobalStats().Misses

	calls := 0
	gen := func() (payload, error) {
		calls++
		return payload{N: 42, S: "hello"}, nil
	}

	v, hit, err := WithCache(ctx, repo, key, "m", "1", "p", gen)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit {
		t.Fatal("expected miss")
	}
	if v.N != 42 {
		t.Fatalf("got %+v", v)
	}

	v2, hit2, err := WithCache(ctx, repo, key, "m", "1", "p", gen)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !hit2 {
		t.Fatal("expected hit")
	}
	if v2.N != 42 || v2.S != "hello" {
		t.Fatalf("got %+v", v2)
	}
	if calls != 1 {
		t.Fatalf("gen called %d times, want 1", calls)
	}
	if repo.hits[key] != 1 {
		t.Fatalf("IncHit count: %d", repo.hits[key])
	}
	if GlobalStats().Hits-hitsBefore != 1 {
		t.Fatalf("global hits delta: %d", GlobalStats().Hits-hitsBefore)
	}
	if GlobalStats().Misses-missesBefore != 1 {
		t.Fatalf("global misses delta: %d", GlobalStats().Misses-missesBefore)
	}
}

func TestWithCache_GenError(t *testing.T) {
	repo := newMemRepo()
	want := errors.New("boom")
	_, hit, err := WithCache(context.Background(), repo, "k", "m", "1", "p",
		func() (payload, error) { return payload{}, want })
	if hit {
		t.Fatal("expected miss")
	}
	if !errors.Is(err, want) {
		t.Fatalf("got err %v", err)
	}
}

func TestStats_HitRate(t *testing.T) {
	s := &Stats{Hits: 3, Misses: 1}
	if got := s.HitRate(); got != 0.75 {
		t.Fatalf("HitRate=%v", got)
	}
	empty := &Stats{}
	if got := empty.HitRate(); got != 0 {
		t.Fatalf("empty HitRate=%v", got)
	}
}

func TestWithCache_NilRepo(t *testing.T) {
	v, hit, err := WithCache(context.Background(), nil, "k", "m", "1", "p",
		func() (payload, error) { return payload{N: 1}, nil })
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if hit {
		t.Fatal("expected miss with nil repo")
	}
	if v.N != 1 {
		t.Fatalf("got %+v", v)
	}
}
