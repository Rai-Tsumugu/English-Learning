package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Rai-Tsumugu/English-Learning/internal/repo"
	"github.com/go-chi/chi/v5"
)

// OnboardingHandler は /api/onboarding を提供する (T12)。
type OnboardingHandler struct {
	users *repo.Users
}

func NewOnboardingHandler(db *sql.DB) *OnboardingHandler {
	return &OnboardingHandler{users: repo.NewUsers(db)}
}

func (h *OnboardingHandler) Mount(r chi.Router) {
	r.Post("/onboarding", h.handlePost)
}

type onboardingReq struct {
	CEFRSelf string  `json:"cefr_self"`
	Theta    float64 `json:"theta"`
	SEM      float64 `json:"sem"`
}

type onboardingResp struct {
	UserID   int64   `json:"user_id"`
	CEFRSelf string  `json:"cefr_self"`
	Theta    float64 `json:"theta"`
	SEM      float64 `json:"sem"`
}

var validCEFR = map[string]bool{
	"A1": true, "A2": true, "B1": true, "B2": true, "C1": true, "C2": true,
}

func (h *OnboardingHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	var req onboardingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.CEFRSelf = strings.ToUpper(strings.TrimSpace(req.CEFRSelf))
	if !validCEFR[req.CEFRSelf] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cefr_self must be one of A1..C2"})
		return
	}
	// Theta が未提供(0) かつ SEM が 0 の場合、CEFR 自己申告から theta 初期値を推定する。
	if req.SEM == 0 {
		req.SEM = 1.0
		if req.Theta == 0 {
			req.Theta = cefrToTheta(req.CEFRSelf)
		}
	}
	id, err := h.users.Create(r.Context(), req.CEFRSelf, req.Theta, req.SEM)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create user: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, onboardingResp{
		UserID: id, CEFRSelf: req.CEFRSelf, Theta: req.Theta, SEM: req.SEM,
	})
}

func cefrToTheta(c string) float64 {
	switch c {
	case "A1":
		return -2.0
	case "A2":
		return -1.0
	case "B1":
		return 0.0
	case "B2":
		return 1.0
	case "C1":
		return 2.0
	case "C2":
		return 2.5
	}
	return 0.0
}
