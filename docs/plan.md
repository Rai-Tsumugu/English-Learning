# English-Learning Phase1 実装計画 (v1.0)

**Author**: RaiTsumugu
**Date**: 2026-05-27
**対象**: docs/design.md v0.3 の Phase1 スコープ
**チーム**: 個人開発1名
**稼働前提**: 1人日 = 集中作業 5h（残りは検証/休憩/雑務バッファ）
**Sprint長**: 1週間 × 2 (= 14 暦日, 10 営業日)
**目標完了**: 着手日 + 14日（バッファ込み 17日）

---

## 0. Phase1 スコープ確認

design.md §1.4 より:
- ダッシュボード + 単語クイズ + ストリーク + 履歴 + 簡易ネットマップ（関連語5件リスト）
- 単一エージェント, SQLite のみ, sqlite-vec, 生成キャッシュ, Reviewer 別系統
- SRS は SM-2（FSRS-5 移行用カラムは先行追加のみ）
- 日本語サポート3モード/長文/TTS は Phase2 へ繰延

**Phase1 で実装しないもの**: 長文読解, 日本語サポート切替UI, FSRS-5本実装, リッチ可視化, TTS, 週次拡張セッション

---

## 1. アーキテクチャ

### 1.1 システム俯瞰

```
┌────────────────────────────────────────────────────────────┐
│  Browser (Vite dev: 5173 / build: Go embed)                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  React SPA (TS + Tailwind)                           │  │
│  │  pages/ Onboarding · Dashboard · Quiz · History      │  │
│  │  hooks/ useSSE · useAttempt · useStreak              │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
                │ REST/JSON + SSE (同一オリジン)
                ▼
┌────────────────────────────────────────────────────────────┐
│  Go API (chi) :8080  127.0.0.1 only                        │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐  │
│  │ handler  │ service  │ agent    │ repo     │ sse      │  │
│  │ (REST)   │ (SRS,    │ (Curr/   │ (sqlc or │ (4-evt   │  │
│  │          │  IRT,    │  Gen/    │  手書き) │  正規化) │  │
│  │          │  cache)  │  Review) │          │          │  │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ infra: openai client / cost-meter / circuit-breaker  │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
                │                                  │
                ▼                                  ▼
   ┌─────────────────────────┐         ┌──────────────────┐
   │ SQLite (modernc) WAL    │         │ OpenAI API       │
   │  - app.db               │         │  - gpt-4o-mini   │
   │  - vocab.db (+vec)      │         │  - gpt-4o (rev)  │
   │  goose migrate (embed)  │         │  - embed-3-small │
   └─────────────────────────┘         └──────────────────┘
```

### 1.2 コンポーネント一覧

| # | コンポーネント | 技術 | 責務 |
|---|---|---|---|
| C1 | Frontend SPA | React + Vite + TS + Tailwind + TanStack Query | 4画面（Onboarding/Dashboard/Quiz/History）+ SSE 受信 |
| C2 | Go API Server | chi + net/http | REST + SSE + CORS allowlist |
| C3 | Persistence | modernc.org/sqlite + sqlite-vec(WASM ext via modernc 拡張) + goose | DDL/DML, ベクタ近傍検索 |
| C4 | Agent Orchestrator | OpenAI Go SDK + invopop/jsonschema | Curriculum→Generator→Reviewer 直列 |
| C5 | SRS Engine | 自前 Go パッケージ | SM-2 計算 + lapse 下限 ease 1.3 |
| C6 | IRT Placement | 自前 Go (2PL) | 静的問題バンクからの適応出題 + SEM 判定 |
| C7 | Cost & Circuit | usage tracker + middleware | 日次トークン上限 / 縮退L1-L4 |
| C8 | SSE Layer | internal/sse | plan/question/done/error 4イベント |
| C9 | Data Ingestion CLI | `app ingest` サブコマンド | NGSL/CEFR-J/Octanove → vocab.db, embedding 一括生成 |
| C10 | Migration CLI | `app migrate up` | goose embed.FS |

### 1.3 sqlite-vec の CGO フリー実装方針（技術判断ポイント）

**前提リスク**: `modernc.org/sqlite/vec` は design.md §2 に記載されているが、実在性が不安定。sqlite-vec 公式は C 拡張（要 CGO）。

