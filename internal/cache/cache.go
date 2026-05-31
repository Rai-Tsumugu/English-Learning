// Package cache implements a content cache wrapper used by generation
// agents. Cache keys are derived from (model, schema_ver, prompt_ver,
// canonical-JSON-of-inputs) and hashed with sha256.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync/atomic"
)

// ContentRepo is the storage interface used by the cache wrapper.
// It is a subset of internal/repo.Content's surface, exposed as an
// interface so callers can inject a concrete repo or an adapter.
type ContentRepo interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, payload []byte, model, schemaVer, promptVer string) error
	IncHit(ctx context.Context, key string) error
}

// Stats holds cache hit/miss counters.
type Stats struct {
	Hits   int64
	Misses int64
}

// HitRate returns hits / (hits + misses); returns 0 when there is no data.
func (s *Stats) HitRate() float64 {
	h := atomic.LoadInt64(&s.Hits)
	m := atomic.LoadInt64(&s.Misses)
	total := h + m
	if total == 0 {
		return 0
	}
	return float64(h) / float64(total)
}

var globalStats Stats

// GlobalStats returns the global Stats snapshot (live atomic counters).
func GlobalStats() *Stats { return &globalStats }

// recordHit/recordMiss update both the global stats and (if non-nil) the
// supplied per-call stats pointer.
func recordHit() {
	atomic.AddInt64(&globalStats.Hits, 1)
}

func recordMiss() {
	atomic.AddInt64(&globalStats.Misses, 1)
}

// Key returns a deterministic cache key for the given parameters.
// Inputs are encoded as canonical JSON (object keys sorted) and then
// hashed together with model, schemaVer and promptVer.
func Key(model, schemaVer, promptVer string, inputs any) string {
	canon, err := canonicalJSON(inputs)
	if err != nil {
		canon = []byte(fmt.Sprintf("%v", inputs))
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%s\n", model, schemaVer, promptVer)
	h.Write(canon)
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalJSON marshals v into JSON with object keys recursively sorted.
func canonicalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return nil, err
	}
	return marshalCanonical(generic), nil
}

func marshalCanonical(v any) []byte {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var out []byte
		out = append(out, '{')
		for i, k := range keys {
			if i > 0 {
				out = append(out, ',')
			}
			kb, _ := json.Marshal(k)
			out = append(out, kb...)
			out = append(out, ':')
			out = append(out, marshalCanonical(x[k])...)
		}
		out = append(out, '}')
		return out
	case []any:
		var out []byte
		out = append(out, '[')
		for i, e := range x {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, marshalCanonical(e)...)
		}
		out = append(out, ']')
		return out
	default:
		b, _ := json.Marshal(x)
		return b
	}
}

// WithCache wraps a generator function with a content cache.
//
// On hit it decodes the stored payload into T and calls IncHit. On miss it
// invokes gen() and stores the JSON-encoded result via Put. The second
// return value is true on hit, false on miss.
func WithCache[T any](
	ctx context.Context,
	repo ContentRepo,
	key string,
	model, schemaVer, promptVer string,
	gen func() (T, error),
) (T, bool, error) {
	var zero T
	if repo != nil {
		payload, err := repo.Get(ctx, key)
		if err == nil && len(payload) > 0 {
			var v T
			if err := json.Unmarshal(payload, &v); err == nil {
				recordHit()
				_ = repo.IncHit(ctx, key)
				return v, true, nil
			}
			// stored payload was invalid -> fall through as miss
		}
	}
	recordMiss()
	v, err := gen()
	if err != nil {
		return zero, false, err
	}
	if repo != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return v, false, fmt.Errorf("cache: marshal payload: %w", err)
		}
		if err := repo.Put(ctx, key, b, model, schemaVer, promptVer); err != nil {
			return v, false, fmt.Errorf("cache: put: %w", err)
		}
	}
	return v, false, nil
}
