package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	"github.com/Rai-Tsumugu/English-Learning/internal/oauth"
)

func newFlow(cfg *config.Config) (*oauth.Flow, oauth.Store, error) {
	path := ""
	if cfg != nil {
		path = cfg.OAuthTokenPath
	}
	store, err := oauth.NewFileStore(path)
	if err != nil {
		return nil, nil, err
	}
	return oauth.New(store), store, nil
}

func runLogin(ctx context.Context, cfg *config.Config) error {
	flow, store, err := newFlow(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("Opening browser for ChatGPT authentication...\n")
	tok, err := flow.Login(ctx)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	fmt.Printf("Authentication successful.\n")
	fmt.Printf("  email   : %s\n", tok.Account.Email)
	if tok.Account.Plan != "" {
		fmt.Printf("  plan    : %s\n", tok.Account.Plan)
	}
	if !tok.Expiry.IsZero() {
		fmt.Printf("  expiry  : %s\n", tok.Expiry.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("  saved to: %s\n", store.Path())
	return nil
}

func runLogout(_ context.Context, cfg *config.Config) error {
	flow, store, err := newFlow(cfg)
	if err != nil {
		return err
	}
	if err := flow.Logout(); err != nil {
		return err
	}
	fmt.Printf("Logged out. Removed %s\n", store.Path())
	return nil
}

func runWhoami(ctx context.Context, cfg *config.Config) error {
	flow, _, err := newFlow(cfg)
	if err != nil {
		return err
	}
	tok, err := flow.Ensure(ctx)
	if err != nil {
		if errors.Is(err, oauth.ErrNotFound) {
			return fmt.Errorf("not logged in. Run `app login` first")
		}
		return err
	}
	fmt.Printf("email : %s\n", tok.Account.Email)
	if tok.Account.Plan != "" {
		fmt.Printf("plan  : %s\n", tok.Account.Plan)
	}
	if !tok.Expiry.IsZero() {
		fmt.Printf("expiry: %s\n", tok.Expiry.Format("2006-01-02 15:04:05"))
	}
	return nil
}