**判断分岐**（TASK-002 のスパイクで決定）:
- **Plan A**: `modernc.org/sqlite/vec` が利用可能 → そのまま使用
- **Plan B**: 利用不可 → CGO ビルドを許容する `vec0` ロードに切替（単一バイナリ堅持のため `-tags sqlite_vec` 静的リンク）
- **Plan C**: 両方不可 → Phase1 では純Go の小規模 KNN（語彙15k 程度なら全件コサイン計算で十分） + Phase2 で sqlite-vec 化

**意思決定タイミング**: Day1 終了時（4h タイムボックス）

### 1.4 データフロー（本日の学習）

```
[1] User → GET /api/sessions/today
[2] service: attempts から SRS due 抽出 + profile
[3] cache key = sha256(model|schema_ver|prompt_ver|inputs)
    HIT → generated_content から payload 返却（SSE で plan/question/done 即時）
    MISS:
[4]   CurriculumAgent: 出題プラン JSON（類義語禁止, interleaving）
[5]   GeneratorAgent : 問題JSON + answer_span + cefr_evidence
[6]   ReviewerAgent  : 10%サンプル or 新形式100%
[7]   generated_content に保存
[8]   SSE で plan → question(複数) → done
[9] User → POST /api/attempts
[10] SRS 更新 (SM-2) → next_review_at 計算
```

---

## 2. タスク分解（依存関係順）

凡例: **見積もり=人日**, P0=ブロッカー / P1=必須 / P2=後回し可

### Sprint 1 (Day 1–7): Foundation & 縦串

| ID | タスク | 種別 | 優先度 | 見積 | 依存 |
|---|---|---|---|---|---|
| T01 | プロジェクト雛形（Go module + Vite + Makefile + .env.example + .gitignore） | Infra | P0 | 0.5 | - |
| T02 | **スパイク**: modernc.org/sqlite + sqlite-vec の動作検証（KNN 1件返るとこまで） | Infra | P0 | 0.5 | T01 |
| T03 | goose embed migration + 全テーブル DDL（users/attempts/generated_content/words/word_vec/examples/placement_items/friction_log） | Infra | P0 | 0.5 | T02 |
| T04 | repository 層（attempts/words/generated_content/users）+ ゴールデン Go test | Impl | P0 | 1.0 | T03 |
| T05 | OpenAI クライアントラッパ + cost-meter + usage hard limit + ログマスキング | Infra | P0 | 0.5 | T01 |
| T06 | invopop/jsonschema による Go struct→Schema 自動生成 + Structured Outputs strict 呼び出しヘルパ | Impl | P0 | 0.5 | T05 |
| T07 | SSE パッケージ（4イベント正規化, Flusher, heartbeat 15s, ctx 解放） | Impl | P0 | 0.5 | T01 |
| T08 | chi ルータ + CORS allowlist(127.0.0.1:5173) + RFC7807 エラー + 127.0.0.1 バインド | Impl | P0 | 0.5 | T01 |
| T09 | データ取り込み CLI（`app ingest`）: NGSL + CEFR-J + Octanove → words テーブル + embedding 一括 + LICENSES 検証 | Infra | P0 | 1.5 | T03, T05 |
| T10 | 静的プレースメント問題バンク 30問の手作業作成 + IRT a/b 暫定値付与 → seed JSON | Content | P0 | 1.0 | T03 |
| T11 | IRT 2PL 適応出題ロジック + SEM ≤ 0.3 停止規準 + `/api/placement/next` ハンドラ | Impl | P0 | 1.0 | T04, T10 |
| T12 | `/api/onboarding` ハンドラ（CEFR自己申告 + プレースメント結果保存） | Impl | P0 | 0.3 | T11 |
| T13 | SRS (SM-2) 計算パッケージ + 単体テスト（lapse ease 下限 1.3 含む） | Impl | P0 | 0.5 | T04 |
| T14 | **縦串疎通**: フロント雛形 → Onboarding 1画面 → /api/onboarding 保存まで | Impl | P0 | 1.0 | T08, T12 |

**Sprint1 合計**: 9.3 人日（10営業日中 9.3 → バッファ 0.7 日）

### Sprint 2 (Day 8–14): エージェント & 残UI

