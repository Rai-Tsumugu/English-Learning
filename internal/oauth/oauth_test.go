package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	b, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	return header + "." + payload + ".sig"
}

func newTokenServer(t *testing.T, want map[string]string, resp map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		for k, v := range want {
			if got := r.PostForm.Get(k); got != v {
				http.Error(w, fmt.Sprintf("form %s: want %q got %q", k, v, got), 400)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "auth.json")
	s, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if got := s.Path(); got != path {
		t.Fatalf("path: got %s want %s", got, path)
	}
	if _, err := s.Load(); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	tok := &Token{
		AccessToken:  "at",
		RefreshToken: "rt",
		Expiry:       time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		Account:      TokenAccount{Email: "x@example.com", Plan: "plus"},
	}
	if err := s.Save(tok); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.AccessToken != "at" || got.RefreshToken != "rt" || got.Account.Email != "x@example.com" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if err := s.Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Load(); err != ErrNotFound {
		t.Fatalf("after delete want ErrNotFound got %v", err)
	}
	// Delete again is no-op.
	if err := s.Delete(); err != nil {
		t.Fatalf("delete idempotent: %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	idTok := makeIDToken(t, map[string]any{
		"sub":   "user_123",
		"email": "alice@example.com",
		"https://api.openai.com/auth/plan": "plus",
	})
	tokSrv := newTokenServer(t,
		map[string]string{"grant_type": "authorization_code", "client_id": ClientID, "code": "the-code"},
		map[string]any{
			"access_token":  "AT1",
			"refresh_token": "RT1",
			"id_token":      idTok,
			"token_type":    "Bearer",
			"expires_in":    3600,
		},
	)
	defer tokSrv.Close()

	// Authorize endpoint: capture redirect_uri + state and call back.
	var authSrv *httptest.Server
	authSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		redir, _ := url.Parse(q.Get("redirect_uri"))
		v := url.Values{}
		v.Set("code", "the-code")
		v.Set("state", q.Get("state"))
		redir.RawQuery = v.Encode()
		// Trigger callback from background to avoid blocking before flow waits.
		go func() {
			_, _ = http.Get(redir.String())
		}()
		w.WriteHeader(200)
	}))
	defer authSrv.Close()

	store, _ := NewFileStore(filepath.Join(t.TempDir(), "auth.json"))
	flow := New(store)
	flow.AuthorizeEndpointOverride = authSrv.URL
	flow.TokenEndpointOverride = tokSrv.URL
	flow.Port = 0 // ephemeral
	flow.Timeout = 5 * time.Second
	flow.OpenBrowser = func(u string) error {
		// Mimic browser: GET authorize URL.
		go func() {
			_, _ = http.Get(u)
		}()
		return nil
	}

	tok, err := flow.Login(context.Background())
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if tok.AccessToken != "AT1" || tok.RefreshToken != "RT1" {
		t.Fatalf("tokens: %+v", tok)
	}
	if tok.Account.Email != "alice@example.com" || tok.Account.Plan != "plus" {
		t.Fatalf("account: %+v", tok.Account)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if loaded.AccessToken != "AT1" {
		t.Fatalf("not persisted: %+v", loaded)
	}
}

func TestRefresh(t *testing.T) {
	tokSrv := newTokenServer(t,
		map[string]string{"grant_type": "refresh_token", "refresh_token": "RT1"},
		map[string]any{
			"access_token": "AT2",
			"token_type":   "Bearer",
			"expires_in":   3600,
		},
	)
	defer tokSrv.Close()

	store, _ := NewFileStore(filepath.Join(t.TempDir(), "auth.json"))
	flow := New(store)
	flow.TokenEndpointOverride = tokSrv.URL

	old := &Token{AccessToken: "AT1", RefreshToken: "RT1", Account: TokenAccount{Email: "a@b"}}
	newTok, err := flow.Refresh(context.Background(), old)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if newTok.AccessToken != "AT2" {
		t.Fatalf("access_token: %s", newTok.AccessToken)
	}
	if newTok.RefreshToken != "RT1" {
		t.Fatalf("refresh_token should be preserved, got %q", newTok.RefreshToken)
	}
	if newTok.Account.Email != "a@b" {
		t.Fatalf("account preserved: %+v", newTok.Account)
	}
}

func TestLoginStateMismatch(t *testing.T) {
	tokSrv := newTokenServer(t, nil, map[string]any{"access_token": "x"})
	defer tokSrv.Close()

	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		redir, _ := url.Parse(q.Get("redirect_uri"))
		v := url.Values{}
		v.Set("code", "c")
		v.Set("state", "WRONG") // intentionally mismatched
		redir.RawQuery = v.Encode()
		go func() { _, _ = http.Get(redir.String()) }()
		w.WriteHeader(200)
	}))
	defer authSrv.Close()

	store, _ := NewFileStore(filepath.Join(t.TempDir(), "auth.json"))
	flow := New(store)
	flow.AuthorizeEndpointOverride = authSrv.URL
	flow.TokenEndpointOverride = tokSrv.URL
	flow.Port = 0
	flow.Timeout = 3 * time.Second
	flow.OpenBrowser = func(u string) error {
		go func() { _, _ = http.Get(u) }()
		return nil
	}

	_, err := flow.Login(context.Background())
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("expected state mismatch error, got %v", err)
	}
}
