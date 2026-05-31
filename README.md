# English-Learning (Phase 1)

CEFR ベースの英単語学習アプリ。SM-2 + IRT による出題制御、ChatGPT (OAuth サブスク) を利用した Curriculum/Generator/Reviewer 三段エージェントで毎日のセッションを生成する個人向けローカルアプリ。**API キー課金なし**で個人の ChatGPT Plus/Pro サブスク枠のみを利用する。

- 設計ドキュメント: [`docs/design.md`](docs/design.md)
- 実装計画: [`docs/plan.md`](docs/plan.md)

---

## 必要環境

| ツール | バージョン | 用途 |
|---|---|---|
| Go | 1.23+ | バックエンド (API / CLI) |
| Node.js | 20+ | フロントエンド (Vite + React) |
| age | 任意 | バックアップ暗号化 (`scripts/backup.sh`) |

ChatGPT Plus / Pro / Team / Enterprise いずれかのサブスクが必要 (Generator/Curriculum/Reviewer 用)。`app login` で OAuth ログインしていない場合、API は起動するが `/api/sessions/today` は「再ログイン要」エラーイベントを返す。

> 注意: 本アプリは [Codex CLI](https://github.com/openai/codex) と同形の OAuth + PKCE フローでログインする。Codex CLI が利用する `client_id` / エンドポイントを `internal/oauth/const.go` に集約しているため、上流変更時はそこを差し替える。
>
> 本リポジトリは個人ローカル運用のみを想定。他者へのホスティング提供は OpenAI 利用規約に抵触する可能性があるため行わない。

---

## セットアップ

```bash
# 1. 環境変数 (モデル名や DB パスを変更したい場合のみ)
cp .env.example .env

# 2. フロントエンド依存
cd web && npm install && cd ..

# 3. Go モジュール
go mod download

# 4. 単一バイナリビルド
make build

# 5. ChatGPT に OAuth ログイン (初回必須 / 再ログインは expiry 時)
./bin/app login            # ブラウザが開く → ChatGPT でログイン
./bin/app whoami           # 現在のアカウント / プラン表示
# ./bin/app logout         # ログアウトする場合 (auth.json 削除)
```

OAuth トークンはデフォルトで `~/.config/english-learning/auth.json` (パーミッション 0600) に保存される。`OAUTH_TOKEN_PATH` で上書き可能。

---

## 起動

```bash
make dev        # API + Vite を並走 (推奨: 開発時)
make run        # Go API のみ (go run ./cmd/app serve)
make build      # 単一バイナリを bin/app に出力 (web 資産を埋め込み)
```

---

## CLI (`cmd/app`)

| サブコマンド | 用途 |
|---|---|
| `app serve` | HTTP API サーバー起動 |
| `app login` / `app logout` / `app whoami` | ChatGPT OAuth ログイン管理 |
| `app migrate up` / `app migrate down` | DB マイグレーション (goose) |
| `app ingest --file data/seed/words.sample.json` | 単語データ取込 (embedding なし) |
| `app pregenerate` | 翌日分セッションを事前生成 (キャッシュ温め)。未ログイン時は skip |

---

## アーキテクチャ概要

```
[ Web (Vite/React) ]
        |
        v
[ Go API (cmd/app serve) ] -- SSE --> ブラウザ
   |  |  |
   |  |  +-- internal/agent  : Curriculum -> Generator -> Reviewer (3段)
   |  +----- internal/cache  : Generator 出力の決定論的キャッシュ
   +-------- internal/repo   : SQLite (app.db + vocab.db / WAL)
              ^
              | nightly batch (23:00)
              +-- cmd/app pregenerate  (launchd)
```

詳細は [`docs/plan.md` §1.1](docs/plan.md) を参照。

---

## 主要エンドポイント

| Method | Path | 用途 |
|---|---|---|
| GET | `/healthz` | ヘルスチェック |
| POST | `/api/onboarding` | 自己申告 CEFR からユーザー作成 |
| GET/POST | `/api/placement/*` | プレースメントテスト (IRT 適応) |
| GET | `/api/sessions/today` | 当日セッション (SSE) |
| POST | `/api/attempts` | 解答結果送信 (SM-2 更新) |
| GET | `/api/words/{id}` | 単語詳細 |
| GET | `/api/words/{id}/neighbors?k=5` | 関連語 (同 CEFR 3-gram Jaccard 上位 k 件) |
| GET | `/api/stats/weekly` | 週次統計 |
| GET | `/api/friction` | 学習摩擦シグナル |

---

## 動作確認手順

```bash
# 1. マイグレーション
go run ./cmd/app migrate up

# 2. 種データ取込
go run ./cmd/app ingest --file data/seed/words.sample.json

# 3. サーバー起動
make dev

# 4. ChatGPT ログイン (Generator/Curriculum/Reviewer を実呼び出しする場合)
go run ./cmd/app login

# 5. ヘルスチェック
curl -s http://127.0.0.1:8080/healthz

# 6. ブラウザで以下の順に確認
#    http://127.0.0.1:5173/onboarding  -> 自己申告 CEFR 登録
#    http://127.0.0.1:5173/dashboard   -> 当日セッション表示
#    http://127.0.0.1:5173/quiz        -> 解答 -> SM-2 更新
#    http://127.0.0.1:5173/history     -> 履歴確認
```

### 事前生成バッチ (launchd)

```bash
# プレースホルダ ({{INSTALL_DIR}}, {{HOME}}) を実パスに置換してインストール
sed -e "s|{{INSTALL_DIR}}|$(pwd)|g" -e "s|{{HOME}}|$HOME|g" \
  scripts/com.rai-tsumugu.englearn.pregenerate.plist \
  > ~/Library/LaunchAgents/com.rai-tsumugu.englearn.pregenerate.plist

launchctl load ~/Library/LaunchAgents/com.rai-tsumugu.englearn.pregenerate.plist
```

毎日 23:00 に `app pregenerate` を実行し、全ユーザー分の翌日セッションを事前生成してキャッシュへ格納する。

---

## ロールバック手順

### マイグレーション

```bash
go run ./cmd/app migrate down   # 1段戻す
```

### バックアップからの復元

`./scripts/backup.sh` で取得した age 暗号化バックアップを復元する。

```bash
# 例: 2026-05-26 のバックアップから app.db を復元
age -d -i ~/.age/identity.txt \
  backups/app.db.2026-05-26.age > /tmp/app.db.restored

# サーバーを停止してから差し替え
mv data/app.db data/app.db.broken
mv /tmp/app.db.restored data/app.db
```

### バッチ停止

```bash
launchctl unload ~/Library/LaunchAgents/com.rai-tsumugu.englearn.pregenerate.plist
```

---

## ライセンス

本リポジトリは個人開発プロジェクト。第三者データ (NGSL / CEFR-J / Octanove など) のライセンス・帰属表示は [`LICENSES/`](LICENSES/) を参照。
