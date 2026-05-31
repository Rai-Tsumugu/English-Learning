package repo

import (
	"context"
	"database/sql"
	"time"
)

// GeneratedContent mirrors the generated_content row.
type GeneratedContent struct {
	CacheKey    string
	Model       string
	SchemaVer   string
	PromptVer   string
	PayloadJSON string
	CreatedAt   time.Time
	HitCount    int64
}

// Content is the generated-content cache repository.
type Content struct{ db *sql.DB }

// NewContent constructs a Content repository.
func NewContent(db *sql.DB) *Content { return &Content{db: db} }

// Get returns a cached entry by key, or sql.ErrNoRows if absent.
func (r *Content) Get(ctx context.Context, key string) (*GeneratedContent, error) {
	row := r.db.QueryRowContext(ctx, `SELECT cache_key, model, schema_ver, prompt_ver,
		payload_json, created_at, hit_count FROM generated_content WHERE cache_key=?`, key)
	g := &GeneratedContent{}
	if err := row.Scan(&g.CacheKey, &g.Model, &g.SchemaVer, &g.PromptVer,
		&g.PayloadJSON, &g.CreatedAt, &g.HitCount); err != nil {
		return nil, err
	}
	return g, nil
}

// Put inserts or replaces a cache entry. hit_count is reset to 0.
func (r *Content) Put(ctx context.Context, g *GeneratedContent) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO generated_content(
		cache_key, model, schema_ver, prompt_ver, payload_json, hit_count
	) VALUES (?,?,?,?,?,0)
	ON CONFLICT(cache_key) DO UPDATE SET
		model=excluded.model,
		schema_ver=excluded.schema_ver,
		prompt_ver=excluded.prompt_ver,
		payload_json=excluded.payload_json`,
		g.CacheKey, g.Model, g.SchemaVer, g.PromptVer, g.PayloadJSON)
	return err
}

// IncHit atomically increments hit_count for `key`.
func (r *Content) IncHit(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE generated_content SET hit_count = hit_count + 1 WHERE cache_key=?`, key)
	return err
}
