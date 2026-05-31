package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/chatgpt"
	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/degrade"
	"github.com/Rai-Tsumugu/English-Learning/internal/handler"
	"github.com/Rai-Tsumugu/English-Learning/internal/license"
	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
	"github.com/Rai-Tsumugu/English-Learning/internal/oauth"

	_ "modernc.org/sqlite"
)

// oauthTokenProvider adapts *oauth.Flow to chatgpt.TokenProvider.
type oauthTokenProvider struct{ flow *oauth.Flow }

func (p *oauthTokenProvider) Token(ctx context.Context) (string, error) {
	t, err := p.flow.Ensure(ctx)
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}

func runServe(ctx context.Context, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(cfg.AppDBPath), 0o755); err != nil {
		return fmt.Errorf("mkdir db dir: %w", err)
	}
	d, err := sql.Open(appdb.DriverName, cfg.AppDBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer d.Close()
	if _, err := d.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := d.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("enable FK: %w", err)
	}
	if err := appdb.Migrate(d, "up"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := license.Verify("./LICENSES"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: license check: %v\n", err)
	}
	if err := ensurePlacementSeed(ctx, d); err != nil {
		return fmt.Errorf("seed placement: %w", err)
	}

	// SessionsHandler は常に mount し、未ログイン時はリクエスト時点で
	// SSE error イベントを返す設計にする (200 + event:error)。
	var sessionsH handler.SessionsMounter
	store, serr := oauth.NewFileStore(cfg.OAuthTokenPath)
	if serr != nil {
		fmt.Fprintf(os.Stderr, "warning: oauth store init: %v\n", serr)
	} else {
		flow := oauth.New(store)
		rl := llm.NewRateLimiter(5*time.Minute, 5)
		chat := chatgpt.New(&oauthTokenProvider{flow: flow}, rl)
		decider := degrade.NewDecider(rl)
		sessionsH = handler.NewSessionsHandler(d, chat, cfg.ModelGenerator, decider)
		if _, err := flow.Ensure(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "warning: not logged in (run `app login`) — /api/sessions/today will stream error events until login")
		}
	}
	h := handler.NewRouterFull(cfg, d, sessionsH)

	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.HTTPAddr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("listening on http://%s\n", cfg.HTTPAddr)
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
