package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"

	_ "modernc.org/sqlite"
)

// runMigrate dispatches `app migrate <action>` against the embedded goose
// migrations. action defaults to "up".
func runMigrate(ctx context.Context, cfg *config.Config, args []string) error {
	action := "up"
	if len(args) > 0 {
		action = args[0]
	}
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
	if err := appdb.Migrate(d, action); err != nil {
		return fmt.Errorf("migrate %s: %w", action, err)
	}
	fmt.Printf("migrate %s on %s: ok\n", action, cfg.AppDBPath)
	return nil
}
