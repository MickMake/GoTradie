package app

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
	goxero "github.com/MickMake/GoXero"
)

func TestXeroCredentialStorageIsAtomicAndRestrictive(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".GoTradie", "keys", "xero.json")
	credentials := testXeroCredentials(time.Now().Add(time.Hour))
	if err := saveXeroCredentials(path, credentials); err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)

	credentials.AccessToken = "rotated-access"
	credentials.RefreshToken = "rotated-refresh"
	if err := saveXeroCredentials(path, credentials); err != nil {
		t.Fatal(err)
	}
	loaded, found, err := loadXeroCredentials(path)
	if err != nil || !found {
		t.Fatalf("load credentials: found=%t err=%v", found, err)
	}
	if loaded.AccessToken != "rotated-access" || loaded.RefreshToken != "rotated-refresh" {
		t.Fatalf("loaded credentials = %#v", loaded)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "xero.json" {
		t.Fatalf("credential directory entries = %v", entries)
	}

	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadXeroCredentials(path); err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)
}

func TestAuthenticateXeroReusesSavedConnectionWithoutBrowser(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".GoTradie", "keys", "xero.json")
	if err := saveXeroCredentials(path, testXeroCredentials(time.Now().Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	browserCalled := false
	client := &http.Client{Transport: appRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://api.xero.com/connections" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer saved-access" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		return appJSONResponse(http.StatusOK, `[{"tenantId":"tenant-1","tenantName":"Acme Tradies","tenantType":"ORGANISATION"}]`), nil
	})}
	var output, diagnostics bytes.Buffer
	application := App{
		In: strings.NewReader(""), Out: &output, Err: &diagnostics,
		xeroRuntime: &xeroAuthRuntime{
			httpClient: client,
			openURL:    func(string) error { browserCalled = true; return nil },
			homeDir:    func() (string, error) { return home, nil },
			now:        time.Now,
		},
	}
	if err := application.authenticateXero(context.Background(), testXeroConfig("http://localhost:8080/callback", false)); err != nil {
		t.Fatal(err)
	}
	if browserCalled {
		t.Fatal("browser opened despite usable saved credentials")
	}
	if output.String() != "Xero authentication: OK\nOrganisation: Acme Tradies\nConnection: OK\n" {
		t.Fatalf("output = %q", output.String())
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}

func TestAuthenticateXeroRefreshesAndPersistsRotatedTokens(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".GoTradie", "keys", "xero.json")
	if err := saveXeroCredentials(path, testXeroCredentials(time.Now().Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: appRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://identity.xero.com/connect/token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("refresh_token") != "saved-refresh" {
				t.Fatalf("refresh token = %q", request.Form.Get("refresh_token"))
			}
			return appJSONResponse(http.StatusOK, `{"access_token":"rotated-access","refresh_token":"rotated-refresh","expires_in":1800}`), nil
		case "https://api.xero.com/connections":
			if request.Header.Get("Authorization") != "Bearer rotated-access" {
				t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
			}
			return appJSONResponse(http.StatusOK, `[{"tenantId":"tenant-1","tenantName":"Acme Tradies","tenantType":"ORGANISATION"}]`), nil
		default:
			t.Fatalf("unexpected request: %s", request.URL)
			return nil, nil
		}
	})}
	var output, diagnostics bytes.Buffer
	application := App{
		In: strings.NewReader(""), Out: &output, Err: &diagnostics,
		xeroRuntime: &xeroAuthRuntime{
			httpClient: client,
			openURL:    func(string) error { t.Fatal("browser should not open"); return nil },
			homeDir:    func() (string, error) { return home, nil },
			now:        time.Now,
		},
	}
	if err := application.authenticateXero(context.Background(), testXeroConfig("http://localhost:8080/callback", true)); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := loadXeroCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "rotated-access" || loaded.RefreshToken != "rotated-refresh" {
		t.Fatalf("persisted credentials = %#v", loaded)
	}
	combined := output.String() + diagnostics.String()
	for _, secret := range []string{"saved-refresh", "rotated-access", "rotated-refresh"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("output exposed %q: %q", secret, combined)
		}
	}
	if !strings.Contains(diagnostics.String(), "refreshing expired Xero access token") ||
		!strings.Contains(diagnostics.String(), "saved rotated Xero token pair") {
		t.Fatalf("diagnostics = %q", diagnostics.String())
	}
}

