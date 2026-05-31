package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/Rai-Tsumugu/English-Learning/internal/srs"
	"github.com/go-chi/chi/v5"
)

// AttemptsHandler は POST /api/attempts を提供する (T20)。
type AttemptsHandler struct {
	attempts *repo.Attempts
}

func NewAttemptsHandler(db *sql.DB) *AttemptsHandler {
	return &AttemptsHandler{attempts: repo.NewAttempts(db)}
}

func (h *AttemptsHandler) Mount(r chi.Router) {
	r.Post("/attempts", h.handlePost)
}

type attemptReq struct {
	UserID      int64  `json:"user_id"`
	WordID      int64  `json:"word_id"`
	ContentHash string `json:"content_hash"`
	Correct     bool   `json:"correct"`
	LatencyMS   int64  `json:"latency_ms"`
}

type attemptResp struct {
	AttemptID    int64     `json:"attempt_id"`
	Quality      int       `json:"quality"`
	NextReviewAt time.Time `json:"next_review_at"`
	Ease         float64   `json:"ease"`
	IntervalDays int       `json:"interval_days"`
	Reps         int       `json:"reps"`
}

func (h *AttemptsHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req attemptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.UserID == 0 || req.WordID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id and word_id required"})
		return
	}

	prev, err := h.attempts.LatestForWord(r.Context(), req.UserID, req.WordID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "load state: " + err.Error()})
		return
	}
	card := srs.Card{}
	if prev != nil {
		if prev.Ease.Valid {
			card.Ease = prev.Ease.Float64
		}
		if prev.IntervalDays.Valid {
			card.IntervalDays = int(prev.IntervalDays.Float64)
		}
		if prev.Reps.Valid {
			card.Reps = int(prev.Reps.Int64)
		}
		if prev.Lapses.Valid {
			card.Lapses = int(prev.Lapses.Int64)
		}
	}

	q := srs.QualityFromLatency(req.Correct, int(req.LatencyMS))
	now := time.Now().UTC()
	next := srs.Next(card, srs.Review{Quality: q, ReviewedAt: now})
	nextAt := srs.NextReviewAt(next)

	a := &repo.Attempt{
		UserID:       req.UserID,
		WordID:       req.WordID,
		ContentHash:  sql.NullString{String: req.ContentHash, Valid: req.ContentHash != ""},
		Correct:      req.Correct,
		LatencyMS:    req.LatencyMS,
		Quality:      sql.NullInt64{Int64: int64(q), Valid: true},
		Ease:         sql.NullFloat64{Float64: next.Ease, Valid: true},
		IntervalDays: sql.NullFloat64{Float64: float64(next.IntervalDays), Valid: true},
		Reps:         sql.NullInt64{Int64: int64(next.Reps), Valid: true},
		Lapses:       sql.NullInt64{Int64: int64(next.Lapses), Valid: true},
		NextReviewAt: sql.NullTime{Time: nextAt, Valid: !nextAt.IsZero()},
	}
	id, err := h.attempts.Insert(r.Context(), a)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "insert: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, attemptResp{
		AttemptID:    id,
		Quality:      q,
		NextReviewAt: nextAt,
		Ease:         next.Ease,
		IntervalDays: next.IntervalDays,
		Reps:         next.Reps,
	})
}
