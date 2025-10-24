package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"boxshell/internal/boxapi"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

const (
	defaultRedirectURL = "http://localhost:8585/oauth/callback"
)

// NewClient は認証済みの http.Client を返します。
// トークンが存在すればそれを使用し、なければ新しいトークンを取得するフローを開始します。
func NewClient(ctx context.Context) (*http.Client, error) {
	cfg, err := newConfig()
	if err != nil {
		return nil, err
	}

	tokenPath, err := getTokenPath()
	if err != nil {
		return nil, err
	}

	token, err := LoadToken(tokenPath)

	restRetry := 1
RetryPointOfRefleshTokenExpired:

	if err != nil {
		// トークンがないか、読み込みに失敗した場合
		fmt.Println("No token found. Starting new authentication flow.")
		token, err = getNewToken(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if err := SaveToken(tokenPath, token); err != nil {
			return nil, err
		}
		fmt.Printf("Authentication successful. Token saved to %s\n", tokenPath)
	} else {
		// トークンの有効性を確認
		client := boxapi.NewClient(cfg.Client(ctx, token))
		user, err := client.GetMe(ctx)
		isExpired := false
		if err != nil {
			// API呼び出し失敗＝トークン切れと判断
			isExpired = true
		}

		if isExpired && restRetry > 0 {
			restRetry--
			fmt.Println("Token has expired. Retrying to get a new token...")
			err = fmt.Errorf("token expired and will be refreshed") // errをnon-nilにしてgoto後のifに入るようにする
			goto RetryPointOfRefleshTokenExpired
		} else if err == nil {
			// ユーザー情報を表示
			fmt.Printf("Authenticated as: %s (%s)\n", user.Name, user.Login)
		}
	}

	return cfg.Client(ctx, token), nil
}

func newConfig() (*oauth2.Config, error) {
	clientID := os.Getenv("BOX_CLIENT_ID")
	clientSecret := os.Getenv("BOX_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("BOX_CLIENT_ID and BOX_CLIENT_SECRET must be set")
	}

	// 環境変数からリダイレクトURLを取得。localhost以外は許可しない
	redirectURL := os.Getenv("BOX_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = defaultRedirectURL
	} else {
		if !strings.HasPrefix(redirectURL, "http://localhost:") {
			return nil, fmt.Errorf("invalid redirect URL: %s. must start with http://localhost:", redirectURL)
		}
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://account.box.com/api/oauth2/authorize",
			TokenURL: "https://api.box.com/oauth2/token",
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"root_readwrite"},
	}, nil
}

func getNewToken(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {

	var token *oauth2.Token

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}

	// stateはcsrfトークン
	state := base64.RawURLEncoding.EncodeToString(b)
	// verifierはPKCE
	verifier := oauth2.GenerateVerifier()
	// リフレッシュトークンを発行してもらうにはOffiline
	authCodeURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))

	parsedURL, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redirect URL: %w", err)
	}
	port := parsedURL.Port()
	if port == "" {
		return nil, fmt.Errorf("port not found in redirect URL")
	}

	// コールバックを受け取るウェブサーバーをセットアップ
	code := make(chan string)
	var server *http.Server
	server = &http.Server{
		Addr: fmt.Sprintf(":%s", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// クエリーパラメータからcodeを取得し、ブラウザを閉じる
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<html><script>window.open('about:blank','_self').close()</script></html>")
			w.(http.Flusher).Flush()
			code <- r.URL.Query().Get("code")
			// サーバーも閉じる
			server.Shutdown(context.Background())
		}),
	}
	go server.ListenAndServe()

	// ブラウザで認可画面を開く
	// 認可が完了すれば上記のサーバーにリダイレクト
	open.Start(authCodeURL)

	var codeVal string
	codeVal = <- code
	token, err = cfg.Exchange(ctx, codeVal, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, err
	}

	return token, nil

}

// getTokenPath はトークンを保存するパスを決定します。
func getTokenPath() (string, error) {
	dir, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok || dir == "" {
		dir = os.Getenv("LOCALAPPDATA")
		if dir == "" {
			return "", fmt.Errorf("XDG_DATA_HOME and LOCALAPPDATA are not set")
		}
	}
	return filepath.Join(dir, "boxshell", "tokens.json"), nil
}

// SaveToken はトークンをファイルに保存します。
func SaveToken(path string, token *oauth2.Token) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// LoadToken はファイルからトークンを読み込みます。
func LoadToken(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var token oauth2.Token
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return nil, err
	}
	return &token, nil
}
