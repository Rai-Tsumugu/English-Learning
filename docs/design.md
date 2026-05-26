# English-Learning 設計書 (v0.3)

> **改訂履歴**
> - v0.1 初版
> - v0.2 Design Council Round 2 で合格（平均9.20）
> - v0.3 Day5 7名チーム評価（平均7.14, 条件付きGo）の必須4＋強推奨3を反映

## 1. プロジェクト概要

localhost で動作する個人用英語学習サイト。OpenAI エージェントによる自律的学習コンテンツ生成と、英単語ネットマップを活用した超パーソナライズが特徴。

### 1.1 目的
毎日継続して英語の地力（**読む・語彙・文法**）をつける。ユーザーの現在地に合わせ難易度・日本語サポートを最適化する。リスニング補助（TTSシャドウイング）は Phase2 の任意拡張として扱う。

### 1.2 非目標
- マルチユーザー対応・認証基盤
- クラウドデプロイ・商用展開
- **スピーキング評価・発音矯正**（ELSA等の専用領域、本プロジェクトのスコープ外）

### 1.3 KPI（成功指標）
| 指標 | 目標 |
|---|---|
| 週次継続率（7日中の学習日数） | ≥ 6/7 |
| 週次新規習得語数 | ≥ 30語 |
| 平均正答率（週次） | 緩やかな単調増加 |
| **目標CEFR残語数の月次減少** | 月 ≥ 100語減 |
| 月次OpenAIコスト | ≤ $5（実運用目標 ≤ $1） |
| 生成キャッシュヒット率 | ≥ 60% |

### 1.4 MVP分割
- **Phase1（2週間で稼働）**: ダッシュボード + 単語クイズ + ストリーク + 履歴 + **簡易ネットマップ（関連語5件リスト）**。単一エージェント、SQLiteのみ、sqlite-vec、生成キャッシュ、Reviewerは別系統で起動。
- **Phase2**: 長文読解 + 日本語サポート3モード + コロケーション必須化 + TTSシャドウイング枠 + FSRS-5 移行。
- **Phase3**: ネットマップのリッチ可視化 + 弱点クラスタ提示。

### 1.5 Phase移行Gate
- Phase1 → Phase2: 週6/7継続 × 2週連続 達成
- Phase2 → Phase3: 月100語以上の語彙獲得が2か月連続 達成

## 2. 技術スタック

| レイヤ | 技術 | 備考 |
|---|---|---|
| Frontend | React (Vite + TypeScript) + Tailwind | |
| Backend | Go (chi router) | 単一バイナリ |
| DB | SQLite (WALモード) + sqlite-vec `0.1.x-alpha` | Qdrant廃止 |
| Goドライバ | `modernc.org/sqlite` + `modernc.org/sqlite/vec` | **CGOフリー**、単一バイナリ堅持 |
| マイグレーション | goose（`embed.FS` 同梱） | `app migrate up` サブコマンド |
| AI (Generator) | OpenAI gpt-4o-mini + Structured Outputs `strict:true` | Batch API + プロンプトキャッシュ採用 |
| AI (Reviewer) | **gpt-4o（別系統）** | self-bias緩和、新形式は100%レビュー |
| Embedding | text-embedding-3-small | |
| 実行環境 | Docker Compose（dev）/ Goバイナリ単体（常用） | `127.0.0.1` 限定バインド |
| テスト | Go test, Playwright, LLM-as-judge | ゴールデンテストはNGSL上位500語 |

## 3. システム構成

```
[Browser:5173] ── React SPA
        │ REST/JSON + SSE
        ▼
[Go API :8080] ──┬── SQLite app.db (履歴/生成キャッシュ/attempts)
        │        ├── SQLite vocab.db (語彙/例文/埋め込み, CC BY-SA分離)
        │        └── sqlite-vec (KNN: k=?必須)
        ▼
[OpenAI API]
  ├ Curriculum (gpt-4o-mini)
  ├ Generator (gpt-4o-mini, Batch可)
  └ Reviewer (gpt-4o, 別系統)
```

### 3.1 リクエストシーケンス（本日の学習）

