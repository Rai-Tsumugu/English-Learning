// Package db opens the application SQLite database, applies embedded goose
// migrations, and exposes a few small helpers used by repo and cmd packages.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	// modernc.org/sqlite registers the "sqlite" driver.
	_ "modernc.org/sqlite"
)

// DriverName is the database/sql driver name registered by modernc.org/sqlite.
const DriverName = "sqlite"

// Open opens a SQLite database at the given path (use ":memory:" for tests),
// enables WAL + sensible pragmas, applies all embedded migrations, and
// returns the ready-to-use *sql.DB.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	sqlDB, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if path != ":memory:" {
		if _, err := sqlDB.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("enable WAL: %w", err)
		}
	}
	if _, err := sqlDB.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}
	if err := Migrate(sqlDB, "up"); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

// Migrate runs goose against the embedded migrations FS.
// action is one of: "up", "down", "status", "reset".
func Migrate(sqlDB *sql.DB, action string) error {
	goose.SetBaseFS(MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	switch action {
	case "", "up":
		return goose.Up(sqlDB, "migrations")
	case "down":
		return goose.Down(sqlDB, "migrations")
	case "status":
		return goose.Status(sqlDB, "migrations")
	case "reset":
		return goose.Reset(sqlDB, "migrations")
	default:
		return fmt.Errorf("unknown migrate action %q", action)
	}
}
