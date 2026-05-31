package handler

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"

	"github.com/Rai-Tsumugu/English-Learning/internal/irt"
	mw "github.com/Rai-Tsumugu/English-Learning/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// PlacementItem は出題候補（DB / in-memory 共通の表現）。
type PlacementItem struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
	Answer  int      `json:"-"`
	A       float64  `json:"-"`
	B       float64  `json:"-"`
}

// PlacementItemProvider はアイテムプール取得・正解判定を抽象化する。
type PlacementItemProvider interface {
	List(ctx context.Context) ([]PlacementItem, error)
	Get(ctx context.Context, id string) (*PlacementItem, error)
}

// dbItemProvider は placement_items テーブルから読む実装。
type dbItemProvider struct{ db *sql.DB }

func (p *dbItemProvider) List(ctx context.Context) ([]PlacementItem, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id, prompt, choices_json, answer_index, irt_a, irt_b FROM placement_items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlacementItem
	for rows.Next() {
		var id int64
		var prompt, choicesJSON string
		var ans int
		var a, b float64
		if err := rows.Scan(&id, &prompt, &choicesJSON, &ans, &a, &b); err != nil {
			return nil, err
		}
		var choices []string
		if err := json.Unmarshal([]byte(choicesJSON), &choices); err != nil {
			return nil, err
		}
		out = append(out, PlacementItem{
			ID: strconv.FormatInt(id, 10), Prompt: prompt, Choices: choices,
			Answer: ans, A: a, B: b,
		})
	}
	return out, rows.Err()
}

func (p *dbItemProvider) Get(ctx context.Context, id string) (*PlacementItem, error) {
	nid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	row := p.db.QueryRowContext(ctx,
		`SELECT id, prompt, choices_json, answer_index, irt_a, irt_b FROM placement_items WHERE id = ?`, nid)
	var pid int64
	var prompt, choicesJSON string
	var ans int
	var a, b float64
	if err := row.Scan(&pid, &prompt, &choicesJSON, &ans, &a, &b); err != nil {
		return nil, err
	}
	var choices []string
	if err := json.Unmarshal([]byte(choicesJSON), &choices); err != nil {
		return nil, err
	}
	return &PlacementItem{
		ID: strconv.FormatInt(pid, 10), Prompt: prompt, Choices: choices,
		Answer: ans, A: a, B: b,
	}, nil
}

// placementSession は in-memory のセッション状態。
type placementSession struct {
	UserID int64
	Theta  float64
	Asked  []string
	Resps  []irt.Response
}

// PlacementHandler は /api/placement/* を提供する。
type PlacementHandler struct {
	provider PlacementItemProvider

	mu       sync.Mutex
	sessions map[string]*placementSession
}

// NewPlacementHandler は DB から item を読む既定実装で構築する。
func NewPlacementHandler(db *sql.DB) *PlacementHandler {
	return NewPlacementHandlerWithProvider(&dbItemProvider{db: db})
}

// NewPlacementHandlerWithProvider はテスト用に provider を差し替え可能なコンストラクタ。
func NewPlacementHandlerWithProvider(p PlacementItemProvider) *PlacementHandler {
	return &PlacementHandler{
		provider: p,
		sessions: make(map[string]*placementSession),
	}
}

const (
	semStopThreshold = 0.3
	maxItems         = 20
)

// Mount は chi.Router に /placement 配下のルートを登録する。
func (h *PlacementHandler) Mount(r chi.Router) {
	r.Post("/placement/start", h.handleStart)
	r.Post("/placement/answer", h.handleAnswer)
}

type startReq struct {
	// UserID はプレースメントを行うユーザー。オンボーディング時はユーザー未作成の
	// ため 0 (匿名) が送られる。session への記録用で、省略・0 も許容する。
	UserID int64 `json:"user_id"`
}

type itemDTO struct {
	ID      string   `json:"id"`
	Prompt  string   `json:"prompt"`
	Choices []string `json:"choices"`
}

type startResp struct {
	SessionID string   `json:"session_id"`
	Item      *itemDTO `json:"item"`
}

