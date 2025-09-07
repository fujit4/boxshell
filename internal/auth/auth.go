package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"path/filepath"
)

const (
	redirectURL = "http://localhost:8585/oauth/callback"
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
	}

	return cfg.Client(ctx, token), nil
}

func newConfig() (*oauth2.Config, error) {
	clientID := os.Getenv("BOX_CLIENT_ID")
	clientSecret := os.Getenv("BOX_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("BOX_CLIENT_ID and BOX_CLIENT_SECRET must be set")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://account.box.com/api/oauth2/authorize",
			TokenURL: "https://api.box.com/oauth2/token",
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"root_readwrite", "manage_managed_users"},
	}, nil
}

func getNewToken(ctx context.Context, cfg *oauth2.Config) (*oauth2.Token, error) {
	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Your browser has been opened to visit the following URL:\n%s\n", authURL)

	// TODO: ブラウザを実際に開く処理 (後で実装)

	fmt.Println("Please enter the authorization code:")
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, err
	}

	return cfg.Exchange(ctx, authCode)
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
