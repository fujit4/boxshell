package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

	redirectURL := os.Getenv("BOX_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = defaultRedirectURL
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

	// Parse the redirect URL to start the server on the correct address.
	u, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redirect URL: %w", err)
	}

	codeChan := make(chan string)
	errChan := make(chan error)

	server := &http.Server{Addr: u.Host}

	http.DefaultServeMux = http.NewServeMux()
	http.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errChan <- fmt.Errorf("authentication failed: %s", errMsg)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("did not find 'code' query parameter")
			return
		}
		fmt.Fprintln(w, "Authentication successful! You can close this window.")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()
	defer server.Shutdown(ctx)

	fmt.Printf("Your browser has been opened to visit the following URL:\n%s\n", authURL)
	if err := exec.Command("cmd", "/C", "start", authURL).Start(); err != nil {
		fmt.Printf("Failed to open browser, please visit the URL manually: %v\n", err)
	}

	select {
	case code := <-codeChan:
		return cfg.Exchange(ctx, code)
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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