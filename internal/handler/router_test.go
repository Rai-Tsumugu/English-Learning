package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	"github.com/go-chi/chi/v5"
)

func newTestCfg() *config.Config {
	return &config.Config{
		HTTPAddr:      "127.0.0.1:0",
		AllowedOrigin: "http://127.0.0.1:5173",
	}
}

func TestHealthz(t *testing.T) {
	h := NewRouter(newTestCfg())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("expected body ok, got %q", rec.Body.String())
	}
}

func TestApiVersion(t *testing.T) {
	h := NewRouter(newTestCfg())
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if body["version"] == "" {
		t.Fatalf("version empty")
	}
}

func TestCORS_AllowedOrigin(t *testing.T) {
	h := NewRouter(newTestCfg())
	req := httptest.NewRequest(http.MethodOptions, "/api/version", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	got := rec.Header().Get("Access-Control-Allow-Origin")
	if got != "http://127.0.0.1:5173" {
		t.Fatalf("expected ACAO=allowed, got %q (status=%d)", got, rec.Code)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	h := NewRouter(newTestCfg())
	req := httptest.NewRequest(http.MethodOptions, "/api/version", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO header for disallowed origin, got %q", got)
	}
}

func TestRecovererReturnsProblem(t *testing.T) {
	cfg := newTestCfg()
	h := NewRouter(cfg).(*chi.Mux)
	h.Get("/panic", func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/problem+json") {
		t.Fatalf("expected problem+json content-type, got %q", ct)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if int(p["status"].(float64)) != 500 {
		t.Fatalf("expected status=500 in body, got %v", p["status"])
	}
	if p["title"] == "" {
		t.Fatalf("title empty")
	}
}

func TestCommaSeparatedAllowedOrigins(t *testing.T) {
	cfg := &config.Config{
		HTTPAddr:      "127.0.0.1:0",
		AllowedOrigin: "http://127.0.0.1:5173, http://localhost:5173",
	}
	h := NewRouter(cfg)
	req := httptest.NewRequest(http.MethodOptions, "/api/version", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected localhost allowed, got %q", got)
	}
}