func TestRejectedRefreshRequiresFreshAuthorization(t *testing.T) {
	httpClient := &http.Client{Transport: appRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://identity.xero.com/connect/token" {
			t.Fatalf("unexpected request: %s", request.URL)
		}
		return appJSONResponse(http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
	})}
	client, err := goxero.NewClient(goxero.Config{
		ClientID: "client-id", RedirectURI: "http://localhost:8080/callback",
		Scopes: []string{"offline_access", "accounting.invoices"},
	}, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	var output, diagnostics bytes.Buffer
	application := App{In: strings.NewReader(""), Out: &output, Err: &diagnostics}
	connected, err := application.useSavedXeroCredentials(
		context.Background(), client, filepath.Join(t.TempDir(), "xero.json"),
		testXeroCredentials(time.Now().Add(-time.Hour)), true,
		xeroAuthRuntime{now: time.Now},
	)
	if err != nil || connected {
		t.Fatalf("connected=%t err=%v; want reauthorization", connected, err)
	}
	if !strings.Contains(output.String(), "starting a new authorization") {
		t.Fatalf("output = %q diagnostics = %q", output.String(), diagnostics.String())
	}
}

func TestAuthenticateXeroFirstRunCompletesCallbackAndSelectsOnlyOrganisation(t *testing.T) {
	home := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirectURI := "http://" + listener.Addr().String() + "/callback"
	client := &http.Client{Transport: appRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "https://identity.xero.com/connect/token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("code") != "authorization-code" || request.Form.Get("code_verifier") == "" {
				t.Fatalf("token form = %v", request.Form)
			}
			return appJSONResponse(http.StatusOK, `{"access_token":"first-access","refresh_token":"first-refresh","expires_in":1800}`), nil
		case "https://api.xero.com/connections":
			return appJSONResponse(http.StatusOK, `[{"tenantId":"tenant-1","tenantName":"Acme Tradies","tenantType":"ORGANISATION"}]`), nil
		default:
			t.Fatalf("unexpected request: %s", request.URL)
			return nil, nil
		}
	})}
	listenCalled := false
	var output, diagnostics bytes.Buffer
	application := App{
		In: strings.NewReader(""), Out: &output, Err: &diagnostics,
		xeroRuntime: &xeroAuthRuntime{
			httpClient: client,
			openURL: func(rawAuthorizationURL string) error {
				authorizationURL, err := url.Parse(rawAuthorizationURL)
				if err != nil {
					t.Fatal(err)
				}
				state := authorizationURL.Query().Get("state")
				if state == "" || strings.Contains(rawAuthorizationURL, "code_verifier") {
					t.Fatalf("authorization URL = %s", rawAuthorizationURL)
				}
				go func() {
					callbackURL := redirectURI + "?code=authorization-code&state=" + url.QueryEscape(state)
					response, callbackErr := http.Get(callbackURL)
					if callbackErr == nil {
						_ = response.Body.Close()
					}
				}()
				return nil
			},
			homeDir: func() (string, error) { return home, nil },
			listen: func(network, address string) (net.Listener, error) {
				listenCalled = true
				if network != "tcp" || address != listener.Addr().String() {
					t.Fatalf("listen = %s %s", network, address)
				}
				return listener, nil
			},
			timeout: 5 * time.Second,
			now:     time.Now,
		},
	}
	if err := application.authenticateXero(context.Background(), testXeroConfig(redirectURI, true)); err != nil {
		t.Fatal(err)
	}
	if !listenCalled {
		t.Fatal("callback listener was not started")
	}
	path := filepath.Join(home, ".GoTradie", "keys", "xero.json")
	loaded, found, err := loadXeroCredentials(path)
	if err != nil || !found {
		t.Fatalf("load credentials: found=%t err=%v", found, err)
	}
	if loaded.AccessToken != "first-access" || loaded.RefreshToken != "first-refresh" || loaded.TenantID != "tenant-1" {
		t.Fatalf("credentials = %#v", loaded)
	}
	assertMode(t, filepath.Dir(path), 0o700)
	assertMode(t, path, 0o600)
	combined := output.String() + diagnostics.String()
	for _, secret := range []string{"authorization-code", "first-access", "first-refresh"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("output exposed %q: %q", secret, combined)
		}
	}
}

