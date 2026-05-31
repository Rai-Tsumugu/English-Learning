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
	"github.com/go-chi/chi/v5"

	_ "modernc.org/sqlite"
)

func setupFrictionRouter(t *testing.T) (*chi.Mux, func()) {
	t.Helper()
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewFrictionHandler(d).Mount(api)
	})
	return r, func() { d.Close() }
}

func TestFriction_PostAndRecent(t *testing.T) {
	r, cleanup := setupFrictionRouter(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]any{
		"user_id": 1,
		"kind":    "too_hard",
		"payload": map[string]any{"word_id": 42, "note": "難しすぎる"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/friction", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("post status=%d body=%s", rr.Code, rr.Body.String())
	}
	var pr frictionResp
	if err := json.Unmarshal(rr.Body.Bytes(), &pr); err != nil {
		t.Fatal(err)
	}
	if pr.ID == 0 {
		t.Errorf("expected non-zero id")
	}

	// もう一件挿入
	body2, _ := json.Marshal(map[string]any{"user_id": 2, "kind": "freetext", "payload": map[string]any{"text": "hi"}})
	req2 := httptest.NewRequest(http.MethodPost, "/api/friction", bytes.NewReader(body2))
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusCreated {
		t.Fatalf("post2 status=%d", rr2.Code)
	}

	// GET recent
	gr := httptest.NewRequest(http.MethodGet, "/api/friction/recent?limit=10", nil)
	gw := httptest.NewRecorder()
	r.ServeHTTP(gw, gr)
	if gw.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", gw.Code, gw.Body.String())
	}
	var resp struct {
		Items []frictionRecord `json:"items"`
	}
	if err := json.Unmarshal(gw.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
	// 新しい順
	if resp.Items[0].Kind != "freetext" {
		t.Errorf("expected first item kind=freetext, got %s", resp.Items[0].Kind)
	}
}

func TestFriction_InvalidKind(t *testing.T) {
	r, cleanup := setupFrictionRouter(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]any{"user_id": 1, "kind": "bogus", "payload": map[string]any{}})
	req := httptest.NewRequest(http.MethodPost, "/api/friction", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestFriction_InvalidJSON(t *testing.T) {
	r, cleanup := setupFrictionRouter(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "/api/friction", bytes.NewReader([]byte("{bad")))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
