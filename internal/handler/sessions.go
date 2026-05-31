package handler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/agent/curriculum"
	"github.com/Rai-Tsumugu/English-Learning/internal/agent/generator"
	"github.com/Rai-Tsumugu/English-Learning/internal/agent/reviewer"
	"github.com/Rai-Tsumugu/English-Learning/internal/cache"
	"github.com/Rai-Tsumugu/English-Learning/internal/degrade"
	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/Rai-Tsumugu/English-Learning/internal/sse"
	"github.com/go-chi/chi/v5"
)

const (
	schemaVer = "v1"
	promptVer = "p1"
)

// SessionsHandler は /api/sessions/today を提供する (T19)。
// Curriculum→Generator→Reviewer→cache→SSE の直列パイプラインを束ねる。
type SessionsHandler struct {
	db        *sql.DB
	chat      llm.Chat
	model     string
	reviewer  *reviewer.Sampler
	decider   *degrade.Decider
	cacheRepo cache.ContentRepo
}

// NewSessionsHandler は依存を注入して SessionsHandler を生成する。
// chat が nil の場合、ハンドラは SSE error イベントを返す（未ログイン環境向け）。
func NewSessionsHandler(db *sql.DB, chat llm.Chat, model string, decider *degrade.Decider) *SessionsHandler {
	return &SessionsHandler{
		db:        db,
		chat:      chat,
		model:     model,
		reviewer:  reviewer.NewSampler(0.10, []string{"cloze"}),
		decider:   decider,
		cacheRepo: newContentRepoAdapter(repo.NewContent(db)),
	}
}

func (h *SessionsHandler) Mount(r chi.Router) {
	r.Get("/sessions/today", h.handleToday)
}

func (h *SessionsHandler) handleToday(w http.ResponseWriter, r *http.Request) {
	userID, _ := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	userCEFR := strings.ToUpper(r.URL.Query().Get("cefr"))
	if userCEFR == "" {
		userCEFR = "A2"
	}

	wr, err := sse.New(w)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	go wr.Heartbeat(ctx, 15*time.Second)

	if h.chat == nil {
		_ = wr.Error(errors.New("chat client not configured (run `app login`)"))
		return
	}

	plan, items, err := h.buildSession(ctx, userID, userCEFR)
	if err != nil {
		_ = wr.Error(err)
		return
	}

	_ = wr.Plan(plan)
	for _, it := range items {
		_ = wr.Question(it)
	}
	_ = wr.Done(map[string]any{"count": len(items)})
}

func (h *SessionsHandler) buildSession(ctx context.Context, userID int64, userCEFR string) (curriculum.Plan, []generator.Item, error) {
	dueWords, newCandidates, err := h.loadCandidates(ctx, userID, userCEFR)
	if err != nil {
		return curriculum.Plan{}, nil, fmt.Errorf("load candidates: %w", err)
	}

	// 1. Curriculum
	cAgent := curriculum.New(h.chat, h.model)
	plan, err := cAgent.Plan(ctx, curriculum.Input{
		UserCEFR:             userCEFR,
		DueWords:             dueWords,
		NewWordCandidates:    newCandidates,
		DaysSinceLastSession: 0,
	})
	if err != nil {
		return curriculum.Plan{}, nil, fmt.Errorf("curriculum: %w", err)
	}

	// 2. Generator (with cache)
	genInput := generator.Input{
		Words:     targetWordsFromPlan(plan),
		UserCEFR:  userCEFR,
		PromptVer: promptVer,
	}
	key := cache.Key(h.model, schemaVer, promptVer, genInput)

	if h.decider != nil && !h.decider.AllowGenerator() {
		return plan, nil, errors.New("generator disabled by degradation level (L4)")
	}

	out, _, err := cache.WithCache[generator.Output](
		ctx, h.cacheRepo, key, h.model, schemaVer, promptVer,
		func() (generator.Output, error) {
			return generator.Generate(ctx, h.chat, h.model, genInput)
		},
	)
	if err != nil {
		return plan, nil, fmt.Errorf("generator: %w", err)
	}

	// 3. Reviewer (sampling)
	if h.decider == nil || h.decider.AllowReviewer() {
		for i := range out.Items {
			if h.reviewer.ShouldReview(out.Items[i]) {
				dec, rerr := reviewer.ReviewOne(ctx, h.chat, h.model, out.Items[i])
				if rerr == nil {
					h.reviewer.Observe(dec.Pass)
				}
			}
		}
	}

	return plan, out.Items, nil
}

func (h *SessionsHandler) loadCandidates(ctx context.Context, userID int64, userCEFR string) (due, fresh []curriculum.DueWord, err error) {
	attempts := repo.NewAttempts(h.db)
	words := repo.NewWords(h.db)

	dueRows, err := attempts.ListDue(ctx, userID, time.Now(), 20)
	if err != nil {
		return nil, nil, err
	}
	for _, a := range dueRows {
		w, werr := words.GetByID(ctx, a.WordID)
		if werr != nil {
			continue
		}
		q := 0
		if a.Quality.Valid {
			q = int(a.Quality.Int64)
		}
		days := 0
		if a.NextReviewAt.Valid {
			days = int(time.Since(a.NextReviewAt.Time).Hours() / 24)
		}
		due = append(due, curriculum.DueWord{
			Lemma:           w.Lemma,
			CEFR:            w.CEFR.String,
			LastQuality:     q,
			DaysSinceReview: days,
		})
	}

	fresh = []curriculum.DueWord{}
	fresh = appendNewCandidatesFromCEFR(ctx, words, userCEFR, fresh, 20)
	return due, fresh, nil
}

func appendNewCandidatesFromCEFR(ctx context.Context, words *repo.Words, cefr string, acc []curriculum.DueWord, limit int) []curriculum.DueWord {
	rows, err := words.ListByCEFR(ctx, cefr, limit)
	if err != nil {
		return acc
	}
	for _, w := range rows {
		acc = append(acc, curriculum.DueWord{Lemma: w.Lemma, CEFR: w.CEFR.String})
	}
	return acc
}

func targetWordsFromPlan(p curriculum.Plan) []generator.TargetWord {
	out := make([]generator.TargetWord, 0, len(p.Targets))
	for _, t := range p.Targets {
		out = append(out, generator.TargetWord{
			Lemma:   t.Lemma,
			CEFR:    t.CEFR,
			GlossJA: "",
		})
	}
	return out
}

// contentRepoAdapter は repo.Content を cache.ContentRepo に適合させる。
type contentRepoAdapter struct{ r *repo.Content }

func newContentRepoAdapter(r *repo.Content) cache.ContentRepo { return &contentRepoAdapter{r: r} }

func (a *contentRepoAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	g, err := a.r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return []byte(g.PayloadJSON), nil
}

func (a *contentRepoAdapter) Put(ctx context.Context, key string, payload []byte, model, schemaVer, promptVer string) error {
	return a.r.Put(ctx, &repo.GeneratedContent{
		CacheKey:    key,
		Model:       model,
		SchemaVer:   schemaVer,
		PromptVer:   promptVer,
		PayloadJSON: string(payload),
	})
}

func (a *contentRepoAdapter) IncHit(ctx context.Context, key string) error {
	return a.r.IncHit(ctx, key)
}
