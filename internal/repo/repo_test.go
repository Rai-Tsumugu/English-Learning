package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := appdb.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUsersRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	users := NewUsers(d)

	id, err := users.Create(ctx, "A2", 0.0, 1.0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == 0 {
		t.Fatalf("want non-zero id")
	}
	u, err := users.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if u.CEFRSelf.String != "A2" {
		t.Errorf("cefr_self got %q want A2", u.CEFRSelf.String)
	}
	if err := users.UpdateTheta(ctx, id, 1.25, 0.4); err != nil {
		t.Fatalf("update: %v", err)
	}
	u2, err := users.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if u2.Theta != 1.25 || u2.SEM != 0.4 {
		t.Errorf("theta/sem mismatch: %+v", u2)
	}
}

func TestAttemptsRoundTrip(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	users := NewUsers(d)
	words := NewWords(d)
	attempts := NewAttempts(d)

	uid, err := users.Create(ctx, "B1", 0, 1)
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	wid, err := words.Create(ctx, "apple", "A1", "noun", "りんご", 100)
	if err != nil {
		t.Fatalf("word: %v", err)
	}

	due := time.Now().Add(-1 * time.Hour)
	_, err = attempts.Insert(ctx, &Attempt{
		UserID:       uid,
		WordID:       wid,
		Correct:      true,
		LatencyMS:    1234,
		NextReviewAt: sql.NullTime{Time: due, Valid: true},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	list, err := attempts.ListByUser(ctx, uid, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || !list[0].Correct || list[0].LatencyMS != 1234 {
		t.Fatalf("unexpected list: %+v", list)
	}

	dueList, err := attempts.ListDue(ctx, uid, time.Now(), 0)
	if err != nil {
		t.Fatalf("listdue: %v", err)
	}
	if len(dueList) != 1 {
		t.Fatalf("want 1 due, got %d", len(dueList))
	}
}

func TestWordsAndNeighbors(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	words := NewWords(d)

	w1, _ := words.Create(ctx, "cat", "A1", "noun", "猫", 10)
	w2, _ := words.Create(ctx, "kitten", "A2", "noun", "子猫", 20)
	w3, _ := words.Create(ctx, "rocket", "B2", "noun", "ロケット", 5000)

	if got, _ := words.GetByLemma(ctx, "cat"); got == nil || got.ID != w1 {
		t.Fatalf("GetByLemma cat: %+v", got)
	}
	if got, _ := words.GetByID(ctx, w2); got == nil || got.Lemma != "kitten" {
		t.Fatalf("GetByID kitten: %+v", got)
	}

	if err := words.UpsertVec(ctx, w1, []float32{1, 0, 0, 0, 0}, "test"); err != nil {
		t.Fatalf("upsert w1: %v", err)
	}
	if err := words.UpsertVec(ctx, w2, []float32{0.95, 0.1, 0, 0, 0}, "test"); err != nil {
		t.Fatalf("upsert w2: %v", err)
	}
	if err := words.UpsertVec(ctx, w3, []float32{0, 0, 0, 1, 0}, "test"); err != nil {
		t.Fatalf("upsert w3: %v", err)
	}

	hits, err := words.NeighborsCosine(ctx, []float32{1, 0, 0, 0, 0}, 2, w1)
	if err != nil {
		t.Fatalf("neighbors: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits got %d", len(hits))
	}
	if hits[0].ID != w2 {
		t.Errorf("top neighbor want %d (kitten) got %d", w2, hits[0].ID)
	}
}

func TestContentCache(t *testing.T) {
	ctx := context.Background()
	d := newTestDB(t)
	c := NewContent(d)
	g := &GeneratedContent{
		CacheKey:    "k1",
		Model:       "gpt-4o-mini",
		SchemaVer:   "1",
		PromptVer:   "1",
		PayloadJSON: `{"x":1}`,
	}
	if err := c.Put(ctx, g); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PayloadJSON != `{"x":1}` {
		t.Errorf("payload mismatch: %s", got.PayloadJSON)
	}
	if err := c.IncHit(ctx, "k1"); err != nil {
		t.Fatalf("inc: %v", err)
	}
	got2, _ := c.Get(ctx, "k1")
	if got2.HitCount != 1 {
		t.Errorf("hit_count want 1 got %d", got2.HitCount)
	}
}
