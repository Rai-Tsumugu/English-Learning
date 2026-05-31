package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TokenAccount は id_token から抽出したアカウント情報。
type TokenAccount struct {
	Email     string `json:"email,omitempty"`
	Plan      string `json:"plan,omitempty"`
	AccountID string `json:"account_id,omitempty"`
}

// Token は OAuth トークン一式と派生メタ。
type Token struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	IDToken      string       `json:"id_token,omitempty"`
	TokenType    string       `json:"token_type,omitempty"`
	Expiry       time.Time    `json:"expiry"`
	Account      TokenAccount `json:"account"`
}

// Expired returns true if the access token is expired or near expiry (60s skew).
func (t *Token) Expired() bool {
	if t == nil || t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(60 * time.Second).After(t.Expiry)
}

// Store はトークンの永続化抽象。
type Store interface {
	Load() (*Token, error)
	Save(*Token) error
	Delete() error
	Path() string
}

// ErrNotFound はトークン未保存時に返る。
var ErrNotFound = errors.New("oauth: token not found")

// FileStore は ~/.config/english-learning/auth.json への保存実装。
type FileStore struct {
	path string
}

// DefaultPath は XDG_CONFIG_HOME を尊重した保存先を返す。
func DefaultPath() (string, error) {
	if p := os.Getenv("OAUTH_TOKEN_PATH"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("oauth: resolve home: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "english-learning", "auth.json"), nil
}

// NewFileStore は与えられたパス (空ならデフォルト) で FileStore を構築する。
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	return &FileStore{path: path}, nil
}

// Path returns the on-disk path.
func (s *FileStore) Path() string { return s.path }

// Load reads and unmarshals the token; returns ErrNotFound if absent.
func (s *FileStore) Load() (*Token, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("oauth: read token: %w", err)
	}
	var t Token
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("oauth: parse token: %w", err)
	}
	return &t, nil
}

// Save writes the token atomically with mode 0600.
func (s *FileStore) Save(t *Token) error {
	if t == nil {
		return errors.New("oauth: nil token")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("oauth: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("oauth: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("oauth: write: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("oauth: rename: %w", err)
	}
	return nil
}

// Delete removes the token file (no error if absent).
func (s *FileStore) Delete() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("oauth: delete: %w", err)
	}
	return nil
}
