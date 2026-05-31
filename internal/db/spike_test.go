package db

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSpikeInMemoryCRUD verifies that modernc.org/sqlite works against an
// in-memory database for plain table CRUD. This is the T02 baseline spike.
func TestSpikeInMemoryCRUD(t *testing.T) {
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	ctx := context.Background()
	if _, err := d.ExecContext(ctx, `CREATE TABLE t(id INTEGER PRIMARY KEY, name TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := d.ExecContext(ctx, `INSERT INTO t(name) VALUES (?)`, "hello"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var name string
	if err := d.QueryRowContext(ctx, `SELECT name FROM t WHERE id=1`).Scan(&name); err != nil {
		t.Fatalf("select: %v", err)
	}
	if name != "hello" {
		t.Fatalf("got %q want hello", name)
	}
}

func TestOpenAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	// Quick sanity check: every required table should now exist.
	for _, tbl := range []string{
		"users", "words", "word_vec", "examples", "attempts",
		"generated_content", "placement_items", "friction_log",
	} {
		var n int
		if err := d.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tbl,
		).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tbl, err)
		}
		if n != 1 {
			t.Errorf("table %s missing", tbl)
		}
	}
}
