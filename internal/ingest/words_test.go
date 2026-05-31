package ingest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := appdb.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestLoadFile_Sample(t *testing.T) {
	f, err := LoadFile("../../data/seed/words.sample.json")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(f.Items) == 0 {
		t.Fatal("no items")
	}
	for _, it := range f.Items {
		if it.Lemma == "" || it.CEFR == "" {
			t.Errorf("bad item: %+v", it)
		}
	}
}

func TestIngest_UpsertSemantics(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()

	ins, upd, _, err := Ingest(ctx, d, Options{Path: "../../data/seed/words.sample.json"})
	if err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	if ins == 0 {
		t.Errorf("expected new inserts, got ins=%d upd=%d", ins, upd)
	}

	// 2回目は全て update 扱いになるはず。
	ins2, upd2, _, err := Ingest(ctx, d, Options{Path: "../../data/seed/words.sample.json"})
	if err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	if ins2 != 0 || upd2 != ins {
		t.Errorf("re-ingest counts: ins=%d upd=%d want ins=0 upd=%d", ins2, upd2, ins)
	}
}
