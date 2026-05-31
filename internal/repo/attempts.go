package repo

import (
	"context"
	"database/sql"
	"time"
)

// Attempt represents a single review/answer record.
type Attempt struct {
	ID            int64
	UserID        int64
	WordID        int64
	ContentHash   sql.NullString
	Correct       bool
	LatencyMS     int64
	Quality       sql.NullInt64
	Ease          sql.NullFloat64
	IntervalDays  sql.NullFloat64
	Reps          sql.NullInt64
	Lapses        sql.NullInt64
	NextReviewAt  sql.NullTime
	FSRSStability sql.NullFloat64
	FSRSDiff      sql.NullFloat64
	FSRSLast      sql.NullTime
	CreatedAt     time.Time
}

// Attempts is the attempts repository.
type Attempts struct{ db *sql.DB }

// NewAttempts constructs an Attempts repository.
func NewAttempts(db *sql.DB) *Attempts { return &Attempts{db: db} }

// Insert writes a new attempt and returns its ID.
func (r *Attempts) Insert(ctx context.Context, a *Attempt) (int64, error) {
	correct := 0
	if a.Correct {
		correct = 1
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO attempts(
		user_id, word_id, content_hash, correct, latency_ms,
		quality, ease, interval_days, reps, lapses, next_review_at,
		fsrs_stability, fsrs_difficulty, fsrs_last_review
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.UserID, a.WordID, a.ContentHash, correct, a.LatencyMS,
		a.Quality, a.Ease, a.IntervalDays, a.Reps, a.Lapses, a.NextReviewAt,
		a.FSRSStability, a.FSRSDiff, a.FSRSLast)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	a.ID = id
	return id, nil
}

// ListDue returns attempts for `userID` whose `next_review_at` is on or
// before `now`, ordered by due time. limit <= 0 means no LIMIT clause.
func (r *Attempts) ListDue(ctx context.Context, userID int64, now time.Time, limit int) ([]Attempt, error) {
	q := `SELECT id, user_id, word_id, content_hash, correct, latency_ms,
		quality, ease, interval_days, reps, lapses, next_review_at,
		fsrs_stability, fsrs_difficulty, fsrs_last_review, created_at
		FROM attempts WHERE user_id=? AND next_review_at IS NOT NULL AND next_review_at <= ?
		ORDER BY next_review_at ASC`
	args := []any{userID, now}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	return r.queryAttempts(ctx, q, args...)
}

// LatestForWord returns the most recent attempt for (userID, wordID), or nil
// if none exists.
func (r *Attempts) LatestForWord(ctx context.Context, userID, wordID int64) (*Attempt, error) {
	q := `SELECT id, user_id, word_id, content_hash, correct, latency_ms,
		quality, ease, interval_days, reps, lapses, next_review_at,
		fsrs_stability, fsrs_difficulty, fsrs_last_review, created_at
		FROM attempts WHERE user_id=? AND word_id=? ORDER BY created_at DESC, id DESC LIMIT 1`
	out, err := r.queryAttempts(ctx, q, userID, wordID)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// ListByUser returns the most recent attempts for the given user.
// limit <= 0 means no LIMIT clause.
func (r *Attempts) ListByUser(ctx context.Context, userID int64, limit int) ([]Attempt, error) {
	q := `SELECT id, user_id, word_id, content_hash, correct, latency_ms,
		quality, ease, interval_days, reps, lapses, next_review_at,
		fsrs_stability, fsrs_difficulty, fsrs_last_review, created_at
		FROM attempts WHERE user_id=? ORDER BY created_at DESC, id DESC`
	args := []any{userID}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	return r.queryAttempts(ctx, q, args...)
}

func (r *Attempts) queryAttempts(ctx context.Context, q string, args ...any) ([]Attempt, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var a Attempt
		var correct int64
		if err := rows.Scan(&a.ID, &a.UserID, &a.WordID, &a.ContentHash, &correct, &a.LatencyMS,
			&a.Quality, &a.Ease, &a.IntervalDays, &a.Reps, &a.Lapses, &a.NextReviewAt,
			&a.FSRSStability, &a.FSRSDiff, &a.FSRSLast, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Correct = correct != 0
		out = append(out, a)
	}
	return out, rows.Err()
}