func TestInteractiveXeroAuthorizationTimesOutWithoutCallback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	redirectURI := "http://" + listener.Addr().String() + "/callback"
	client, err := goxero.NewClient(goxero.Config{
		ClientID: "client-id", RedirectURI: redirectURI,
		Scopes: []string{"offline_access", "accounting.invoices"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	application := App{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard}
	err = application.authorizeXeroInteractively(
		context.Background(), client, redirectURI, "client-id", filepath.Join(t.TempDir(), "xero.json"), false,
		xeroAuthRuntime{
			openURL: func(string) error { return nil },
			listen:  func(string, string) (net.Listener, error) { return listener, nil },
			timeout: 10 * time.Millisecond,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for Xero callback") {
		t.Fatalf("error = %v", err)
	}
}

func TestSelectXeroConnectionPromptsForMultipleOrganisations(t *testing.T) {
	var output bytes.Buffer
	application := App{In: strings.NewReader("2\n"), Out: &output}
	connection, err := application.selectXeroConnection([]goxero.Connection{
		{TenantID: "practice", TenantName: "Practice", TenantType: "PRACTICE"},
		{TenantID: "tenant-1", TenantName: "First", TenantType: "ORGANISATION"},
		{TenantID: "tenant-2", TenantName: "Second", TenantType: "ORGANISATION"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if connection.TenantID != "tenant-2" {
		t.Fatalf("connection = %#v", connection)
	}
	if !strings.Contains(output.String(), "1. First") || !strings.Contains(output.String(), "2. Second") || strings.Contains(output.String(), "Practice") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestSelectXeroConnectionRejectsNoOrganisation(t *testing.T) {
	application := App{In: strings.NewReader(""), Out: io.Discard}
	_, err := application.selectXeroConnection([]goxero.Connection{
		{TenantID: "practice", TenantName: "Practice", TenantType: "PRACTICE"},
		{TenantName: "Missing ID", TenantType: "ORGANISATION"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "no connected Xero organisations") {
		t.Fatalf("error = %v", err)
	}
}

func TestMissingSavedOrganisationRequiresExplicitReplacement(t *testing.T) {
	var output bytes.Buffer
	application := App{In: strings.NewReader("1\n"), Out: &output}
	connection, err := application.selectXeroConnection([]goxero.Connection{
		{TenantID: "replacement", TenantName: "Replacement Co", TenantType: "ORGANISATION"},
	}, "missing-saved-tenant")
	if err != nil {
		t.Fatal(err)
	}
	if connection.TenantID != "replacement" || !strings.Contains(output.String(), "saved Xero organisation is no longer connected") || !strings.Contains(output.String(), "Selection:") {
		t.Fatalf("connection=%#v output=%q", connection, output.String())
	}
}

func testXeroConfig(redirectURI string, verbose bool) config.Config {
	return config.Config{
		Verbose: verbose,
		Providers: map[string]config.ProviderConfig{
			"xero": {Auth: &config.XeroAuthConfig{
				ClientID: "client-id", RedirectURI: redirectURI,
				Scopes: []string{"offline_access", "accounting.invoices"},
			}},
		},
	}
}

func testXeroCredentials(expiry time.Time) xeroCredentialFile {
	return xeroCredentialFile{
		ClientID: "client-id", AccessToken: "saved-access", RefreshToken: "saved-refresh", Expiry: expiry,
		TenantID: "tenant-1", TenantName: "Acme Tradies", TenantType: "ORGANISATION",
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o; want %#o", path, got, want)
	}
}

type appRoundTripFunc func(*http.Request) (*http.Response, error)

func (function appRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func appJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestRunAuthRejectsFlagsAndOtherProviders(t *testing.T) {
	for _, args := range [][]string{{}, {"other"}, {"xero", "--commit"}, {"xero", "--force"}} {
		var output bytes.Buffer
		application := App{In: strings.NewReader(""), Out: &output, Err: &output}
		if code := application.runAuth(context.Background(), config.Config{}, args); code != 2 {
			t.Fatalf("args=%v exit=%d; want 2", args, code)
		}
		if got := output.String(); got != "usage: GoTradie auth xero\n" {
			t.Fatalf("args=%v output=%q", args, got)
		}
	}
}

func TestLoadXeroCredentialsRejectsCorruptOrNonRegularFiles(t *testing.T) {
	directory := t.TempDir()
	corrupt := filepath.Join(directory, "corrupt.json")
	if err := os.WriteFile(corrupt, []byte(`{"access_token":"secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadXeroCredentials(corrupt); err == nil || !strings.Contains(err.Error(), "client_id is missing") {
		t.Fatalf("corrupt credentials error = %v", err)
	}
	nonRegular := filepath.Join(directory, "not-a-file")
	if err := os.Mkdir(nonRegular, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadXeroCredentials(nonRegular); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("non-regular credentials error = %v", err)
	}
}
