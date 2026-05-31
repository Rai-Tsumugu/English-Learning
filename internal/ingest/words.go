// Package ingest は語彙データを app.db / vocab.db に取り込む CLI ロジックを提供する。
// Phase1 ではサンプル JSON のみサポートし、Phase 後半で NGSL/CEFR-J/Octanove
// の実データに対応する想定。
//
// ChatGPT OAuth サブスクへの移行に伴い、embedding 取得経路は廃止された。
// Embedding 列はテーブルから残しているが ingest 経路では書き込まない。
package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

type WordItem struct {
	Lemma    string `json:"lemma"`
	CEFR     string `json:"cefr"`
	FreqRank int    `json:"freq_rank"`
	POS      string `json:"pos"`
	GlossJA  string `json:"gloss_ja"`
}

type WordsFile struct {
	Version string     `json:"version"`
	Source  string     `json:"source"`
	Note    string     `json:"note,omitempty"`
	Items   []WordItem `json:"items"`
}

// LoadFile はファイルパスから WordsFile を読み込む。
func LoadFile(path string) (*WordsFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var f WordsFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &f, nil
}

// Options は ingest 実行のオプション。
// Phase1 (OAuth 化後) では Path のみ有効。Embedding 関連フィールドは
// 互換のため残してあるが無視される。
type Options struct {
	Path string
}

// Ingest は words テーブルに upsert する。embedding は計算しない (戻り値の
// embedded は常に 0)。
func Ingest(ctx context.Context, db *sql.DB, opts Options) (inserted, updated, embedded int, err error) {
	f, err := LoadFile(opts.Path)
	if err != nil {
		return 0, 0, 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	lookup, err := tx.PrepareContext(ctx, `SELECT id FROM words WHERE lemma = ?`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare lookup: %w", err)
	}
	defer lookup.Close()

	upsert, err := tx.PrepareContext(ctx, `
		INSERT INTO words (lemma, cefr, freq_rank, pos, gloss_ja)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(lemma) DO UPDATE SET
			cefr = excluded.cefr,
			freq_rank = excluded.freq_rank,
			pos = excluded.pos,
			gloss_ja = excluded.gloss_ja
		RETURNING id
	`)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare upsert: %w", err)
	}
	defer upsert.Close()

	for _, it := range f.Items {
		var existingID int64
		exists := true
		if err := lookup.QueryRowContext(ctx, it.Lemma).Scan(&existingID); err != nil {
			if err == sql.ErrNoRows {
				exists = false
			} else {
				return inserted, updated, embedded, fmt.Errorf("lookup %s: %w", it.Lemma, err)
			}
		}
		var id int64
		if err := upsert.QueryRowContext(ctx, it.Lemma, it.CEFR, it.FreqRank, it.POS, it.GlossJA).Scan(&id); err != nil {
			return inserted, updated, embedded, fmt.Errorf("upsert %s: %w", it.Lemma, err)
		}
		if exists {
			updated++
		} else {
			inserted++
		}
		_ = id
	}
	if err = tx.Commit(); err != nil {
		return inserted, updated, embedded, fmt.Errorf("commit: %w", err)
	}
	return inserted, updated, embedded, nil
}
