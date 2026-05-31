package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/go-chi/chi/v5"

	_ "modernc.org/sqlite"
)

func TestStats_Weekly(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()

	users := repo.NewUsers(d)
	uid, _ := users.Create(ctx, "B1", 0, 1)
	words := repo.NewWords(d)
	w1, _ := words.Create(ctx, "apple", "A1", "n", "", 1)
	_, _ = words.Create(ctx, "banana", "A1", "n", "", 2)

	attempts := repo.NewAttempts(d)
	now := time.Now().UTC()
	_, err = attempts.Insert(ctx, &repo.Attempt{
		UserID: uid, WordID: w1, Correct: true, LatencyMS: 1000,
		Quality:      sql.NullInt64{Int64: 5, Valid: true},
		Ease:         sql.NullFloat64{Float64: 2.5, Valid: true},
		IntervalDays: sql.NullFloat64{Float64: 1, Valid: true},
		Reps:         sql.NullInt64{Int64: 1, Valid: true},
		Lapses:       sql.NullInt64{Int64: 0, Valid: true},
		NextReviewAt: sql.NullTime{Time: now.AddDate(0, 0, 1), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	// generated_content にダミーキャッシュを入れて hit_rate を非ゼロに
	content := repo.NewContent(d)
	_ = content.Put(ctx, &repo.GeneratedContent{
		CacheKey: "k1", Model: "m", SchemaVer: "v1", PromptVer: "p1", PayloadJSON: "{}",
	})
	_ = content.IncHit(ctx, "k1")
	_ = content.IncHit(ctx, "k1")

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewStatsHandler(d).Mount(api)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/stats/weekly", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp weeklyResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TotalAttempts7d != 1 {
		t.Errorf("expected 1 attempt, got %d", resp.TotalAttempts7d)
	}
	if resp.RemainingWords != 1 { // 2 words - 1 learned
		t.Errorf("expected remaining=1, got %d", resp.RemainingWords)
	}
	if resp.StreakDays < 1 {
		t.Errorf("expected streak >= 1, got %d", resp.StreakDays)
	}
	// 2 hits / (2 + 1 row) = 0.666...
	if resp.CacheHitRate <= 0 || resp.CacheHitRate > 1 {
		t.Errorf("unexpected hit rate: %v", resp.CacheHitRate)
	}
}
