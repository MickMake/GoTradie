package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/config"
	goxero "github.com/MickMake/GoXero"
)

const (
	xeroCredentialDirectory  = "keys"
	xeroCredentialFilename   = "xero.json"
	xeroCallbackTimeout      = 3 * time.Minute
	xeroBrowserLaunchTimeout = 5 * time.Second
	xeroExpirySkew           = time.Minute
)

type xeroAuthRuntime struct {
	httpClient *http.Client
	openURL    func(string) error
	homeDir    func() (string, error)
	listen     func(string, string) (net.Listener, error)
	timeout    time.Duration
	now        func() time.Time
}

type xeroCredentialFile struct {
	ClientID     string    `json:"client_id"`
	Scopes       []string  `json:"scopes,omitempty"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	TenantID     string    `json:"tenant_id,omitempty"`
	TenantName   string    `json:"tenant_name,omitempty"`
	TenantType   string    `json:"tenant_type,omitempty"`
}

type xeroCallback struct {
	code       string
	state      string
	oauthError string
}

func (a App) runAuth(ctx context.Context, cfg config.Config, args []string) int {
	if len(args) != 1 || args[0] != "xero" {
		fmt.Fprintln(a.Err, "usage: GoTradie auth xero")
		return 2
	}
	if err := cfg.ValidateXero(); err != nil {
		fmt.Fprintln(a.Err, "config error:", err)
		return 2
	}
	if err := a.authenticateXero(ctx, cfg); err != nil {
		fmt.Fprintln(a.Err, "xero auth error:", err)
		return 1
	}
	return 0
}

func (a App) authenticateXero(ctx context.Context, cfg config.Config) error {
	provider := cfg.Providers["xero"]
	auth := provider.Auth
	runtime := a.resolvedXeroRuntime()
	client, err := goxero.NewClient(goxero.Config{
		ClientID: auth.ClientID, RedirectURI: auth.RedirectURI, Scopes: append([]string(nil), auth.Scopes...),
	}, runtime.httpClient)
	if err != nil {
		return err
	}
	credentialPath, err := xeroCredentialPath(runtime.homeDir)
	if err != nil {
		return err
	}
	credentials, found, err := loadXeroCredentials(credentialPath)
	if err != nil {
		return err
	}
	clientMatches := credentials.ClientID == strings.TrimSpace(auth.ClientID)
	scopesMatch := xeroScopeSetsEqual(credentials.Scopes, auth.Scopes)
	if found && clientMatches && scopesMatch {
		connected, err := a.useSavedXeroCredentials(ctx, client, credentialPath, credentials, cfg.Verbose, runtime)
		if err != nil {
			return err
		}
		if connected {
			return nil
		}
	} else if found && !clientMatches {
		fmt.Fprintln(a.Out, "Saved Xero credentials belong to a different client; starting a new authorization.")
	} else if found {
		fmt.Fprintln(a.Out, "Saved Xero authorization scopes changed; starting a new authorization.")
	}
	return a.authorizeXeroInteractively(ctx, client, auth.RedirectURI, auth.ClientID, auth.Scopes, credentialPath, cfg.Verbose, runtime)
}

func (a App) useSavedXeroCredentials(
	ctx context.Context,
	client *goxero.Client,
	credentialPath string,
	credentials xeroCredentialFile,
	verbose bool,
	runtime xeroAuthRuntime,
) (bool, error) {
	tokens := credentials.tokens()
	refreshed := false
	if !tokens.Expiry.After(runtime.now().Add(xeroExpirySkew)) {
		xeroDiagnostic(a.Err, verbose, "refreshing expired Xero access token")
		var err error
		tokens, err = client.Refresh(ctx, tokens.RefreshToken)
		if err != nil {
			if xeroReauthorizationRequired(err) {
				fmt.Fprintln(a.Out, "Saved Xero authorization is no longer valid; starting a new authorization.")
				return false, nil
			}
			return false, err
		}
		refreshed = true
		credentials.setTokens(tokens)
		if err := saveXeroCredentials(credentialPath, credentials); err != nil {
			return false, err
		}
		xeroDiagnostic(a.Err, verbose, "saved rotated Xero token pair")
	}

	connections, err := client.Connections(ctx, tokens.AccessToken)
	if err != nil && !refreshed && xeroReauthorizationRequired(err) {
		xeroDiagnostic(a.Err, verbose, "refreshing Xero credentials after a rejected connection check")
		tokens, err = client.Refresh(ctx, tokens.RefreshToken)
		if err == nil {
			credentials.setTokens(tokens)
			if err := saveXeroCredentials(credentialPath, credentials); err != nil {
				return false, err
			}
			connections, err = client.Connections(ctx, tokens.AccessToken)
		}
	}
	if err != nil {
		if xeroReauthorizationRequired(err) {
			fmt.Fprintln(a.Out, "Saved Xero authorization is no longer valid; starting a new authorization.")
			return false, nil
		}
		return false, err
	}
	connection, err := a.selectXeroConnection(connections, credentials.TenantID)
	if err != nil {
		return false, err
	}
	if credentials.TenantID != connection.TenantID || credentials.TenantName != connection.TenantName || credentials.TenantType != connection.TenantType {
		credentials.setConnection(connection)
		if err := saveXeroCredentials(credentialPath, credentials); err != nil {
			return false, err
		}
	}
	printXeroSuccess(a.Out, connection)
	return true, nil
}

func (a App) authorizeXeroInteractively(
	ctx context.Context,
	client *goxero.Client,
	redirectURI string,
	clientID string,
	scopes []string,
	credentialPath string,
	verbose bool,
	runtime xeroAuthRuntime,
) error {
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("parse Xero callback URL: %w", err)
	}
	listener, err := runtime.listen("tcp", redirect.Host)
	if err != nil {
		return fmt.Errorf("listen for Xero callback on %s: %w", redirect.Host, err)
	}
	defer listener.Close()

	session, authorizationURL, err := client.BeginAuth()
	if err != nil {
		return err
	}
	callbackChannel := make(chan xeroCallback, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(redirect.Path, func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		callback := xeroCallback{
			code: strings.TrimSpace(request.URL.Query().Get("code")), state: request.URL.Query().Get("state"),
			oauthError: strings.TrimSpace(request.URL.Query().Get("error")),
		}
		if err := client.ValidateCallback(session, callback.state); err != nil {
			http.Error(writer, "invalid Xero authorization callback", http.StatusBadRequest)
			return
		}
		select {
		case callbackChannel <- callback:
			writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintln(writer, "Xero authorization received. You may return to GoTradie.")
		default:
			http.Error(writer, "authorization callback already received", http.StatusConflict)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serverErrors := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrors <- serveErr
		}
	}()
	xeroDiagnostic(a.Err, verbose, "Xero callback listener started")
	launchXeroAuthorization(a.Out, runtime.openURL, authorizationURL)

	timer := time.NewTimer(runtime.timeout)
	defer timer.Stop()
	var callback xeroCallback
	select {
	case callback = <-callbackChannel:
	case serveErr := <-serverErrors:
		return fmt.Errorf("serve Xero callback: %w", serveErr)
	case <-timer.C:
		return errors.New("timed out waiting for Xero callback")
	case <-ctx.Done():
		return ctx.Err()
	}
	shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_ = server.Shutdown(shutdownContext)
	cancel()
	xeroDiagnostic(a.Err, verbose, "Xero callback received")
	if callback.oauthError != "" {
		if err := client.ValidateCallback(session, callback.state); err != nil {
			return err
		}
		return errors.New("Xero authorization was declined or failed")
	}
	tokens, err := client.CompleteAuth(ctx, session, callback.code, callback.state)
	if err != nil {
		return err
	}
	credentials := xeroCredentialFile{
		ClientID: strings.TrimSpace(clientID),
		Scopes:   normalizedXeroScopes(scopes),
	}
	credentials.setTokens(tokens)
	if err := saveXeroCredentials(credentialPath, credentials); err != nil {
		return err
	}
	xeroDiagnostic(a.Err, verbose, "saved Xero credentials")
	connections, err := client.Connections(ctx, tokens.AccessToken)
	if err != nil {
		return err
	}
	connection, err := a.selectXeroConnection(connections, "")
	if err != nil {
		return err
	}
	credentials.setConnection(connection)
	if err := saveXeroCredentials(credentialPath, credentials); err != nil {
		return err
	}
	printXeroSuccess(a.Out, connection)
	return nil
}

func (a App) selectXeroConnection(connections []goxero.Connection, preferredTenantID string) (goxero.Connection, error) {
	organizations := make([]goxero.Connection, 0, len(connections))
	for _, connection := range connections {
		if strings.EqualFold(strings.TrimSpace(connection.TenantType), "ORGANISATION") && strings.TrimSpace(connection.TenantID) != "" {
			organizations = append(organizations, connection)
		}
	}
	if len(organizations) == 0 {
		return goxero.Connection{}, errors.New("no connected Xero organisations were returned")
	}
	for _, connection := range organizations {
		if preferredTenantID != "" && connection.TenantID == preferredTenantID {
			return connection, nil
		}
	}
	if len(organizations) == 1 && preferredTenantID == "" {
		return organizations[0], nil
	}
	if preferredTenantID != "" {
		fmt.Fprintln(a.Out, "The saved Xero organisation is no longer connected. Choose another organisation:")
	} else {
		fmt.Fprintln(a.Out, "Choose a Xero organisation:")
	}
	for index, connection := range organizations {
		fmt.Fprintf(a.Out, "  %d. %s\n", index+1, xeroConnectionName(connection))
	}
	fmt.Fprint(a.Out, "Selection: ")
	scanner := bufio.NewScanner(a.In)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return goxero.Connection{}, fmt.Errorf("read Xero organisation selection: %w", err)
		}
		return goxero.Connection{}, errors.New("Xero organisation selection is required")
	}
	selection, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || selection < 1 || selection > len(organizations) {
		return goxero.Connection{}, fmt.Errorf("Xero organisation selection must be between 1 and %d", len(organizations))
	}
	return organizations[selection-1], nil
}

func (a App) resolvedXeroRuntime() xeroAuthRuntime {
	resolved := xeroAuthRuntime{}
	if a.xeroRuntime != nil {
		resolved = *a.xeroRuntime
	}
	if resolved.httpClient == nil {
		resolved.httpClient = http.DefaultClient
	}
	if resolved.openURL == nil {
		resolved.openURL = openBrowser
	}
	if resolved.homeDir == nil {
		resolved.homeDir = os.UserHomeDir
	}
	if resolved.listen == nil {
		resolved.listen = net.Listen
	}
	if resolved.timeout <= 0 {
		resolved.timeout = xeroCallbackTimeout
	}
	if resolved.now == nil {
		resolved.now = time.Now
	}
	return resolved
}

func openBrowser(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), xeroBrowserLaunchTimeout)
	defer cancel()

	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", target)
	case "linux":
		command = exec.CommandContext(ctx, "xdg-open", target)
	case "windows":
		command = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target)
	default:
		return fmt.Errorf("opening a browser is unsupported on %s", runtime.GOOS)
	}
	return runBrowserCommand(command)
}

func runBrowserCommand(command *exec.Cmd) error {
	if err := command.Run(); err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	return nil
}

func launchXeroAuthorization(writer io.Writer, openURL func(string) error, authorizationURL string) {
	if err := openURL(authorizationURL); err != nil {
		fmt.Fprintln(writer, "Open this URL in a browser on this computer:")
		fmt.Fprintln(writer, authorizationURL)
		return
	}
	fmt.Fprintln(writer, "Waiting for Xero authorization in your browser...")
}

func xeroCredentialPath(homeDir func() (string, error)) (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("resolve home directory: empty path")
	}
	return filepath.Join(home, ".GoTradie", xeroCredentialDirectory, xeroCredentialFilename), nil
}

func loadXeroCredentials(path string) (xeroCredentialFile, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return xeroCredentialFile{}, false, nil
	}
	if err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("inspect Xero credentials: %w", err)
	}
	if !info.Mode().IsRegular() {
		return xeroCredentialFile{}, false, errors.New("Xero credential path is not a regular file")
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("secure Xero credential directory: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("secure Xero credential file: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("read Xero credentials: %w", err)
	}
	var credentials xeroCredentialFile
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&credentials); err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("decode Xero credentials: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return xeroCredentialFile{}, false, fmt.Errorf("decode Xero credentials: %w", err)
	} else if err == nil {
		return xeroCredentialFile{}, false, errors.New("decode Xero credentials: multiple JSON values are not supported")
	}
	if err := credentials.validate(); err != nil {
		return xeroCredentialFile{}, false, fmt.Errorf("decode Xero credentials: %w", err)
	}
	return credentials, true, nil
}

func saveXeroCredentials(path string, credentials xeroCredentialFile) error {
	if err := credentials.validate(); err != nil {
		return fmt.Errorf("save Xero credentials: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create Xero credential directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("secure Xero credential directory: %w", err)
	}
	data, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Xero credentials: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, ".xero-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Xero credential file: %w", err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary Xero credential file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary Xero credential file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary Xero credential file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary Xero credential file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace Xero credential file: %w", err)
	}
	removeTemporary = false
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure Xero credential file: %w", err)
	}
	directoryFile, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open Xero credential directory for sync: %w", err)
	}
	if syncErr := directoryFile.Sync(); syncErr != nil {
		_ = directoryFile.Close()
		return fmt.Errorf("sync Xero credential directory: %w", syncErr)
	}
	if closeErr := directoryFile.Close(); closeErr != nil {
		return fmt.Errorf("close Xero credential directory: %w", closeErr)
	}
	return nil
}

func (credentials xeroCredentialFile) validate() error {
	if strings.TrimSpace(credentials.ClientID) == "" {
		return errors.New("client_id is missing")
	}
	if strings.TrimSpace(credentials.AccessToken) == "" || strings.TrimSpace(credentials.RefreshToken) == "" {
		return errors.New("token pair is incomplete")
	}
	if credentials.Expiry.IsZero() {
		return errors.New("expiry is missing")
	}
	return nil
}

func (credentials xeroCredentialFile) tokens() goxero.Tokens {
	return goxero.Tokens{
		AccessToken: credentials.AccessToken, RefreshToken: credentials.RefreshToken, Expiry: credentials.Expiry,
	}
}

func (credentials *xeroCredentialFile) setTokens(tokens goxero.Tokens) {
	credentials.AccessToken = tokens.AccessToken
	credentials.RefreshToken = tokens.RefreshToken
	credentials.Expiry = tokens.Expiry
}

func (credentials *xeroCredentialFile) setConnection(connection goxero.Connection) {
	credentials.TenantID = connection.TenantID
	credentials.TenantName = connection.TenantName
	credentials.TenantType = connection.TenantType
}

func normalizedXeroScopes(scopes []string) []string {
	normalized := make([]string, len(scopes))
	for index, scope := range scopes {
		normalized[index] = strings.TrimSpace(scope)
	}
	return normalized
}

func xeroScopeSetsEqual(stored, configured []string) bool {
	if len(stored) == 0 || len(stored) != len(configured) {
		return false
	}
	storedSet := make(map[string]struct{}, len(stored))
	for _, scope := range stored {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return false
		}
		if _, duplicate := storedSet[scope]; duplicate {
			return false
		}
		storedSet[scope] = struct{}{}
	}
	for _, scope := range configured {
		if _, exists := storedSet[strings.TrimSpace(scope)]; !exists {
			return false
		}
	}
	return true
}

func xeroReauthorizationRequired(err error) bool {
	var httpError *goxero.HTTPError
	return errors.As(err, &httpError) &&
		(httpError.StatusCode == http.StatusBadRequest || httpError.StatusCode == http.StatusUnauthorized || httpError.StatusCode == http.StatusForbidden)
}

func xeroDiagnostic(writer io.Writer, verbose bool, message string) {
	if verbose {
		fmt.Fprintln(writer, "xero:", message)
	}
}

func printXeroSuccess(writer io.Writer, connection goxero.Connection) {
	fmt.Fprintln(writer, "Xero authentication: OK")
	fmt.Fprintln(writer, "Organisation:", xeroConnectionName(connection))
	fmt.Fprintln(writer, "Connection: OK")
}

func xeroConnectionName(connection goxero.Connection) string {
	if name := strings.TrimSpace(connection.TenantName); name != "" {
		return name
	}
	return connection.TenantID
}
