package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/go-chi/chi/v5"

	_ "modernc.org/sqlite"
)

func TestAttempts_Post(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	users := repo.NewUsers(d)
	uid, err := users.Create(context.Background(), "B1", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	words := repo.NewWords(d)
	wid, err := words.Create(context.Background(), "apple", "A1", "n", "りんご", 100)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewAttemptsHandler(d).Mount(api)
	})

	body, _ := json.Marshal(map[string]any{
		"user_id":    uid,
		"word_id":    wid,
		"correct":    true,
		"latency_ms": 1500,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/attempts", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp attemptResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AttemptID == 0 || resp.Quality != 5 || resp.Reps != 1 || resp.IntervalDays != 1 {
		t.Errorf("unexpected resp: %+v", resp)
	}

	// 2回目 — 直近 state からの継続が効くか確認
	body2, _ := json.Marshal(map[string]any{
		"user_id":    uid,
		"word_id":    wid,
		"correct":    true,
		"latency_ms": 1500,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/attempts", bytes.NewReader(body2))
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusCreated {
		t.Fatalf("2nd status=%d body=%s", rr2.Code, rr2.Body.String())
	}
	var resp2 attemptResp
	_ = json.Unmarshal(rr2.Body.Bytes(), &resp2)
	if resp2.Reps != 2 || resp2.IntervalDays != 6 {
		t.Errorf("expected reps=2 interval=6, got %+v", resp2)
	}
}
