package handler

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// StatsHandler は GET /api/stats/weekly を提供する (T22)。
type StatsHandler struct {
	db *sql.DB
}

func NewStatsHandler(db *sql.DB) *StatsHandler {
	return &StatsHandler{db: db}
}

func (h *StatsHandler) Mount(r chi.Router) {
	r.Get("/stats/weekly", h.handleWeekly)
}

type weeklyResp struct {
	RemainingWords  int64   `json:"remaining_words"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
	TotalAttempts7d int64   `json:"total_attempts_7d"`
	StreakDays      int     `json:"streak_days"`
	EstCostUSD7d    float64 `json:"est_cost_usd_7d"`
}

func (h *StatsHandler) handleWeekly(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	now := time.Now().UTC()
	since := now.AddDate(0, 0, -7)

	var resp weeklyResp

	// remaining_words: words 総数 から ユニーク学習済み word_id を引いたもの
	var totalWords, learnedWords int64
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM words`).Scan(&totalWords); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := h.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT word_id) FROM attempts`).Scan(&learnedWords); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	resp.RemainingWords = totalWords - learnedWords
	if resp.RemainingWords < 0 {
		resp.RemainingWords = 0
	}

	// total_attempts_7d
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM attempts WHERE created_at >= ?`, since).Scan(&resp.TotalAttempts7d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// cache_hit_rate: sum(hit_count) / (sum(hit_count) + count(rows))
	var hitSum, rowCount sql.NullInt64
	if err := h.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(hit_count), 0), COUNT(*) FROM generated_content`).Scan(&hitSum, &rowCount); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total := hitSum.Int64 + rowCount.Int64
	if total > 0 {
		resp.CacheHitRate = float64(hitSum.Int64) / float64(total)
	}

	// streak_days: 過去7日で今日から遡って連続して attempts がある日数
	streak := 0
	for i := 0; i < 7; i++ {
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var c int64
		if err := h.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM attempts WHERE created_at >= ? AND created_at < ?`,
			dayStart, dayEnd).Scan(&c); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if c == 0 {
			break
		}
		streak++
	}
	resp.StreakDays = streak

	// est_cost_usd_7d: コスト記録列がないため 0.0
	resp.EstCostUSD7d = 0.0

	writeJSON(w, http.StatusOK, resp)
}
