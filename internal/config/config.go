package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ModelGenerator string
	ModelReviewer  string
	ModelEmbedding string
	OAuthTokenPath string
	AppDBPath      string
	VocabDBPath    string
	HTTPAddr       string
	AllowedOrigin  string
	LogLevel       string
}

func Load() (*Config, error) {
	_ = loadDotEnv(".env")
	c := &Config{
		ModelGenerator: getenvFallback("MODEL_GENERATOR", "OPENAI_MODEL_GENERATOR", "gpt-4o-mini"),
		ModelReviewer:  getenvFallback("MODEL_REVIEWER", "OPENAI_MODEL_REVIEWER", "gpt-4o"),
		ModelEmbedding: getenvFallback("MODEL_EMBEDDING", "OPENAI_MODEL_EMBEDDING", "text-embedding-3-small"),
		OAuthTokenPath: getenv("OAUTH_TOKEN_PATH", defaultOAuthTokenPath()),
		AppDBPath:      getenv("APP_DB_PATH", "./data/app.db"),
		VocabDBPath:    getenv("VOCAB_DB_PATH", "./data/vocab.db"),
		HTTPAddr:       getenv("HTTP_ADDR", "127.0.0.1:8080"),
		AllowedOrigin:  getenv("ALLOWED_ORIGIN", "http://127.0.0.1:5173"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
	}
	return c, nil
}

// defaultOAuthTokenPath returns the XDG-respecting default path to
// ~/.config/english-learning/auth.json (or $XDG_CONFIG_HOME/english-learning/auth.json).
func defaultOAuthTokenPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "english-learning", "auth.json")
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
	return sc.Err()
}

func getenv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

// getenvFallback returns the first non-empty of (primary, fallback) env, or def.
func getenvFallback(primary, fallback, def string) string {
	if v, ok := os.LookupEnv(primary); ok && v != "" {
		return v
	}
	if v, ok := os.LookupEnv(fallback); ok && v != "" {
		return v
	}
	return def
}

func (c *Config) String() string {
	return fmt.Sprintf("addr=%s app_db=%s vocab_db=%s", c.HTTPAddr, c.AppDBPath, c.VocabDBPath)
}
