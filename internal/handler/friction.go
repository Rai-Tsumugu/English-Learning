package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// FrictionHandler は POST /api/friction と GET /api/friction/recent を提供する (T26)。
type FrictionHandler struct {
	db *sql.DB
}

func NewFrictionHandler(db *sql.DB) *FrictionHandler {
	return &FrictionHandler{db: db}
}

func (h *FrictionHandler) Mount(r chi.Router) {
	r.Post("/friction", h.handlePost)
	r.Get("/friction/recent", h.handleRecent)
}

var allowedFrictionKinds = map[string]bool{
	"ui_stuck": true,
	"too_hard": true,
	"too_easy": true,
	"freetext": true,
}

type frictionReq struct {
	UserID  int64           `json:"user_id"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type frictionResp struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *FrictionHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req frictionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !allowedFrictionKinds[req.Kind] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid kind"})
		return
	}
	payload := string(req.Payload)
	if payload == "" || payload == "null" {
		payload = "{}"
	}
	now := time.Now().UTC()
	var userID sql.NullInt64
	if req.UserID > 0 {
		userID = sql.NullInt64{Int64: req.UserID, Valid: true}
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO friction_log (user_id, kind, payload_json, created_at) VALUES (?, ?, ?, ?)`,
		userID, req.Kind, payload, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "insert: " + err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusCreated, frictionResp{ID: id, CreatedAt: now})
}

type frictionRecord struct {
	ID        int64           `json:"id"`
	UserID    *int64          `json:"user_id,omitempty"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

func (h *FrictionHandler) handleRecent(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, user_id, kind, payload_json, created_at FROM friction_log ORDER BY id DESC LIMIT ?`,
		limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	out := make([]frictionRecord, 0, limit)
	for rows.Next() {
		var rec frictionRecord
		var uid sql.NullInt64
		var payload string
		if err := rows.Scan(&rec.ID, &uid, &rec.Kind, &payload, &rec.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if uid.Valid {
			v := uid.Int64
			rec.UserID = &v
		}
		if payload == "" {
			payload = "{}"
		}
		rec.Payload = json.RawMessage(payload)
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}
