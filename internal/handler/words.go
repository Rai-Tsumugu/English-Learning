package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"

	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/go-chi/chi/v5"
)

// WordsHandler は /api/words/* を提供する (T21)。
type WordsHandler struct {
	words *repo.Words
}

func NewWordsHandler(db *sql.DB) *WordsHandler {
	return &WordsHandler{words: repo.NewWords(db)}
}

func (h *WordsHandler) Mount(r chi.Router) {
	r.Get("/words/{id}", h.handleGet)
	r.Get("/words/{id}/neighbors", h.handleNeighbors)
}

type wordDTO struct {
	ID       int64  `json:"id"`
	Lemma    string `json:"lemma"`
	CEFR     string `json:"cefr,omitempty"`
	FreqRank int64  `json:"freq_rank,omitempty"`
	POS      string `json:"pos,omitempty"`
	GlossJA  string `json:"gloss_ja,omitempty"`
}

func toWordDTO(w *repo.Word) wordDTO {
	d := wordDTO{ID: w.ID, Lemma: w.Lemma}
	if w.CEFR.Valid {
		d.CEFR = w.CEFR.String
	}
	if w.FreqRank.Valid {
		d.FreqRank = w.FreqRank.Int64
	}
	if w.POS.Valid {
		d.POS = w.POS.String
	}
	if w.GlossJA.Valid {
		d.GlossJA = w.GlossJA.String
	}
	return d
}

func parseIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func (h *WordsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	word, err := h.words.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "word not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toWordDTO(word))
}

type neighborHit struct {
	ID    int64   `json:"id"`
	Lemma string  `json:"lemma"`
	Score float32 `json:"score"`
}

type neighborsResp struct {
	WordID    int64         `json:"word_id"`
	Neighbors []neighborHit `json:"neighbors"`
}

// handleNeighbors は OAuth サブスク化に伴い embedding が利用できないため、
// 同 CEFR レベルから 3-gram Jaccard 類似度上位 k 件を返す lexical fallback。
// 将来ローカル sentence-transformer 等で再導入する場合は、ここを差し替える。
func (h *WordsHandler) handleNeighbors(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	k := 5
	if s := r.URL.Query().Get("k"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			k = v
		}
	}
	target, err := h.words.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "word not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	cefr := target.CEFR.String
	if cefr == "" {
		cefr = "A2"
	}
	pool, err := h.words.ListByCEFR(r.Context(), cefr, 500)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	targetGrams := trigrams(target.Lemma)
	type scored struct {
		id    int64
		lemma string
		score float32
	}
	scoredList := make([]scored, 0, len(pool))
	for _, p := range pool {
		if p.ID == id {
			continue
		}
		s := jaccard(targetGrams, trigrams(p.Lemma))
		if s <= 0 {
			continue
		}
		scoredList = append(scoredList, scored{id: p.ID, lemma: p.Lemma, score: float32(s)})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	if len(scoredList) > k {
		scoredList = scoredList[:k]
	}
	out := make([]neighborHit, 0, len(scoredList))
	for _, s := range scoredList {
		out = append(out, neighborHit{ID: s.id, Lemma: s.lemma, Score: s.score})
	}
	writeJSON(w, http.StatusOK, neighborsResp{WordID: id, Neighbors: out})
}

func trigrams(s string) map[string]struct{} {
	if len(s) < 3 {
		return map[string]struct{}{s: {}}
	}
	out := make(map[string]struct{}, len(s)-2)
	for i := 0; i <= len(s)-3; i++ {
		out[s[i:i+3]] = struct{}{}
	}
	return out
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for g := range a {
		if _, ok := b[g]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