| ID | タスク | 種別 | 優先度 | 見積 | 依存 |
|---|---|---|---|---|---|
| T15 | Curriculum Agent（類義語/近傍語禁止 + interleaving + 1日上限10–15語 + 未消化3日ブレーカ） | Impl | P0 | 1.0 | T06, T09 |
| T16 | Generator Agent（answer_span/cefr_evidence 必須 + 再試行2回 + max_tokens 1.5x 拡張） | Impl | P0 | 1.0 | T06 |
| T17 | Reviewer Agent（gpt-4o 別系統, 4段階ルーブリック, 10%サンプリング + 新形式100% + 自動昇格） | Impl | P1 | 0.7 | T16 |
| T18 | 生成キャッシュ + ハッシュキー + ヒット率メトリクス | Impl | P0 | 0.5 | T16 |
| T19 | `/api/sessions/today` SSE ハンドラ（CurriculumAgent→Generator→Reviewer→キャッシュ保存→SSE 返却） | Impl | P0 | 1.0 | T15, T16, T17, T18, T07 |
| T20 | `/api/attempts` POST（SRS 更新 + 苦手重み付け + latency_ms quality 推定） | Impl | P0 | 0.5 | T13 |
| T21 | `/api/words/:id` + `/api/words/:id/neighbors`（コサイン上位5件） | Impl | P1 | 0.5 | T04 |
| T22 | `/api/stats/weekly`（残語数・キャッシュ率・コスト・ストリーク） | Impl | P1 | 0.5 | T04 |
| T23 | Dashboard 画面（巨大CTA「今日の5分を始める」+ 週7マスストリーク + 残語数 + 累積分） | Impl | P0 | 1.0 | T14, T22 |
| T24 | Quiz 画面（SSE 受信 + 正解マイクロ演出 + リカバリーチケット表示） | Impl | P0 | 1.5 | T19, T20, T23 |
| T25 | History 画面 + 簡易ネットマップ（関連語5件リスト） | Impl | P1 | 0.7 | T21 |
| T26 | 摩擦ログ `/api/friction` + 週次レポート末尾の自由記述欄 | Impl | P2 | 0.3 | T08 |
| T27 | 縮退テーブル L1–L4 実装（cost-meter 連動 + 静的教材プール用 fallback JSON） | Infra | P1 | 0.7 | T05, T18 |
| T28 | 前夜23時 Batch 事前生成（`launchd` plist + `app pregenerate` サブコマンド） | Infra | P2 | 0.5 | T19 |
| T29 | Playwright E2E: Onboarding→Quiz→History 主要フロー | Test | P1 | 0.7 | T24, T25 |
| T30 | NGSL上位500語ゴールデンテスト + JSON Schema CI 検証 | Test | P1 | 0.5 | T16 |
| T31 | 起動時 LICENSES/ ファイル検証 + age 暗号化バックアップスクリプト | Infra | P1 | 0.3 | T09 |
| T32 | README + Phase1 動作確認手順 + ロールバック手順 + `make run/dev` | Docs | P1 | 0.3 | T28 |

**Sprint2 合計**: 12.2 人日

---

## 3. 依存グラフ & クリティカルパス

```
T01 ─┬─ T02 ── T03 ─┬─ T04 ─┬─ T11 ── T12 ─┐
     │              │       │              │
     │              │       └─ T13 ──┐     │
     │              ├─ T09 ──┐       │     │
     │              └─ T10 ──┘       │     │
     ├─ T05 ── T06 ─┬─ T15 ─┐        │     │
     │              ├─ T16 ─┼─ T18 ──┤     │
     │              │       └─ T17 ──┤     │
     ├─ T07 ───────────────────────  │     │
     └─ T08 ─────────────────── T14 ─┘     │
                                            │
              T15+T16+T17+T18+T07 ── T19 ──┤
                                            ├─ T24 ── T29
              T13 ──────────── T20 ────────┤
              T04 ── T21 ──────── T25 ─────┘
              T04 ── T22 ── T23 ────────────┘
```

**クリティカルパス**: T01→T02→T03→T04→T11→T12→T14→…→T15/T16→T19→T24→T29
**長さ**: 0.5+0.5+0.5+1.0+1.0+0.3+1.0+1.0+1.0+1.5+0.7 ≈ **9.0 人日**

並列化余地: T05/T07/T08 系（基盤整備）と T09/T10 系（データ整備）は T03 完了後に並走可能。

---

## 4. Sprint スケジュール

### Sprint 1: Foundation & 縦串（Day1–7）

- **Goal**: Onboarding 画面から DB 保存まで疎通。エージェント抜きで縦串が通る状態。
- **Deliverable**: `make dev` 起動 → ブラウザでオンボーディング20問完了 → users 行が保存される。
- **Demo シナリオ**: ブラウザで CEFR 自己申告 → 静的バンクから IRT 適応出題 → 結果 DB 確認。
- **Risk**:
  - sqlite-vec 動作不良 → T02 で早期判断、Plan B/C へ即移行
  - 静的問題バンク 30問作成の手間 → 初期20問で運用開始、5問単位で追加