```
User → GET /api/sessions/today
  Server: 履歴+苦手単語+SRSキュー集約
        → CurriculumAgent (JSON Schema, 類義語禁止/混在制約)
        → GeneratorAgent (Batch可, answer_span/cefr_evidence必須)
        → ReviewAgent (10%サンプリング, 新形式は100%)
        → generated_content にハッシュキャッシュ保存
        → SSEで段階返却
```

#### SSE実装規約
- `internal/sse` パッケージで `plan` / `question` / `done` / `error` の4イベントに正規化
- `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`
- `http.Flusher.Flush()` をイベント毎に強制、`r.Context().Done()` 購読で goroutine leak 防止
- 15秒間隔の `: ping` heartbeat
- OpenAI ストリームは素通しせず、サーバ側で集約後に上記4イベントへ変換

## 4. 主要機能

### 4.1 オンボーディング（コールドスタート）
1. CEFR自己申告（A1〜C2、ベイズ事前分布として使用）
2. **静的問題バンク**（人手レビュー済み・freeze版）から20問の適応出題
   - 動的生成は禁止（捏造idiom による初期プロファイル歪み回避）
   - 各問に IRT a/b パラメータ事前付与
   - 5問ごとに進捗バー表示、中断再開可能
   - 停止規準: SEM ≤ 0.3 または 20問到達
3. 結果から初期難易度プロファイルを `users` に保存（1行のみ）

### 4.2 単語ネットマップ
- **NGSL + CEFR-J + Octanove C1/C2**（全て CC BY-SA 4.0）を取り込み、各単語に CEFR + 独自難易度スコア + 例文最低3つ
- COCA は頻度スコア参照のみ（DB再配布禁止、ローカル参照のみ）
- text-embedding-3-small で埋め込み生成、sqlite-vec で近傍探索（`k=?` 強制、後段フィルタ）
- 関連語＝コサイン類似度上位 + 共起語
- **用途限定**: ネットマップは「復習・可視化・関連語提示」専用。**新規出題には使用しない**（意味干渉回避）

### 4.3 自律的コンテンツ生成（エージェント設計）

| Agent | モデル | 入力 | 出力 | 失敗時 |
|---|---|---|---|---|
| Curriculum | gpt-4o-mini | 履歴/SRSキュー/プロファイル | 出題プラン（語ID＋形式） | プロファイルfallback |
| Generator | gpt-4o-mini | 出題プラン | 問題JSON + `answer_span` + `cefr_evidence` | 再試行 最大2回、length切れは max_tokens 1.5倍拡張 |
| Reviewer | **gpt-4o（別系統）** | 生成物 | 4段階ルーブリック + 理由文 | 10%サンプリング、`review_score`分布で 10%→20%→40% 自動昇格サーキット |

- 全I/Oは JSON Schema `strict:true`、`additionalProperties:false`、Go struct から `invopop/jsonschema` で自動生成
- 新出題形式・新規CEFR帯の初出時は **100%レビュー**
- 3回連続NGで人手通知、サーキットブレーカー作動
- プロンプトインジェクション対策: ユーザー入力は `<user_input>...</user_input>` で囲い、システムプロンプトで「指示と解釈しない」固定
- refusal フィールドを必ず受領・ログ化

#### Curriculum Agent 出題制約（学習科学準拠）
- **類義語・近傍語の同時出題禁止**（Tinkham 干渉効果回避）
- 品詞・意味カテゴリ・CEFR帯を**混在**させる（interleaving）
- A1〜A2 は初回 blocked → 2回目以降 interleaved のハイブリッド
- 1日新規語上限 10〜15 語
- **未消化SRSが3日分超で新規語投入を自動停止**（過負荷ブレーカー）

### 4.4 パーソナライズ
- **Phase1**: SRS（SM-2、`ease_factor`、`next_review_at`、lapse時 ease 下限 1.3）
- **Phase2**: FSRS-5 へ移行（`stability` / `difficulty` / `lapses` 列を Phase1 から先行追加）
- 苦手単語は重み付け強化
- 長文中の未習得単語に自動ツールチップ訳（Phase2）
- `latency_ms` から自動 quality 推定（解答時間が極端に長い→自信なし扱い）

