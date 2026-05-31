package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Flow は PKCE OAuth フローの実行単位。
type Flow struct {
	Store       Store
	HTTP        *http.Client
	OpenBrowser func(url string) error

	// 以下は主にテストで差し替えるためのフック (空ならパッケージ var を使用)。
	AuthorizeEndpointOverride string
	TokenEndpointOverride     string
	RedirectHost              string // default 127.0.0.1
	Port                      int    // default LocalCallbackPort; 0 → 自動採番
	Timeout                   time.Duration
}

// New はデフォルト設定の Flow を返す。
func New(store Store) *Flow {
	return &Flow{
		Store:       store,
		HTTP:        http.DefaultClient,
		OpenBrowser: defaultOpenBrowser,
		Timeout:     5 * time.Minute,
	}
}

func defaultOpenBrowser(rawurl string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawurl)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawurl)
	default:
		cmd = exec.Command("xdg-open", rawurl)
	}
	return cmd.Start()
}

func (f *Flow) authorizeEndpoint() string {
	if f.AuthorizeEndpointOverride != "" {
		return f.AuthorizeEndpointOverride
	}
	return AuthorizeEndpoint
}

func (f *Flow) tokenEndpoint() string {
	if f.TokenEndpointOverride != "" {
		return f.TokenEndpointOverride
	}
	return TokenEndpoint
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkcePair returns (verifier, challenge S256).
func pkcePair() (string, string, error) {
	v, err := randomURLSafe(64) // 86 chars
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(v))
	c := base64.RawURLEncoding.EncodeToString(sum[:])
	return v, c, nil
}

type callbackResult struct {
	code  string
	state string
	err   error
}

// Login executes the PKCE authorization code flow.
func (f *Flow) Login(ctx context.Context) (*Token, error) {
	if f.HTTP == nil {
		f.HTTP = http.DefaultClient
	}
	if f.Timeout == 0 {
		f.Timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, f.Timeout)
	defer cancel()

	verifier, challenge, err := pkcePair()
	if err != nil {
		return nil, fmt.Errorf("oauth: pkce: %w", err)
	}
	state, err := randomURLSafe(24)
	if err != nil {
		return nil, fmt.Errorf("oauth: state: %w", err)
	}

	host := f.RedirectHost
	if host == "" {
		host = RedirectHost
	}
	port := f.Port
	if port == 0 {
		port = LocalCallbackPort
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("oauth: listen %s:%d: %w", host, port, err)
	}
	defer ln.Close()
	actualPort := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://%s:%d%s", host, actualPort, RedirectPath)

	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(RedirectPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if errParam := q.Get("error"); errParam != "" {
			http.Error(w, errParam, http.StatusBadRequest)
			resultCh <- callbackResult{err: fmt.Errorf("oauth: callback error: %s (%s)", errParam, q.Get("error_description"))}
			return
		}
		code := q.Get("code")
		gotState := q.Get("state")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			resultCh <- callbackResult{err: errors.New("oauth: callback missing code")}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<html><body><h1>Authentication successful</h1><p>You can close this tab.</p></body></html>")
		resultCh <- callbackResult{code: code, state: gotState}
	})

	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// Build authorize URL.
	u, err := url.Parse(f.authorizeEndpoint())
	if err != nil {
		return nil, fmt.Errorf("oauth: parse authorize: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", Scope)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	// Codex CLI の login フローと同じ追加パラメータ。これらが無いと組織選択や
	// 簡易フローが有効にならず、id_token に必要なクレームが欠けることがある。
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	u.RawQuery = q.Encode()

	if f.OpenBrowser != nil {
		if err := f.OpenBrowser(u.String()); err != nil {
			return nil, fmt.Errorf("oauth: open browser: %w", err)
		}
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("oauth: login timeout: %w", ctx.Err())
	case res := <-resultCh:
		if res.err != nil {
			return nil, res.err
		}
		if res.state != state {
			return nil, errors.New("oauth: state mismatch")
		}
		return f.exchange(ctx, res.code, verifier, redirectURI)
	}
}

func (f *Flow) exchange(ctx context.Context, code, verifier, redirectURI string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", ClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)
	tok, err := f.postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	if f.Store != nil {
		if err := f.Store.Save(tok); err != nil {
			return nil, fmt.Errorf("oauth: save: %w", err)
		}
	}
	return tok, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

func (f *Flow) postToken(ctx context.Context, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenEndpoint(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oauth: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("oauth: token endpoint status %d: %s", resp.StatusCode, string(body))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("oauth: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, errors.New("oauth: empty access_token")
	}
	tok := &Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		IDToken:      tr.IDToken,
		TokenType:    tr.TokenType,
	}
	if tr.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	if claims, err := decodeIDTokenClaims(tr.IDToken); err == nil {
		tok.Account = TokenAccount{
			Email:     claims.Email,
			Plan:      claims.Plan,
			AccountID: firstNonEmpty(claims.Sub, claims.AccountID),
		}
	}
	return tok, nil
}

type idTokenClaims struct {
	Sub       string `json:"sub"`
	Email     string `json:"email"`
	Plan      string `json:"https://api.openai.com/auth/plan"`
	AccountID string `json:"https://api.openai.com/auth/account_id"`
}

func decodeIDTokenClaims(idToken string) (*idTokenClaims, error) {
	if idToken == "" {
		return nil, errors.New("empty id_token")
	}
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return nil, errors.New("malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some JWTs may use standard base64; try with padding.
		payload, err = base64.URLEncoding.DecodeString(padBase64(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("decode payload: %w", err)
		}
	}
	// Tolerant of unknown claim shapes.
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	c := &idTokenClaims{}
	if v, ok := raw["sub"].(string); ok {
		c.Sub = v
	}
	if v, ok := raw["email"].(string); ok {
		c.Email = v
	}
	if v, ok := raw["https://api.openai.com/auth/plan"].(string); ok {
		c.Plan = v
	}
	if v, ok := raw["plan"].(string); ok && c.Plan == "" {
		c.Plan = v
	}
	if v, ok := raw["https://api.openai.com/auth/account_id"].(string); ok {
		c.AccountID = v
	}
	return c, nil
}

func padBase64(s string) string {
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return s
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// Refresh exchanges a refresh_token for a new access_token.
func (f *Flow) Refresh(ctx context.Context, t *Token) (*Token, error) {
	if t == nil || t.RefreshToken == "" {
		return nil, errors.New("oauth: no refresh_token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", ClientID)
	form.Set("refresh_token", t.RefreshToken)
	form.Set("scope", Scope)
	newTok, err := f.postToken(ctx, form)
	if err != nil {
		return nil, err
	}
	// Some IdPs omit refresh_token on refresh — preserve previous.
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = t.RefreshToken
	}
	// Preserve account info if new id_token absent.
	if newTok.Account.Email == "" {
		newTok.Account = t.Account
	}
	if f.Store != nil {
		if err := f.Store.Save(newTok); err != nil {
			return nil, fmt.Errorf("oauth: save refresh: %w", err)
		}
	}
	return newTok, nil
}

// Ensure loads a token, refreshing if necessary.
func (f *Flow) Ensure(ctx context.Context) (*Token, error) {
	if f.Store == nil {
		return nil, errors.New("oauth: no store")
	}
	t, err := f.Store.Load()
	if err != nil {
		return nil, err
	}
	if !t.Expired() {
		return t, nil
	}
	return f.Refresh(ctx, t)
}

// Logout clears persisted token.
func (f *Flow) Logout() error {
	if f.Store == nil {
		return nil
	}
	return f.Store.Delete()
}
