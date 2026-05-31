package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/go-chi/chi/v5"

	_ "modernc.org/sqlite"
)

func TestWords_Get(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	words := repo.NewWords(d)
	wid, err := words.Create(context.Background(), "apple", "A1", "n", "りんご", 100)
	if err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewWordsHandler(d).Mount(api)
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/words/%d", wid), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto wordDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Lemma != "apple" || dto.CEFR != "A1" {
		t.Errorf("unexpected dto: %+v", dto)
	}

	// 存在しない id は 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/words/99999", nil)
	rr404 := httptest.NewRecorder()
	r.ServeHTTP(rr404, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", rr404.Code)
	}
}

func TestWords_Neighbors(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	words := repo.NewWords(d)

	// Lexical 3-gram Jaccard を確認: 共通文字列を持つ語が高スコアになる。
	w1, _ := words.Create(ctx, "manage", "B1", "v", "", 1)
	w2, _ := words.Create(ctx, "management", "B1", "n", "", 2)
	w3, _ := words.Create(ctx, "manager", "B1", "n", "", 3)
	_, _ = words.Create(ctx, "xyz", "B1", "n", "", 4)

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewWordsHandler(d).Mount(api)
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/words/%d/neighbors?k=5", w1), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp neighborsResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// management と manager は manage と共通 trigram 多数 → 上位 2 件
	if len(resp.Neighbors) < 2 {
		t.Fatalf("expected >=2 neighbors, got %d (%+v)", len(resp.Neighbors), resp.Neighbors)
	}
	gotIDs := map[int64]bool{}
	for _, n := range resp.Neighbors {
		gotIDs[n.ID] = true
	}
	if !gotIDs[w2] || !gotIDs[w3] {
		t.Errorf("expected w2 and w3 in neighbors, got %+v", resp.Neighbors)
	}

	// 存在しない id は 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/words/99999/neighbors", nil)
	rr404 := httptest.NewRecorder()
	r.ServeHTTP(rr404, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 got %d", rr404.Code)
	}
}
