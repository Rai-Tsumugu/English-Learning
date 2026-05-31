package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

type memProvider struct{ items []PlacementItem }

func (m *memProvider) List(_ context.Context) ([]PlacementItem, error) {
	out := make([]PlacementItem, len(m.items))
	copy(out, m.items)
	return out, nil
}
func (m *memProvider) Get(_ context.Context, id string) (*PlacementItem, error) {
	for _, it := range m.items {
		if it.ID == id {
			c := it
			return &c, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func newPlacementTestRouter(items []PlacementItem) http.Handler {
	r := chi.NewRouter()
	h := NewPlacementHandlerWithProvider(&memProvider{items: items})
	r.Route("/api", func(api chi.Router) {
		h.Mount(api)
	})
	return r
}

func samplePool() []PlacementItem {
	out := make([]PlacementItem, 0, 30)
	// b を -2.5 .. 2.5 まで広く分布させる
	for i := 0; i < 30; i++ {
		b := -2.5 + float64(i)*0.2
		out = append(out, PlacementItem{
			ID: fmt.Sprintf("q%d", i), Prompt: "p", Choices: []string{"a", "b", "c", "d"},
			Answer: 0, A: 1.2, B: b,
		})
	}
	return out
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPlacement_StartAndAnswerFlow(t *testing.T) {
	h := newPlacementTestRouter(samplePool())

	rec := doJSON(t, h, http.MethodPost, "/api/placement/start", map[string]int64{"user_id": 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}
	var sr startResp
	if err := json.Unmarshal(rec.Body.Bytes(), &sr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sr.SessionID == "" || sr.Item == nil {
		t.Fatalf("bad start resp: %+v", sr)
	}

	// 常に正答で進める。20問以内に done になるか、SEM 停止
	curItem := sr.Item.ID
	correct := true
	for i := 0; i < 25; i++ {
		rec := doJSON(t, h, http.MethodPost, "/api/placement/answer", answerReq{
			SessionID: sr.SessionID, ItemID: curItem, Correct: &correct,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("answer status=%d body=%s", rec.Code, rec.Body.String())
		}
		var ar answerResp
		if err := json.Unmarshal(rec.Body.Bytes(), &ar); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if ar.Done {
			if ar.CEFR == "" {
				t.Fatalf("expected CEFR on done")
			}
			return
		}
		if ar.Item == nil {
			t.Fatalf("expected next item")
		}
		curItem = ar.Item.ID
	}
	t.Fatalf("did not terminate within 25 iterations")
}

func TestPlacement_BadRequests(t *testing.T) {
	h := newPlacementTestRouter(samplePool())
	// user_id が数値でない場合はデコードエラーで 400。
	rec := doJSON(t, h, http.MethodPost, "/api/placement/start", map[string]string{"user_id": "not-a-number"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/placement/answer", answerReq{SessionID: "missing", ItemID: "q0"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 session not found, got %d", rec.Code)
	}
}
