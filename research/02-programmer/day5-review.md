# Day5 レビュー（プログラマ視点）

## 1. 目標達成可能性スコア: 8 / 10

Phase1の2週間稼働は現実的。ただしsqlite-vec alphaとSSE実装の落とし穴が未明示で、放置するとスケジュール遅延リスクあり。

## 2. 強み
1. **Qdrant廃止 + sqlite-vec採用**は個人用localhost規模に合致し、外部依存削減で運用コスト最小化（調査§1,§5と一致）。
2. **Curriculum→Generator→Reviewの直列3エージェント + JSON Schema強制**はGoのResponses API + Structured Outputs `strict:true` で素直に実装可能（§3）。
3. **Dev=Compose / 常用=単一バイナリのハイブリッド方針**は静的リンクGoの利点を活かし、127.0.0.1限定バインドでセキュリティも担保（§5）。

## 3. 弱み・盲点
1. **sqlite-vecがalpha版である事実とKNN制約（k=?必須、auxiliary列フィルタ不可）が未記載**。Goバインディング選定（CGO vs modernc）も未指定（§1）。
2. **SSE実装の必須要件（Flusher・heartbeat・X-Accel-Buffering・OpenAIストリーム再正規化）が未記述**。素通し実装だとスキーマ強制・トークン計上が崩壊（§2）。
3. **プレースメント20問を動的生成する記述**だが、コールドスタートで捏造idiomが混入すると初期プロファイル自体が歪む。静的freeze問題バンク化が必要（§4）。

## 4. 改訂提案
1. **§2 技術スタック**に「sqlite-vec 0.1.7-alpha / Goドライバ=`modernc.org/sqlite`+`modernc.org/sqlite/vec`（CGOフリー）」を明記し、§11残課題に「alpha破壊的変更監視・埋め込み再投入手順」を追加。
2. **§3.1 リクエストシーケンス**節に「`internal/sse`パッケージで `plan/question/done` 3イベントに正規化、15秒heartbeat、OpenAI delta は集約後配信」を追記。
3. **§4.1 オンボーディング**を「静的問題バンク（人手レビュー済みfreeze版）を使用、動的生成しない」に改訂し、§4.3 Review表に「review_score分布で10%→20%→40%自動昇格サーキット」を追加。
