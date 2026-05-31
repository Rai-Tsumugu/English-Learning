package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/agent/curriculum"
	"github.com/Rai-Tsumugu/English-Learning/internal/agent/generator"
	"github.com/Rai-Tsumugu/English-Learning/internal/cache"
	"github.com/Rai-Tsumugu/English-Learning/internal/chatgpt"
	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
	"github.com/Rai-Tsumugu/English-Learning/internal/oauth"
	"github.com/Rai-Tsumugu/English-Learning/internal/repo"

	_ "modernc.org/sqlite"
)

const (
	pregenSchemaVer = "v1"
	pregenPromptVer = "p1"
)

// runPregenerate は前夜23時バッチで全ユーザー分のセッション素材を
// 事前生成しキャッシュへ保存する (T28)。未ログインなら no-op で正常終了する。
func runPregenerate(ctx context.Context, cfg *config.Config) error {
	store, err := oauth.NewFileStore(cfg.OAuthTokenPath)
	if err != nil {
		fmt.Println("pregenerate: skip: oauth store init:", err)
		return nil
	}
	flow := oauth.New(store)
	if _, err := flow.Ensure(ctx); err != nil {
		fmt.Println("pregenerate: skip: not logged in (run `app login`)")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.AppDBPath), 0o755); err != nil {
		return fmt.Errorf("mkdir db dir: %w", err)
	}
	d, err := sql.Open(appdb.DriverName, cfg.AppDBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := d.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("enable FK: %w", err)
	}

	rl := llm.NewRateLimiter(5*time.Minute, 5)
	chat := chatgpt.New(&oauthTokenProvider{flow: flow}, rl)
	contentRepo := &pregenCacheRepo{r: repo.NewContent(d)}
	words := repo.NewWords(d)

	rows, err := d.QueryContext(ctx, `SELECT id, COALESCE(cefr_self,'') FROM users ORDER BY id`)
	if err != nil {
		return fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	type userRow struct {
		id   int64
		cefr string
	}
	var users []userRow
	for rows.Next() {
		var u userRow
		if err := rows.Scan(&u.id, &u.cefr); err != nil {
			return fmt.Errorf("scan user: %w", err)
		}
		if u.cefr == "" {
			u.cefr = "A2"
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("pregenerate: no users found")
		return nil
	}

	for _, u := range users {
		if err := pregenerateForUser(ctx, chat, cfg.ModelGenerator, words, contentRepo, u.id, u.cefr); err != nil {
			fmt.Fprintf(os.Stderr, "user %d: error: %v\n", u.id, err)
			continue
		}
	}
	return nil
}

func pregenerateForUser(
	ctx context.Context,
	chat llm.Chat,
	model string,
	words *repo.Words,
	contentRepo cache.ContentRepo,
	userID int64,
	cefr string,
) error {
	ws, err := words.ListByCEFR(ctx, cefr, 12)
	if err != nil {
		return fmt.Errorf("list by cefr: %w", err)
	}
	if len(ws) == 0 {
		fmt.Printf("user %d: skip (no words for CEFR=%s)\n", userID, cefr)
		return nil
	}
	targets := make([]generator.TargetWord, 0, len(ws))
	candidates := make([]curriculum.DueWord, 0, len(ws))
	for _, w := range ws {
		targets = append(targets, generator.TargetWord{
			Lemma:   w.Lemma,
			CEFR:    w.CEFR.String,
			GlossJA: w.GlossJA.String,
		})
		candidates = append(candidates, curriculum.DueWord{Lemma: w.Lemma, CEFR: w.CEFR.String})
	}

	cAgent := curriculum.New(chat, model)
	if _, err := cAgent.Plan(ctx, curriculum.Input{
		UserCEFR:          cefr,
		NewWordCandidates: candidates,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "user %d: curriculum: %v (fallback to direct targets)\n", userID, err)
	}

	genInput := generator.Input{
		Words:     targets,
		UserCEFR:  cefr,
		PromptVer: pregenPromptVer,
	}
	key := cache.Key(model, pregenSchemaVer, pregenPromptVer, genInput)

	if payload, err := contentRepo.Get(ctx, key); err == nil && len(payload) > 0 {
		fmt.Printf("user %d: cached=hit\n", userID)
		_ = contentRepo.IncHit(ctx, key)
		return nil
	}

	out, _, err := cache.WithCache[generator.Output](
		ctx, contentRepo, key, model, pregenSchemaVer, pregenPromptVer,
		func() (generator.Output, error) {
			return generator.Generate(ctx, chat, model, genInput)
		},
	)
	if err != nil {
		return fmt.Errorf("generator: %w", err)
	}
	fmt.Printf("user %d: cached=miss, saved (items=%d)\n", userID, len(out.Items))
	return nil
}

// pregenCacheRepo は repo.Content を cache.ContentRepo に適合させる
// 最小アダプタ (handler 側の同等実装と独立)。
type pregenCacheRepo struct{ r *repo.Content }

func (a *pregenCacheRepo) Get(ctx context.Context, key string) ([]byte, error) {
	g, err := a.r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return []byte(g.PayloadJSON), nil
}

func (a *pregenCacheRepo) Put(ctx context.Context, key string, payload []byte, model, schemaVer, promptVer string) error {
	return a.r.Put(ctx, &repo.GeneratedContent{
		CacheKey:    key,
		Model:       model,
		SchemaVer:   schemaVer,
		PromptVer:   promptVer,
		PayloadJSON: string(payload),
	})
}

func (a *pregenCacheRepo) IncHit(ctx context.Context, key string) error {
	return a.r.IncHit(ctx, key)
}
