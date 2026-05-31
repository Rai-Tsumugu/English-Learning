// Package seed loads static seed data (e.g. placement item bank) from JSON
// files into the application database.
package seed

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// PlacementItem mirrors one entry in data/seed/placement_items.json.
type PlacementItem struct {
	ID          string   `json:"id"`
	Prompt      string   `json:"prompt"`
	Choices     []string `json:"choices"`
	AnswerIndex int      `json:"answer_index"`
	CEFR        string   `json:"cefr"`
	IRTA        float64  `json:"irt_a"`
	IRTB        float64  `json:"irt_b"`
	Topic       string   `json:"topic"`
	Source      string   `json:"source"`
}

// placementFile is the on-disk JSON envelope.
type placementFile struct {
	Version string          `json:"version"`
	Items   []PlacementItem `json:"items"`
}

// Load reads the placement item bank from the given JSON file path and
// validates basic invariants (unique IDs, answer_index in range).
func Load(path string) ([]PlacementItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pf placementFile
	if err := json.Unmarshal(b, &pf); err != nil {
		return nil, fmt.Errorf("unmarshal placement json: %w", err)
	}
	seen := make(map[string]struct{}, len(pf.Items))
	for i, it := range pf.Items {
		if it.ID == "" {
			return nil, fmt.Errorf("item %d: empty id", i)
		}
		if _, dup := seen[it.ID]; dup {
			return nil, fmt.Errorf("duplicate placement id %q", it.ID)
		}
		seen[it.ID] = struct{}{}
		if len(it.Choices) == 0 {
			return nil, fmt.Errorf("item %s: no choices", it.ID)
		}
		if it.AnswerIndex < 0 || it.AnswerIndex >= len(it.Choices) {
			return nil, fmt.Errorf("item %s: answer_index %d out of range [0,%d)", it.ID, it.AnswerIndex, len(it.Choices))
		}
	}
	return pf.Items, nil
}

// LoadInto loads the placement item bank from path and bulk-inserts the rows
// into the placement_items table. Existing rows are left untouched; this
// function is intended for first-run seeding only.
func LoadInto(ctx context.Context, db *sql.DB, path string) error {
	items, err := Load(path)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO placement_items (prompt, choices_json, answer_index, irt_a, irt_b, cefr)
        VALUES (?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, it := range items {
		cj, err := json.Marshal(it.Choices)
		if err != nil {
			return fmt.Errorf("marshal choices for %s: %w", it.ID, err)
		}
		if _, err := stmt.ExecContext(ctx, it.Prompt, string(cj), it.AnswerIndex, it.IRTA, it.IRTB, it.CEFR); err != nil {
			return fmt.Errorf("insert %s: %w", it.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
