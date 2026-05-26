# Phase1 技術的実現性 実地調査レポート

対象: English-Learning Phase1（2週間稼働 / 単一ユーザー / localhost）

## 1. sqlite-vec の安定性・本番事例・Goバインディング

### 結論
Phase1での採用は妥当だが「**alpha版である**」前提のリスク管理が必須。Qdrant廃止判断は個人用localhost規模では正解。

### 根拠
- 最新版は `0.1.7-alpha.10`、依然 alpha。`ncruces` バインディングは更新停止中との issue 報告あり。
- Goバインディングは2系統:
  - (a) CGO前提 `github.com/asg017/sqlite-vec-go-bindings/cgo`
  - (b) CGOフリー `modernc.org/sqlite/vec`（2026年4月更新）
- KNN制約: `vec0` は `LIMIT` または `k=?` 必須、`distance` 列での閾値絞込・ページング未サポート、auxiliary columns はフィルタ不可、`JOIN+WHERE` でも事前フィルタが最適化されないケースあり。
- 性能: 1536次元 float で約105ms/クエリ、bit量子化なら 3072次元でも11ms。NGSL 2,800語規模なら余裕。

### 設計への反映提案
1. Goドライバは **`modernc.org/sqlite` + `modernc.org/sqlite/vec` を第一候補**（CGOなしで単一バイナリ成立）
2. 近傍探索クエリは `k=?` を強制するラッパ関数で集約
3. CEFR・既習フィルタはKNN後段で適用（`k=50`取って後段フィルタ）
4. `embedding` を別テーブルに分離、ローカルJSONLにもキャッシュし再投入可能化
5. 起動時に拡張バージョンをログ出力し破壊的変更を検出

## 2. Go chi + SSE 実装パターンと既知の罠

### 結論
chi はSSEに特別対応不要だが、**Flusher・Context・heartbeat・プロキシ無効化**の4点必須。OpenAI Responses APIストリームは **素通しせず一度集約して自前のSSEイベント名で再配信**する。

### 根拠
- 必須ヘッダ: `Content-Type: text/event-stream` / `Cache-Control: no-cache` / `Connection: keep-alive` / `X-Accel-Buffering: no`
- Goの `ResponseWriter` はバッファするため **イベント毎に `http.Flusher.Flush()` 必須**
- 既知の罠:
  1. Flusher型アサーション忘れpanic
  2. `r.Context().Done()` 未購読でgoroutine leak
  3. 15〜30秒間隔の `: ping` heartbeat必須
  4. ブラウザの6接続/ドメイン制限
- OpenAI Responses API は `stream=true` で多数のイベント型を返す。素通しすると **スキーマ強制・トークン計上・サーキットブレーカ判定が不能**

### 設計への反映提案
1. `internal/sse` パッケージで `Writer.Send(event, data)` / `Heartbeat(ctx)` / `Close()` を提供
2. `/api/sessions/today` は **3段階イベント正規化** (`plan`→`question`→`done`)、上流deltaを隠蔽
3. `http.Server.WriteTimeout=0`、ハンドラ単位で `context.WithTimeout(r.Context(), 60s)`
4. SPA側は `fetch`+ReadableStream（POSTヘッダ送信可）。Phase1なら EventSource でも可

## 3. OpenAI Agents SDK / Function calling / Structured Outputs

### 結論
**Phase1は Agents SDK を使わず Responses API + Structured Outputs (`strict: true`) で十分**。Go公式SDK未対応、Curriculum→Generator→Reviewは直列パイプラインで handoff 不要。

### 根拠
- 2025年3月 Responses API・Agents SDK公開だが、Agents SDK は Python/TS のみ
- Structured Outputs は `strict: true` で **JSON Schema 完全準拠を保証**、`gpt-4o-mini` 対応
- 制約: スキーマに `additionalProperties: false`・全プロパティ `required` 必要、optional は nullable で表現、再帰深度上限あり

