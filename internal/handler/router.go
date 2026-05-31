package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	mw "github.com/Rai-Tsumugu/English-Learning/internal/middleware"
)

// Version はビルド時に上書き可能なアプリバージョン文字列。
var Version = "dev"

// NewRouter は chi ベースの http.Handler を生成する。
// CORS allowlist、構造化ログ、panic recoverer、/healthz, /api/version を組み込む。
func NewRouter(cfg *config.Config) http.Handler {
	return NewRouterWithDB(cfg, nil)
}

// NewRouterWithDB は DB ハンドルを受け取り placement 等の DB 依存ルートを Mount する。
func NewRouterWithDB(cfg *config.Config, db *sql.DB) http.Handler {
	return NewRouterFull(cfg, db, nil)
}

// SessionsMounter は /api/sessions 系を Mount するためのインターフェース。
// 循環依存と nil チェックを避けるため、SessionsHandler 自身を渡せるよう抽象化している。
type SessionsMounter interface {
	Mount(r chi.Router)
}

// NewRouterFull は DB と SessionsMounter を受け取って全ルートを mount する。
func NewRouterFull(cfg *config.Config, db *sql.DB, sessions SessionsMounter) http.Handler {
	r := chi.NewRouter()

	r.Use(mw.Logger)
	r.Use(mw.Recoverer)

	allowedOrigins := splitAndTrim(cfg.AllowedOrigin)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// /api 配下に将来のハンドラを束ねるための Mount ポイント。
	r.Route("/api", func(api chi.Router) {
		api.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]string{"version": Version})
		})
		// T11: placement (IRT 適応出題) — DB 設定がある場合のみ Mount。
		if db != nil {
			NewPlacementHandler(db).Mount(api)
			// T12: onboarding (CEFR 自己申告 + プレースメント結果保存)
			NewOnboardingHandler(db).Mount(api)
			// T20: POST /api/attempts (SRS 採点)
			NewAttemptsHandler(db).Mount(api)
			// T21: GET /api/words/{id}, /api/words/{id}/neighbors
			NewWordsHandler(db).Mount(api)
			// T22: GET /api/stats/weekly
			NewStatsHandler(db).Mount(api)
			// T26: POST /api/friction, GET /api/friction/recent
			NewFrictionHandler(db).Mount(api)
			// T19: GET /api/sessions/today (SSE) — 設定された場合のみ Mount
			if sessions != nil {
				sessions.Mount(api)
			}
		}
	})

	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		mw.WriteProblem(w, req, http.StatusNotFound, "Not Found", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		mw.WriteProblem(w, req, http.StatusMethodNotAllowed, "Method Not Allowed", "method not allowed")
	})

	return r
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = append(out, "http://127.0.0.1:5173")
	}
	return out
}