type answerReq struct {
	SessionID string `json:"session_id"`
	ItemID    string `json:"item_id"`
	Correct   *bool  `json:"correct,omitempty"`
	Choice    *int   `json:"choice,omitempty"`
}

type answerResp struct {
	Done  bool     `json:"done"`
	Theta float64  `json:"theta"`
	SEM   float64  `json:"sem"`
	CEFR  string   `json:"cefr,omitempty"`
	Item  *itemDTO `json:"item,omitempty"`
}

func (h *PlacementHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	var req startReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid json")
		return
	}
	pool, err := h.provider.List(r.Context())
	if err != nil {
		mw.WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	if len(pool) == 0 {
		mw.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "no placement items")
		return
	}
	first := irt.SelectNext(0.0, toIRTItems(pool), nil)
	if first == nil {
		mw.WriteProblem(w, r, http.StatusServiceUnavailable, "Service Unavailable", "no item selectable")
		return
	}
	sess := &placementSession{UserID: req.UserID, Theta: 0.0}
	sid := newSessionID()
	h.mu.Lock()
	h.sessions[sid] = sess
	sess.Asked = append(sess.Asked, first.ID)
	h.mu.Unlock()

	dto := findDTO(pool, first.ID)
	writeJSON(w, http.StatusOK, startResp{SessionID: sid, Item: dto})
}

func (h *PlacementHandler) handleAnswer(w http.ResponseWriter, r *http.Request) {
	var req answerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mw.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "invalid json")
		return
	}
	if req.SessionID == "" || req.ItemID == "" {
		mw.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "session_id and item_id required")
		return
	}
	h.mu.Lock()
	sess, ok := h.sessions[req.SessionID]
	h.mu.Unlock()
	if !ok {
		mw.WriteProblem(w, r, http.StatusNotFound, "Not Found", "session not found")
		return
	}
	item, err := h.provider.Get(r.Context(), req.ItemID)
	if err != nil || item == nil {
		mw.WriteProblem(w, r, http.StatusNotFound, "Not Found", "item not found")
		return
	}
	// correct を決定（明示 / choice から判定）
	var correct bool
	switch {
	case req.Correct != nil:
		correct = *req.Correct
	case req.Choice != nil:
		correct = *req.Choice == item.Answer
	default:
		mw.WriteProblem(w, r, http.StatusBadRequest, "Bad Request", "correct or choice required")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	sess.Resps = append(sess.Resps, irt.Response{A: item.A, B: item.B, Correct: correct})
	sess.Theta = irt.UpdateTheta(sess.Theta, sess.Resps)
	sem := irt.SEM(sess.Theta, sess.Resps)

	// 停止判定
	if sem <= semStopThreshold || len(sess.Resps) >= maxItems {
		resp := answerResp{
			Done:  true,
			Theta: sess.Theta,
			SEM:   sem,
			CEFR:  irt.ThetaToCEFR(sess.Theta),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// 次の問題を選択
	pool, err := h.provider.List(r.Context())
	if err != nil {
		mw.WriteProblem(w, r, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	next := irt.SelectNext(sess.Theta, toIRTItems(pool), sess.Asked)
	if next == nil {
		// 残弾なしで強制終了
		writeJSON(w, http.StatusOK, answerResp{
			Done: true, Theta: sess.Theta, SEM: sem, CEFR: irt.ThetaToCEFR(sess.Theta),
		})
		return
	}
	sess.Asked = append(sess.Asked, next.ID)
	dto := findDTO(pool, next.ID)
	writeJSON(w, http.StatusOK, answerResp{
		Done: false, Theta: sess.Theta, SEM: sem, Item: dto,
	})
}

func toIRTItems(pool []PlacementItem) []irt.Item {
	out := make([]irt.Item, len(pool))
	for i, p := range pool {
		out[i] = irt.Item{ID: p.ID, A: p.A, B: p.B}
	}
	return out
}

func findDTO(pool []PlacementItem, id string) *itemDTO {
	for _, p := range pool {
		if p.ID == id {
			return &itemDTO{ID: p.ID, Prompt: p.Prompt, Choices: p.Choices}
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
