# Third-Party Licenses (Phase1 暫定)

本プロジェクトは以下のサードパーティ Go モジュールおよび公開データ源を
利用しています。各ライセンス全文は公式リポジトリ／配布元を参照してください。
正式な版は Phase2 以降で `go-licenses` 等により自動生成へ移行予定。

## Go modules (主要)

| モジュール | ライセンス | 用途 |
|---|---|---|
| github.com/go-chi/chi/v5 | MIT | HTTP ルーター |
| github.com/go-chi/cors | MIT | CORS ミドルウェア |
| modernc.org/sqlite | BSD-3-Clause | Pure-Go SQLite ドライバ |
| github.com/pressly/goose/v3 | MIT | DB マイグレーション |
| (その他 go.mod に記載) | 各 OSS ライセンス | — |

## 公開データ源

| データセット | ライセンス | 用途 |
|---|---|---|
| NGSL (New General Service List) | CC BY-SA 4.0 | 語彙頻度リスト |
| CEFR-J Wordlist | CC BY-SA 4.0 (要確認) | CEFR レベルタグ |
| Octanove Vocabulary Profile | 各配布条件に従う | 補助的な難易度推定 |

## 設計参考 (コード非依存)

`internal/oauth/` の PKCE フローおよび `internal/chatgpt/` の Responses API クライアントは、
OpenAI の [Codex CLI (`openai/codex`)](https://github.com/openai/codex) の認証実装を**設計参考**としている。
本リポジトリには Codex CLI のソースコードは含まれていないが、`client_id` / エンドポイント等の
定数値は Codex CLI と同じものを使用する想定 (`internal/oauth/const.go` 参照)。
これらは公開仕様の一部とみなし、上流変更があれば追従する。

## 注意

- 本ファイルは Phase1 のプレースホルダであり、最終的なライセンス表示は
  リリース前に正式版へ差し替えること。
- 新規依存追加時はここに 1 行追加するか、`go-licenses report ./...` で再生成する。
- 本アプリは ChatGPT サブスクリプションを個人ローカルで利用する前提。他者ホスティング
  提供は OpenAI 利用規約に抵触する可能性があるため行わない旨を README に明記する。