### 4.5 日本語サポート（3モード、Phase2）
全訳併記 / タップで訳 / 英語のみ — 学習中に即切替可能

### 4.6 学習継続支援（UX）

| 要素 | 設計 |
|---|---|
| メインCTA | ダッシュボード中央に巨大「今日の5分を始める」 |
| 事前生成 | 前夜23時バッチ（Batch API 50%OFF）→ 朝のゼロ待機 |
| ストリーク表示 | **週7マスの埋まり可視化**（all-or-nothine心理回避） |
| リカバリーチケット | **月2枚を自動付与**、使用時コピー「賢く休んだ」 |
| 即時報酬 | 正解時マイクロ演出（語点灯 + 音 + 「次に会える日」表示） |
| 通知 | macOS `launchd` 1日1回 ambient cue（過剰通知回避） |
| 摩擦ログ | 週次レポートに「今週UIで詰まった瞬間」自由記述欄（dogfooding 盲点検知） |
| 進捗可視化 | ダッシュボードに「累積分/週」「目標CEFR残語数」表示 |
| 週次拡張 | 週1回15分の拡張セッション（長文・ディクテーション、Phase2） |

## 5. データモデル

```sql
-- app.db (運用DB、独自ライセンス)
CREATE TABLE users (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  cefr TEXT, difficulty_profile JSON,
  desired_retention REAL DEFAULT 0.9,
  created_at DATETIME
);

CREATE TABLE attempts (
  id INTEGER PRIMARY KEY, word_id INTEGER,
  result INTEGER, latency_ms INTEGER,
  -- SM-2 (Phase1)
  next_review_at DATETIME, ease_factor REAL,
  -- FSRS-5 先行カラム (Phase2移行)
  stability REAL, difficulty REAL, lapses INTEGER,
  tokens_used INTEGER, created_at DATETIME
);

CREATE TABLE generated_content (
  hash TEXT PRIMARY KEY,  -- sha256(model || schema_ver || prompt_version || prompt_inputs)
  payload JSON, model TEXT, prompt_version TEXT, tokens INTEGER,
  reviewed INTEGER, review_score REAL, reviewer_model TEXT,
  created_at DATETIME
);

CREATE TABLE friction_log (
  id INTEGER PRIMARY KEY, week TEXT, note TEXT, created_at DATETIME
);

-- vocab.db (CC BY-SA 4.0 ライセンス分離)
CREATE TABLE words (
  id INTEGER PRIMARY KEY, lemma TEXT UNIQUE,
  cefr TEXT, difficulty REAL,
  source TEXT NOT NULL  -- 'NGSL' | 'CEFR-J' | 'Octanove'
);

CREATE TABLE word_vec (id INTEGER PRIMARY KEY, embedding BLOB); -- sqlite-vec

CREATE TABLE examples (
  id INTEGER PRIMARY KEY, word_id INTEGER REFERENCES words(id),
  text TEXT, ja TEXT, cefr TEXT,
  collocation TEXT,            -- 高頻度コロケート（Phase2必須）
  source TEXT NOT NULL,        -- 'Tatoeba' | 'Tanaka' | 'generated'
  attribution TEXT             -- CC BY のクレジット要件
);

-- 静的プレースメント問題バンク（人手レビュー済 freeze）
CREATE TABLE placement_items (
  id INTEGER PRIMARY KEY, cefr TEXT, payload JSON,
  irt_a REAL, irt_b REAL  -- IRT 2PL パラメータ
);
```

インデックス: `attempts(next_review_at)`, `attempts(word_id)`, `examples(word_id)`, `placement_items(cefr)`

## 6. API仕様（抜粋・OpenAPI形式）

| Method | Path | 用途 |
|---|---|---|
| POST | /api/onboarding | プレースメント結果保存 |
| GET | /api/placement/next | 静的バンクから次の1問（IRT適応） |
| GET | /api/sessions/today | 本日の学習プラン（SSE） |
| POST | /api/attempts | 解答結果記録 |
| GET | /api/words/:id | 単語詳細 |
| GET | /api/words/:id/neighbors | 関連語（ネットマップ、復習用） |
| GET | /api/stats/weekly | 週次レポート（残語数・キャッシュ率・コスト含む） |
| POST | /api/friction | 摩擦ログ投稿 |

