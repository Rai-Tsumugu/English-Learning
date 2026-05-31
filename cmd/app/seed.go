package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/Rai-Tsumugu/English-Learning/internal/seed"
)

// ensurePlacementSeed は placement_items が空のときだけ data/seed/placement_items.json
// から取り込む。本番ではマイグレーションと分離して冪等にしておきたいため。
func ensurePlacementSeed(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_items`).Scan(&n); err != nil {
		return fmt.Errorf("count: %w", err)
	}
	if n > 0 {
		return nil
	}
	path := "data/seed/placement_items.json"
	if _, err := os.Stat(path); err != nil {
		// シードファイルが見つからなくても起動は継続（テスト/CI 等）。
		fmt.Printf("seed: %s not found, skipping placement seed\n", path)
		return nil
	}
	if err := seed.LoadInto(ctx, db, path); err != nil {
		return err
	}
	fmt.Printf("seed: placement_items loaded from %s\n", path)
	return nil
}
