package oauth

// OAuth エンドポイント / クライアント設定。
//
// 値は openai/codex (Codex CLI) の login サーバ実装に合わせている。上流変更時は
// ここを差し替える。テスト差し替えに対応できるよう var として定義している。
var (
	// ClientID は Codex CLI が利用するパブリッククライアント ID。
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// AuthorizeEndpoint はブラウザを誘導する OAuth 認可エンドポイント。
	AuthorizeEndpoint = "https://auth.openai.com/oauth/authorize"

	// TokenEndpoint は code → token / refresh_token 交換用エンドポイント。
	TokenEndpoint = "https://auth.openai.com/oauth/token"

	// Issuer は id_token 発行者 (検証用)。
	Issuer = "https://auth.openai.com"

	// RedirectHost はコールバックの待受ホスト。OpenAI 側に登録された
	// redirect_uri (http://localhost:1455/auth/callback) と完全一致させる必要が
	// あるため localhost を使う (127.0.0.1 では redirect_uri mismatch になる)。
	RedirectHost = "localhost"

	// RedirectPath はローカルコールバックのパス。
	RedirectPath = "/auth/callback"

	// LocalCallbackPort はローカル HTTP サーバの待受ポート。
	LocalCallbackPort = 1455

	// Scope は OAuth スコープ。offline_access で refresh_token を取得する。
	Scope = "openid profile email offline_access"
)