### 設計への反映提案
1. Goは **`sashabaranov/go-openai` で Responses API + `response_format: json_schema` を直接利用**
2. Curriculum/Generator/Review に対応する Go struct と JSON Schema を **`invopop/jsonschema` 等で自動生成**
3. Review は別APIコール、10%サンプリングは Go側 rand 判定で完結
4. Structured Outputs の **refusal フィールド**を必ず受領・ログ化

## 4. gpt-4o-mini の英語教育コンテンツ生成品質

### 結論
**A1〜B2の語彙クイズ・短文例には十分**。ただし**捏造idiom・誤コロケーション**を生成しうるため Review Agent省略不可。

### 根拠
- MMLU 82.0%、HumanEval 87.2%、MGSM 87.0%、トークナイザ改良で非英語も安価
- 教育応用事例あり
- **重要リスク**: 学習者が誤った構造・捏造idiomに晒されると知識に組み込まれる

### 設計への反映提案
1. Review サンプリング率は10%スタートで `review_score` 分布監視、閾値割れで自動 20→40% に昇格するサーキット
2. プロンプトに「実在する用法のみ、Oxford/COCA に存在するコロケーション限定、不確実なら refusal」明示
3. ゴールデンテストに NGSL頻度上位500語の例文生成スナップショット + LLM-as-judge 回帰検出
4. オンボーディングのプレースメントは **静的問題バンクを初期生成→人手レビュー後 freeze**、動的生成しない
5. 日本語訳キャッシュもReviewでスコアリング

## 5. Docker Compose vs 単一バイナリ運用（個人用localhost）

### 結論
**dev=Compose / 常用=単一バイナリのハイブリッドが最適**（設計書方針で正解）。SQLite+sqlite-vec構成は外部依存ゼロのためComposeのメリットは「再現性」のみ、起動コストの方が大きい。

### 根拠
- Goのstatic linkバイナリは2-10MB、依存ゼロ
- Composeの価値は複数サービスのオーケストレーション・分離。SQLite同梱の本構成では不要
- シングルホスト運用にはCompose十分、Swarmすら不要

### 設計への反映提案
1. `make run` は `go run`、`make dev` で Compose（フロントhot reload + air）。日次起動は `launchd`/`systemd --user` で単一バイナリ常駐
2. **SQLite WAL ファイル群はホスト直マウント**（Compose named volume だとバックアップ困難）
3. Dockerイメージは distroless/scratch、非root、`read_only: true`、`cap_drop: [ALL]`、`127.0.0.1:8080:8080`限定
4. マイグレーション(goose)は `embed.FS` 同梱+`app migrate up` サブコマンド化
5. age暗号化バックアップは **ホスト側cron**で実行

## 総括: Phase1 落とし穴トップ5

1. **sqlite-vec alpha起因のスキーマ変更** → 埋め込み再生成可能な分離設計＋ローカルJSONLキャッシュで吸収
2. **SSE Flush/Heartbeat忘れ**による無反応 → `internal/sse` パッケージで集約
3. **Structured Outputs スキーマ違反**（`additionalProperties` 等） → struct→schema 自動生成＋CI検証
4. **gpt-4o-mini の捏造idiom** → Review Agent + ゴールデンテスト + プロンプトに辞書根拠要求
5. **CGO依存でDockerビルド肥大** → `modernc.org/sqlite` で純Go単一バイナリ堅持

## 出典
- sqlite-vec: https://github.com/asg017/sqlite-vec
- modernc.org/sqlite/vec: https://pkg.go.dev/modernc.org/sqlite/vec
- sqlite-vec KNN docs: https://alexgarcia.xyz/sqlite-vec/features/knn.html
- thedevelopercafe SSE in Go: https://thedevelopercafe.com/articles/server-sent-events-in-go-595ae2740c7a
- OpenAI Streaming guide: https://developers.openai.com/api/docs/guides/streaming-responses
- OpenAI Agents guide: https://developers.openai.com/api/docs/guides/agents
- Structured Outputs: https://openai.com/index/introducing-structured-outputs-in-the-api/
- gpt-4o-mini benchmarks: https://www.hixx.ai/blog/xxai-news/surfacing-gpt-4o-mini
- BretFisher AMA #146: https://github.com/BretFisher/ama/discussions/146
