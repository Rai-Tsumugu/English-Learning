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

func TestOnboarding_Post(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewOnboardingHandler(d).Mount(api)
	})

	body, _ := json.Marshal(map[string]any{"cefr_self": "A2"})
	req := httptest.NewRequest(http.MethodPost, "/api/onboarding", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp onboardingResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.UserID == 0 || resp.CEFRSelf != "A2" || resp.Theta != -1.0 {
		t.Errorf("unexpected resp: %+v", resp)
	}
}

func TestOnboarding_InvalidCEFR(t *testing.T) {
	d, err := appdb.Open(context.Background(), filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	r := chi.NewRouter()
	r.Route("/api", func(api chi.Router) {
		NewOnboardingHandler(d).Mount(api)
	})

	body, _ := json.Marshal(map[string]any{"cefr_self": "X9"})
	req := httptest.NewRequest(http.MethodPost, "/api/onboarding", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rr.Code)
	}
}
