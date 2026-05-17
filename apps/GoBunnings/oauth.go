package gobunnings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ClientCredentialsTokenSource struct {
	HTTP         *http.Client
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func NewClientCredentialsTokenSource(env Env, clientID, clientSecret string, scopes []string) (*ClientCredentialsTokenSource, error) {
	sub := ".sandbox"
	switch env {
	case EnvSandbox:
		sub = ".sandbox"
	case EnvTest:
		sub = ".stg"
	case EnvLive:
		sub = ""
	default:
		return nil, fmt.Errorf("unknown environment %q", env)
	}
	return &ClientCredentialsTokenSource{HTTP: &http.Client{Timeout: 20 * time.Second}, TokenURL: fmt.Sprintf("https://connect%s.api.bunnings.com.au/connect/token", sub), ClientID: clientID, ClientSecret: clientSecret, Scopes: scopes}, nil
}

func (t *ClientCredentialsTokenSource) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	if t.token != "" && time.Now().Add(60*time.Second).Before(t.expiry) {
		tok := t.token
		t.mu.Unlock()
		return tok, nil
	}
	t.mu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if len(t.Scopes) > 0 {
		form.Set("scope", strings.Join(t.Scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(t.ClientID, t.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := t.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("token endpoint returned empty access_token")
	}
	if tr.ExpiresIn == 0 {
		tr.ExpiresIn = 3600
	}
	t.mu.Lock()
	t.token = tr.AccessToken
	t.expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	t.mu.Unlock()
	return tr.AccessToken, nil
}
