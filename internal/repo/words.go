package repo

import (
	"context"
	"database/sql"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/db/vec"
)

// Word mirrors a words row.
type Word struct {
	ID        int64
	Lemma     string
	CEFR      sql.NullString
	FreqRank  sql.NullInt64
	POS       sql.NullString
	GlossJA   sql.NullString
	CreatedAt time.Time
}

// Words is the words repository.
type Words struct{ db *sql.DB }

// NewWords constructs a Words repository.
func NewWords(db *sql.DB) *Words { return &Words{db: db} }

// Create inserts a new word and returns its ID. Mainly used by tests / ingest.
func (r *Words) Create(ctx context.Context, lemma, cefr, pos, glossJA string, freqRank int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO words(lemma, cefr, pos, gloss_ja, freq_rank) VALUES (?,?,?,?,?)`,
		lemma, nullableString(cefr), nullableString(pos), nullableString(glossJA),
		nullableInt64(freqRank))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetByID fetches a word by id.
func (r *Words) GetByID(ctx context.Context, id int64) (*Word, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, lemma, cefr, freq_rank, pos, gloss_ja, created_at FROM words WHERE id=?`, id)
	return scanWord(row)
}

// GetByLemma fetches a word by its unique lemma.
func (r *Words) GetByLemma(ctx context.Context, lemma string) (*Word, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, lemma, cefr, freq_rank, pos, gloss_ja, created_at FROM words WHERE lemma=?`, lemma)
	return scanWord(row)
}

// ListByCEFR lists words filtered by CEFR level, ordered by freq_rank then id.
func (r *Words) ListByCEFR(ctx context.Context, cefr string, limit int) ([]Word, error) {
	q := `SELECT id, lemma, cefr, freq_rank, pos, gloss_ja, created_at
		FROM words WHERE cefr=? ORDER BY freq_rank ASC NULLS LAST, id ASC`
	args := []any{cefr}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		w, err := scanWordRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// UpsertVec writes the embedding BLOB for a word, replacing any existing row.
func (r *Words) UpsertVec(ctx context.Context, wordID int64, embedding []float32, model string) error {
	blob := vec.Encode(embedding)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO word_vec(word_id, embedding, model) VALUES (?, ?, ?)
		ON CONFLICT(word_id) DO UPDATE SET embedding=excluded.embedding, model=excluded.model`,
		wordID, blob, model)
	return err
}

// GetVec returns the embedding vector for `wordID`, or sql.ErrNoRows if absent.
func (r *Words) GetVec(ctx context.Context, wordID int64) ([]float32, error) {
	row := r.db.QueryRowContext(ctx, `SELECT embedding FROM word_vec WHERE word_id=?`, wordID)
	var blob []byte
	if err := row.Scan(&blob); err != nil {
		return nil, err
	}
	return vec.Decode(blob)
}

// NeighborsCosine returns the top-K nearest words (by cosine on word_vec) to
// `query`. Excludes the seed wordID itself if it appears.
func (r *Words) NeighborsCosine(ctx context.Context, query []float32, k int, excludeID int64) ([]vec.Hit, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT word_id, embedding FROM word_vec`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[int64][]float32)
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		if id == excludeID {
			continue
		}
		v, err := vec.Decode(blob)
		if err != nil {
			return nil, err
		}
		items[id] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return vec.TopK(query, items, k), nil
}

func scanWord(row *sql.Row) (*Word, error) {
	w := &Word{}
	if err := row.Scan(&w.ID, &w.Lemma, &w.CEFR, &w.FreqRank, &w.POS, &w.GlossJA, &w.CreatedAt); err != nil {
		return nil, err
	}
	return w, nil
}

func scanWordRows(rows *sql.Rows) (*Word, error) {
	w := &Word{}
	if err := rows.Scan(&w.ID, &w.Lemma, &w.CEFR, &w.FreqRank, &w.POS, &w.GlossJA, &w.CreatedAt); err != nil {
		return nil, err
	}
	return w, nil
}

func nullableInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