エラー形式は RFC 7807 (Problem Details) 統一。

## 7. セキュリティ要件

| 項目 | 方針 |
|---|---|
| シークレット | `.env` + `.gitignore` 必須、Docker secrets で注入、ログマスキング |
| ポート | `127.0.0.1:8080` 限定バインド、LAN露出禁止 |
| CORS | `http://127.0.0.1:5173` のみ allowlist、Origin/Referer 検証、SSE は同一オリジン |
| コンテナ | 非rootユーザー、`read_only: true`, `cap_drop: [ALL]`、distroless/scratch |
| データ保護 | SQLite/sqlite-vec ボリュームを `chmod 700`、age による暗号化バックアップ（鍵は別ボリューム） |
| OpenAI送信データ | 履歴は単語ID集約のみ。自由記述PIIは送らない |
| プロンプトインジェクション | JSON Schema 強制 + 入力タグ分離 + allowlist 検証 + refusal ログ |
| コスト/レート | OpenAI usage hard limit $5、サーバ側で1日トークン上限・サーキットブレーカー |
| 依存管理 | Dependabot、定期 `govulncheck` |
| ライセンス整合性 | 起動時に `data/sources/LICENSES/` を検証、欠落で起動失敗 |

## 8. コスト試算

| 項目 | 試算 |
|---|---|
| 通常生成 (1日) | input 20k + output 10k tok → $0.009 |
| Batch + キャッシュ 75%ヒット時 (1日) | $0.003〜0.005 |
| 月次予算 | $5（hard limit）／**実運用目標 $1 以下** |
| キャッシュヒット率目標 | ≥ 60% |
| 浮いた予算の使途 | Reviewer 強化（gpt-4o 100%レビュー枠の拡張） |

### 8.1 縮退テーブル（予算 / キャッシュ未達時）

| 段階 | トリガ | アクション |
|---|---|---|
| L1 | 日次予算 50% 消費 | Reviewer サンプリング 10% に固定 |
| L2 | 日次予算 80% 消費 | Reviewer スキップ、Batch のみ |
| L3 | 日次予算超過 | 当日は **静的教材プール**（事前生成済み）から配信 |
| L4 | キャッシュヒット率 < 40% × 3日 | Generator 一時停止、人手調査通知 |

## 9. テスト戦略

- **Go test**: ハンドラ / リポジトリ / SRS / IRT スコアリング
- **ゴールデンテスト**: NGSL頻度上位500語の例文生成スナップショット
- **LLM-as-judge**: Reviewer 自体の評価（週次バッチ、別モデルで監査）
- **Playwright E2E**: オンボーディング → クイズ → 履歴の主要フロー
- **JSON Schema CI 検証**: Go struct ↔ schema 自動同期テスト

## 10. 運用・バックアップ

- `app.db` / `vocab.db` を 1日1回 `sqlite3 .backup` → age 暗号化 → ローカルディレクトリ（鍵は別パス、`chmod 600`）
- マイグレーションは goose、起動時自動適用、ロールバック手順を README に記載
- ログは JSON 構造化、APIキーは自動マスク
- observability: 日次トークン消費・キャッシュヒット率・p95レイテンシ・Review 合格率をログ出力
- `make run`（go run）/ `make dev`（Compose hot reload）/ 常用は `launchd` で単一バイナリ常駐

## 11. 残課題・今後

- Phase2 長文素材: Project Gutenberg 第一選定、JParaCrawl / OPUS は localhost 利用に限定
- Phase2 TTS: OpenAI TTS のシャドウイング枠（1日1文）の費用対効果検証
- Phase3 ネットマップのリッチ可視化UI設計
- sqlite-vec の alpha 破壊的変更監視と埋め込み再投入手順整備
- FSRS-5 Go 実装の選定（`open-spaced-repetition/go-fsrs` 等）
