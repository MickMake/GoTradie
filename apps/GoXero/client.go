package goxero

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAuthorizationEndpoint = "https://login.xero.com/identity/connect/authorize"
	defaultTokenEndpoint         = "https://identity.xero.com/connect/token"
	defaultConnectionsEndpoint   = "https://api.xero.com/connections"
	maximumResponseBytes         = 1 << 20
)

// Config contains the public OAuth settings for a Xero PKCE application.
type Config struct {
	ClientID    string
	RedirectURI string
	Scopes      []string
}

// Tokens contains the current Xero OAuth credentials returned by an exchange
// or refresh. Callers are responsible for secure persistence.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// Connection identifies a Xero tenant available to the authenticated user.
type Connection struct {
	TenantID   string `json:"tenantId"`
	TenantName string `json:"tenantName"`
	TenantType string `json:"tenantType"`
}

// HTTPError reports a rejected Xero HTTP operation without exposing a
// credential-bearing response body.
type HTTPError struct {
	Operation  string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Xero %s request failed: HTTP %d", e.Operation, e.StatusCode)
}

// AuthSession contains one login attempt's ephemeral verifier and state. Its
// fields are deliberately private so callers cannot persist or log them.
type AuthSession struct {
	verifier string
	state    string
}

// Client implements only OAuth PKCE, token refresh and connection discovery.
type Client struct {
	config                Config
	httpClient            *http.Client
	random                io.Reader
	now                   func() time.Time
	authorizationEndpoint string
	tokenEndpoint         string
	connectionsEndpoint   string
}

// NewClient creates a Xero authentication client.
func NewClient(cfg Config, httpClient *http.Client) (*Client, error) {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.RedirectURI = strings.TrimSpace(cfg.RedirectURI)
	cfg.Scopes = append([]string(nil), cfg.Scopes...)
	if cfg.ClientID == "" {
		return nil, errors.New("Xero client ID is required")
	}
	redirect, err := url.Parse(cfg.RedirectURI)
	if err != nil || redirect.Host == "" || (redirect.Scheme != "http" && redirect.Scheme != "https") {
		return nil, errors.New("Xero redirect URI must be an absolute http or https URL")
	}
	if len(cfg.Scopes) == 0 {
		return nil, errors.New("at least one Xero scope is required")
	}
	seen := make(map[string]struct{}, len(cfg.Scopes))
	for index, scope := range cfg.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return nil, errors.New("Xero scopes must not contain blank values")
		}
		if _, exists := seen[scope]; exists {
			return nil, fmt.Errorf("Xero scope %q is duplicated", scope)
		}
		seen[scope] = struct{}{}
		cfg.Scopes[index] = scope
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		config: cfg, httpClient: httpClient, random: rand.Reader, now: time.Now,
		authorizationEndpoint: defaultAuthorizationEndpoint,
		tokenEndpoint:         defaultTokenEndpoint,
		connectionsEndpoint:   defaultConnectionsEndpoint,
	}, nil
}

// BeginAuth creates a fresh PKCE verifier and state and returns the Xero
// authorization URL for this single login attempt.
func (c *Client) BeginAuth() (*AuthSession, string, error) {
	verifier, err := randomString(c.random, 64)
	if err != nil {
		return nil, "", fmt.Errorf("create PKCE verifier: %w", err)
	}
	state, err := randomString(c.random, 32)
	if err != nil {
		return nil, "", fmt.Errorf("create OAuth state: %w", err)
	}
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])

	authorizationURL, err := url.Parse(c.authorizationEndpoint)
	if err != nil {
		return nil, "", fmt.Errorf("parse Xero authorization endpoint: %w", err)
	}
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.config.ClientID)
	query.Set("redirect_uri", c.config.RedirectURI)
	query.Set("scope", strings.Join(c.config.Scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	authorizationURL.RawQuery = query.Encode()
	return &AuthSession{verifier: verifier, state: state}, authorizationURL.String(), nil
}

// CompleteAuth validates the callback state before exchanging the code.
func (c *Client) CompleteAuth(ctx context.Context, session *AuthSession, code, returnedState string) (Tokens, error) {
	if err := c.ValidateCallback(session, returnedState); err != nil {
		return Tokens{}, err
	}
	if code = strings.TrimSpace(code); code == "" {
		return Tokens{}, errors.New("Xero authorization code is missing")
	}
	values := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.config.ClientID},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURI},
		"code_verifier": {session.verifier},
	}
	return c.exchangeTokens(ctx, "token exchange", values)
}

// ValidateCallback verifies the state returned by Xero for either a successful
// callback or an OAuth error callback.
func (c *Client) ValidateCallback(session *AuthSession, returnedState string) error {
	if session == nil || session.verifier == "" || session.state == "" {
		return errors.New("Xero authentication session is missing")
	}
	if subtle.ConstantTimeCompare([]byte(session.state), []byte(returnedState)) != 1 {
		return errors.New("Xero callback state did not match")
	}
	return nil
}

// Refresh exchanges a refresh token for Xero's rotated token pair.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	if refreshToken = strings.TrimSpace(refreshToken); refreshToken == "" {
		return Tokens{}, errors.New("Xero refresh token is missing")
	}
	values := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.config.ClientID},
		"refresh_token": {refreshToken},
	}
	return c.exchangeTokens(ctx, "token refresh", values)
}

// Connections returns the tenants currently connected to the authenticated
// Xero user. This operation is read-only.
func (c *Client) Connections(ctx context.Context, accessToken string) ([]Connection, error) {
	if accessToken = strings.TrimSpace(accessToken); accessToken == "" {
		return nil, errors.New("Xero access token is missing")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.connectionsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create Xero connections request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform Xero connections request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPError{Operation: "connections", StatusCode: response.StatusCode}
	}
	var connections []Connection
	if err := decodeJSON(response.Body, &connections); err != nil {
		return nil, fmt.Errorf("decode Xero connections response: %w", err)
	}
	return connections, nil
}

func (c *Client) exchangeTokens(ctx context.Context, operation string, values url.Values) (Tokens, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("create Xero %s request: %w", operation, err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.SetBasicAuth(c.config.ClientID, "")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return Tokens{}, fmt.Errorf("perform Xero %s request: %w", operation, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Tokens{}, &HTTPError{Operation: operation, StatusCode: response.StatusCode}
	}
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    any    `json:"expires_in"`
	}
	if err := decodeJSON(response.Body, &body); err != nil {
		return Tokens{}, fmt.Errorf("decode Xero %s response: %w", operation, err)
	}
	expiresIn, err := seconds(body.ExpiresIn)
	if err != nil || expiresIn <= 0 {
		return Tokens{}, fmt.Errorf("decode Xero %s response: invalid expires_in", operation)
	}
	if strings.TrimSpace(body.AccessToken) == "" || strings.TrimSpace(body.RefreshToken) == "" {
		return Tokens{}, fmt.Errorf("decode Xero %s response: token pair is incomplete", operation)
	}
	return Tokens{
		AccessToken: body.AccessToken, RefreshToken: body.RefreshToken,
		Expiry: c.now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func randomString(source io.Reader, size int) (string, error) {
	data := make([]byte, size)
	if _, err := io.ReadFull(source, data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeJSON(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, maximumResponseBytes))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not supported")
		}
		return err
	}
	return nil
}

func seconds(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported expires_in value %T", value)
	}
}
