package goxero

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewClientValidatesConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{"client ID", Config{RedirectURI: "http://localhost:8080/callback", Scopes: []string{"offline_access"}}, "client ID"},
		{"redirect", Config{ClientID: "client", RedirectURI: "/callback", Scopes: []string{"offline_access"}}, "redirect URI"},
		{"scopes", Config{ClientID: "client", RedirectURI: "http://localhost:8080/callback"}, "scope"},
		{"blank scope", Config{ClientID: "client", RedirectURI: "http://localhost:8080/callback", Scopes: []string{" "}}, "blank"},
		{"duplicate scope", Config{ClientID: "client", RedirectURI: "http://localhost:8080/callback", Scopes: []string{"offline_access", "offline_access"}}, "duplicated"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClient(test.cfg, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v; want %q", err, test.want)
			}
		})
	}
}

func TestBeginAuthUsesFreshPKCES256AndSeparateState(t *testing.T) {
	client := testClient(t, nil)
	randomData := append(bytes.Repeat([]byte{0x42}, 96), bytes.Repeat([]byte{0x43}, 96)...)
	client.random = bytes.NewReader(randomData)

	session, rawURL, err := client.BeginAuth()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != "client-id" ||
		query.Get("redirect_uri") != "http://localhost:8080/callback" ||
		query.Get("scope") != "offline_access accounting.invoices" ||
		query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization query = %v", query)
	}
	digest := sha256.Sum256([]byte(session.verifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(digest[:])
	if query.Get("code_challenge") != wantChallenge || query.Get("state") != session.state {
		t.Fatalf("PKCE/state query = %v", query)
	}
	if strings.Contains(rawURL, session.verifier) || session.verifier == session.state {
		t.Fatal("authorization URL exposed the verifier or reused it as state")
	}

	next, _, err := client.BeginAuth()
	if err != nil {
		t.Fatal(err)
	}
	if next.verifier == session.verifier || next.state == session.state {
		t.Fatal("subsequent authentication reused verifier or state")
	}
}

func TestCompleteAuthRejectsStateBeforeExchange(t *testing.T) {
	called := false
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, io.EOF
	}))
	session := &AuthSession{verifier: "verifier", state: "expected"}
	_, err := client.CompleteAuth(context.Background(), session, "code", "wrong")
	if err == nil || !strings.Contains(err.Error(), "state did not match") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("token endpoint called before state validation")
	}
}

func TestValidateCallbackRejectsMissingSessionAndState(t *testing.T) {
	client := testClient(t, nil)
	if err := client.ValidateCallback(nil, "state"); err == nil || !strings.Contains(err.Error(), "session is missing") {
		t.Fatalf("missing session error = %v", err)
	}
	if err := client.ValidateCallback(&AuthSession{verifier: "verifier", state: "expected"}, ""); err == nil || !strings.Contains(err.Error(), "state did not match") {
		t.Fatalf("missing state error = %v", err)
	}
}

func TestCompleteAuthExchangesCodeWithoutLeakingSecrets(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	client := testClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != defaultTokenEndpoint || request.Method != http.MethodPost {
			t.Fatalf("request = %s %s", request.Method, request.URL)
		}
		username, password, ok := request.BasicAuth()
		if !ok || username != "client-id" || password != "" {
			t.Fatalf("basic auth = %q %q %t", username, password, ok)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "secret-code" ||
			request.Form.Get("code_verifier") != "secret-verifier" || request.Form.Get("client_id") != "client-id" {
			t.Fatalf("form = %v", request.Form)
		}
		return jsonResponse(http.StatusOK, `{"access_token":"access","refresh_token":"refresh","expires_in":1800}`), nil
	}))
	client.now = func() time.Time { return now }

	tokens, err := client.CompleteAuth(context.Background(), &AuthSession{verifier: "secret-verifier", state: "secret-state"}, "secret-code", "secret-state")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "access" || tokens.RefreshToken != "refresh" || !tokens.Expiry.Equal(now.Add(30*time.Minute)) {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestRefreshReturnsRotatedTokenPair(t *testing.T) {
	client := testClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("grant_type") != "refresh_token" || request.Form.Get("refresh_token") != "old-refresh" {
			t.Fatalf("form = %v", request.Form)
		}
		return jsonResponse(http.StatusOK, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":"1800"}`), nil
	}))
	tokens, err := client.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "new-access" || tokens.RefreshToken != "new-refresh" {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestConnectionsIsReadOnlyAndDecodesTenants(t *testing.T) {
	client := testClient(t, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != defaultConnectionsEndpoint {
			t.Fatalf("request = %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("xero-tenant-id") != "" {
			t.Fatalf("headers = %v", request.Header)
		}
		return jsonResponse(http.StatusOK, `[{"tenantId":"tenant-1","tenantName":"Acme","tenantType":"ORGANISATION"}]`), nil
	}))
	connections, err := client.Connections(context.Background(), "access-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].TenantID != "tenant-1" || connections[0].TenantName != "Acme" {
		t.Fatalf("connections = %#v", connections)
	}
}

func TestHTTPFailuresDoNotExposeResponseBodies(t *testing.T) {
	client := testClient(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, `{"error":"invalid_grant","refresh_token":"do-not-print"}`), nil
	}))
	_, err := client.Refresh(context.Background(), "old-refresh")
	if err == nil || strings.Contains(err.Error(), "do-not-print") || err.Error() != "Xero token refresh request failed: HTTP 400" {
		t.Fatalf("error = %v", err)
	}
}

func TestAuthenticationAndConnectionsAgainstFakeXeroServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("token method = %s", request.Method)
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("code") != "fake-code" || request.Form.Get("code_verifier") != "fake-verifier" {
			t.Fatalf("token form = %v", request.Form)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"access_token":"fake-access","refresh_token":"fake-refresh","expires_in":1800}`)
	})
	mux.HandleFunc("/connections", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer fake-access" {
			t.Fatalf("connections request = %s auth=%q", request.Method, request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `[{"tenantId":"tenant-1","tenantName":"Fake Co","tenantType":"ORGANISATION"}]`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := testClient(t, server.Client().Transport)
	client.tokenEndpoint = server.URL + "/token"
	client.connectionsEndpoint = server.URL + "/connections"
	tokens, err := client.CompleteAuth(context.Background(), &AuthSession{verifier: "fake-verifier", state: "fake-state"}, "fake-code", "fake-state")
	if err != nil {
		t.Fatal(err)
	}
	connections, err := client.Connections(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].TenantName != "Fake Co" {
		t.Fatalf("connections = %#v", connections)
	}
}

func testClient(t *testing.T, transport http.RoundTripper) *Client {
	t.Helper()
	if transport == nil {
		transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, io.EOF })
	}
	client, err := NewClient(Config{
		ClientID: "client-id", RedirectURI: "http://localhost:8080/callback",
		Scopes: []string{"offline_access", "accounting.invoices"},
	}, &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestConnectionJSONContract(t *testing.T) {
	encoded, err := json.Marshal(Connection{TenantID: "id", TenantName: "name", TenantType: "ORGANISATION"})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"tenantId":"id","tenantName":"name","tenantType":"ORGANISATION"}` {
		t.Fatalf("JSON = %s", encoded)
	}
}