### Sprint 2: エージェント & 残UI（Day8–14）

- **Goal**: 単語クイズが生成キャッシュ経由で動き、SRS と履歴・関連語表示が成立。
- **Deliverable**: 「今日の5分」ボタンから問題到達 → 解答 → 履歴更新 → 関連語確認。
- **Demo シナリオ**: 2日連続学習でストリーク2、リカバリーチケット表示、Reviewer ログ確認。
- **Risk**:
  - OpenAI Structured Outputs の refusal/length 切れハンドリング漏れ → T16 で正常系・異常系両方をユニット網羅
  - Reviewer コスト過多 → T27 縮退で対処

---

## 5. リスク

| リスク | 影響 | 確率 | 対策 |
|---|---|---|---|
| `modernc.org/sqlite/vec` が実在せず CGO 必須化 | 高 | 中 | T02 スパイクで Day1 判断、Plan B/C 用意済み |
| プレースメント問題 30問の作成負荷 | 中 | 高 | 初期20問で運用開始 + Phase 中の継続追加 |
| OpenAI gpt-4o-mini が Structured Outputs strict で refusal 頻発 | 中 | 中 | refusal ログ + 再試行2回 + 縮退L3で静的pool |
| Phase1 の 14日工期超過 | 中 | 中 | P2タスク（T26/T28/T32の一部）を Phase2 へ繰延可 |
| SSE が CORS/プロキシ環境で切断 | 中 | 低 | 同一オリジン + heartbeat 15s + reconnect クライアント側 |
| 個人開発の集中力ムラ | 中 | 高 | 1日5h前提（残3h はバッファ）、Sprint1 終了時に再見積もり |

---

## 6. 工期見積もりサマリ

| 区分 | 人日 |
|---|---|
| Sprint1 合計 | 9.3 |
| Sprint2 合計 | 12.2 |
| **合計（理論値）** | **21.5 人日** |
| 営業日換算（1日5h稼働） | 21.5 日 |
| 暦日換算（土日含む週5稼働） | 約 30 暦日 = **4.3 週** |
| design.md の「2週間で稼働」目標との差 | +2.3 週 |

### 工期調整オプション

- **A: 設計書通り 2週間（10営業日）厳守** → P2 タスク（T26/T28/T29 一部/T31/T32 一部）と Reviewer(T17) を縮退L2 同等 = 後回しに。実質 14 人日まで圧縮可能。**動くものは出るが運用性が下がる**。
- **B: 3週間（15営業日）に延長** → P1 全部入り、P2 のみカット。**推奨**。
- **C: 設計書全部入り** → 4.3週（約1か月）。Reviewer/縮退/E2E/バックアップまで完備。

**推奨**: **Option B（3週間 / 15営業日）**。MVP の本質（学習が回ること + コスト制御）を満たしつつ、リカバリチケットとモニタリングは確保される。

### 採用決定 (2026-05-27)

**Option B を採用**。3週間 / 15営業日（暦日換算 約4.3週）で Phase1 を進行する。
- P1 タスクは全て含める（Reviewer, 縮退L1–L4, E2E, ライセンス検証, バックアップ）
- P2 タスク（T26 摩擦ログ / T28 前夜Batch / T32 一部）はスプリント余力に応じて取込
- Sprint1 終了時に進捗を再見積もりし、超過時は P2 を Phase2 へ繰延

---

## 7. 次アクション

1. 本計画をユーザーレビュー → Option A/B/C 選択
2. GitHub Issues 化（CLAUDE.md のカンバン管理ルールに従い `feat/fix/chore` ラベル付与）
3. T01 着手前に `app.db`/`vocab.db` 配置パス・`.env` 構成を確定
4. T02 スパイク結果に応じて §1.3 の Plan A/B/C を本書に追記

---

## 8. 品質チェック

- [x] 全コンポーネント記載済（C1–C10）
- [x] 全タスクに依存・見積・優先度付き
- [x] 循環依存なし（DAG 検査済）
- [x] クリティカルパス特定済
- [x] リスクに対策あり
- [x] Sprint 容量と工期の整合（Sprint1 9.3/10, Sprint2 12.2/10→Option B推奨理由）
