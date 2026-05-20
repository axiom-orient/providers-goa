package client

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/axiom-orient/providers-goa/client/internal/authjson"
)

const defaultBrowserReloginPort = 1455

// BrowserReloginOptions configures interactive OAuth browser relogin.
type BrowserReloginOptions struct {
	NoBrowser          bool
	CallbackPort       int
	Timeout            time.Duration
	PersistPath        string
	Issuer             string
	ClientID           string
	AllowedWorkspaceID string
}

// DefaultBrowserReloginOptions returns Codex-compatible browser relogin defaults.
func DefaultBrowserReloginOptions() BrowserReloginOptions {
	return BrowserReloginOptions{
		CallbackPort: defaultBrowserReloginPort,
		Timeout:      180 * time.Second,
	}
}

// BrowserReloginOutcome summarizes a completed browser relogin.
type BrowserReloginOutcome struct {
	AuthURL      string    `json:"auth_url"`
	CallbackPort int       `json:"callback_port"`
	PersistedTo  string    `json:"persisted_to,omitempty"`
	AuthState    AuthState `json:"auth_state"`
}

// BrowserReloginSession exposes the authorize URL before waiting for completion.
type BrowserReloginSession struct {
	AuthURL      string
	CallbackPort int
	done         <-chan reloginResult
}

type reloginResult struct {
	outcome BrowserReloginOutcome
	err     error
}

type pkceCodes struct {
	verifier  string
	challenge string
}

type exchangedOAuthTokens struct {
	IDToken      string
	AccessToken  string
	RefreshToken string
	AccountID    string
}

// ReloginBrowser starts browser relogin and waits for completion.
func (c *Client) ReloginBrowser(ctx context.Context, opts BrowserReloginOptions) (BrowserReloginOutcome, error) {
	session, err := c.StartBrowserReloginSession(ctx, opts)
	if err != nil {
		return BrowserReloginOutcome{}, err
	}
	return session.Wait(ctx)
}

// StartBrowserReloginSession starts browser relogin and returns before the callback completes.
func (c *Client) StartBrowserReloginSession(ctx context.Context, opts BrowserReloginOptions) (*BrowserReloginSession, error) {
	if c == nil {
		return nil, ErrMissingCredential
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.ensureNoActiveSend(); err != nil {
		return nil, err
	}
	opts = normalizeBrowserReloginOptions(c.cfg, opts)
	if opts.CallbackPort < 0 || opts.CallbackPort > 65535 {
		return nil, &ValidationError{Field: "callback_port", Message: "must be between 0 and 65535"}
	}
	if opts.Timeout <= 0 {
		return nil, &ValidationError{Field: "timeout", Message: "must be greater than zero"}
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return nil, &ValidationError{Field: "client_id", Message: "must not be empty"}
	}
	if strings.TrimSpace(opts.Issuer) == "" {
		return nil, &ValidationError{Field: "issuer", Message: "must not be empty"}
	}
	if _, err := url.ParseRequestURI(opts.Issuer); err != nil {
		return nil, &ValidationError{Field: "issuer", Message: err.Error()}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.CallbackPort))
	if err != nil {
		return nil, fmt.Errorf("goa: browser relogin callback listen: %w", err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	pkce, err := generatePKCE()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	state, err := randomBase64URL(32)
	if err != nil {
		_ = listener.Close()
		return nil, err
	}
	redirectURI := fmt.Sprintf("http://localhost:%d/auth/callback", actualPort)
	authURL := buildBrowserAuthorizeURL(opts.Issuer, opts.ClientID, redirectURI, pkce, state, opts.AllowedWorkspaceID)

	done := make(chan reloginResult, 1)
	completed := make(chan struct{})
	var completeOnce sync.Once
	complete := func(result reloginResult) {
		completeOnce.Do(func() {
			done <- result
			close(done)
			close(completed)
		})
	}

	server := &http.Server{}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/callback" && r.URL.Path != "/cancel" {
			writeReloginHTML(w, http.StatusNotFound, "Not Found")
			return
		}
		result := c.handleBrowserReloginCallback(ctx, w, r, opts, pkce, state, redirectURI, authURL, actualPort)
		complete(result)
		go func() { _ = server.Shutdown(context.Background()) }()
	})

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			complete(reloginResult{err: fmt.Errorf("goa: browser relogin callback server: %w", err)})
		}
	}()
	go func() {
		timer := time.NewTimer(opts.Timeout)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			complete(reloginResult{err: ctx.Err()})
			_ = server.Shutdown(context.Background())
		case <-timer.C:
			complete(reloginResult{err: ErrReloginTimeout})
			_ = server.Shutdown(context.Background())
		case <-completed:
		}
	}()

	if !opts.NoBrowser {
		if err := openSystemBrowser(authURL); err != nil {
			_ = server.Shutdown(context.Background())
			return nil, err
		}
	}

	return &BrowserReloginSession{AuthURL: authURL, CallbackPort: actualPort, done: done}, nil
}

// Wait waits for a browser relogin session to complete.
func (s *BrowserReloginSession) Wait(ctx context.Context) (BrowserReloginOutcome, error) {
	if s == nil {
		return BrowserReloginOutcome{}, &ValidationError{Field: "session", Message: "must not be nil"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case result, ok := <-s.done:
		if !ok {
			return BrowserReloginOutcome{}, ErrReloginTimeout
		}
		return result.outcome, result.err
	case <-ctx.Done():
		return BrowserReloginOutcome{}, ctx.Err()
	}
}

func normalizeBrowserReloginOptions(cfg Config, opts BrowserReloginOptions) BrowserReloginOptions {
	cfg = cfg.normalized()
	if opts == (BrowserReloginOptions{}) {
		opts = DefaultBrowserReloginOptions()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 180 * time.Second
	}
	if strings.TrimSpace(opts.Issuer) == "" {
		opts.Issuer = cfg.AuthIssuerURL
	}
	opts.Issuer = strings.TrimRight(strings.TrimSpace(opts.Issuer), "/")
	if strings.TrimSpace(opts.ClientID) == "" {
		opts.ClientID = cfg.OAuthClientID
	}
	opts.ClientID = strings.TrimSpace(opts.ClientID)
	opts.AllowedWorkspaceID = strings.TrimSpace(opts.AllowedWorkspaceID)
	return opts
}

func (c *Client) handleBrowserReloginCallback(ctx context.Context, w http.ResponseWriter, r *http.Request, opts BrowserReloginOptions, pkce pkceCodes, state, redirectURI, authURL string, actualPort int) reloginResult {
	if r.Method != http.MethodGet {
		writeReloginHTML(w, http.StatusBadRequest, "Malformed HTTP request")
		return reloginResult{err: &ValidationError{Field: "callback", Message: "method must be GET"}}
	}
	switch r.URL.Path {
	case "/auth/callback":
		if r.URL.Query().Get("state") != state {
			writeReloginHTML(w, http.StatusBadRequest, "State mismatch")
			return reloginResult{err: fmt.Errorf("%w: state mismatch", ErrReloginDenied)}
		}
		if errorCode := r.URL.Query().Get("error"); errorCode != "" {
			message := oauthCallbackErrorMessage(errorCode, r.URL.Query().Get("error_description"))
			writeReloginHTML(w, http.StatusForbidden, message)
			return reloginResult{err: fmt.Errorf("%w: %s", ErrReloginDenied, message)}
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			writeReloginHTML(w, http.StatusBadRequest, "Missing authorization code in callback.")
			return reloginResult{err: &ValidationError{Field: "callback.code", Message: "missing authorization code"}}
		}

		tokens, err := c.exchangeCodeForTokens(ctx, opts.Issuer, opts.ClientID, redirectURI, pkce, code)
		if err != nil {
			writeReloginHTML(w, http.StatusBadGateway, "Sign-in failed while exchanging OAuth tokens. Return to the application for details.")
			return reloginResult{err: err}
		}
		if err := ensureWorkspaceAllowed(opts.AllowedWorkspaceID, tokens.IDToken); err != nil {
			writeReloginHTML(w, http.StatusForbidden, "The selected account is not allowed for this workspace restriction.")
			return reloginResult{err: err}
		}
		apiKey, err := c.obtainAPIKey(ctx, opts.Issuer, opts.ClientID, tokens.IDToken)
		if err != nil {
			writeReloginHTML(w, http.StatusBadGateway, apiKeyExchangeFailurePageMessage(err))
			return reloginResult{err: err}
		}
		outcome, err := c.finalizeBrowserRelogin(opts, authURL, actualPort, apiKey, tokens)
		if err != nil {
			writeReloginHTML(w, http.StatusBadGateway, "Sign-in succeeded but credential persistence failed. Return to the application for details.")
			return reloginResult{err: err}
		}
		writeReloginHTML(w, http.StatusOK, "Sign-in completed. You can return to the application.")
		return reloginResult{outcome: outcome}
	case "/cancel":
		writeReloginHTML(w, http.StatusOK, "Login cancelled")
		return reloginResult{err: fmt.Errorf("%w: login cancelled", ErrReloginDenied)}
	default:
		writeReloginHTML(w, http.StatusNotFound, "Not Found")
		return reloginResult{err: &ValidationError{Field: "callback.path", Message: "unknown path"}}
	}
}

func (c *Client) finalizeBrowserRelogin(opts BrowserReloginOptions, authURL string, actualPort int, apiKey string, tokens exchangedOAuthTokens) (BrowserReloginOutcome, error) {
	if c == nil {
		return BrowserReloginOutcome{}, ErrMissingCredential
	}
	persistedTo, err := c.persistBrowserReloginAuth(opts, apiKey, tokens)
	if err != nil {
		return BrowserReloginOutcome{}, err
	}

	c.authMu.Lock()
	defer c.authMu.Unlock()
	c.apiKey = apiKey
	c.accessToken = tokens.AccessToken
	c.refreshToken = tokens.RefreshToken
	c.idToken = tokens.IDToken
	c.accountID = tokens.AccountID
	c.transport = authTransportOpenAI
	c.authPath = persistedTo
	c.authState.AuthPath = persistedTo
	c.authState.AuthFileFound = true
	c.authState.AuthFileReadError = ""
	c.authState.AuthFileParseError = ""
	c.authState.AuthFileHasKnownAPIKey = true
	c.authState.AuthFileKnownAPIKeyRef = "OPENAI_API_KEY"
	c.authState.AuthMode = "chatgpt"
	c.authState.HasAccessToken = tokens.AccessToken != ""
	c.authState.HasRefreshToken = tokens.RefreshToken != ""
	c.authState.HasIDToken = tokens.IDToken != ""
	c.authState.HasAccountID = tokens.AccountID != ""
	c.authState.APIKeySource = APIKeySourceAuthFile
	c.authState.HasAPIKey = true
	c.authState.Transport = string(authTransportOpenAI)
	state := c.authState.clone()
	return BrowserReloginOutcome{AuthURL: authURL, CallbackPort: actualPort, PersistedTo: persistedTo, AuthState: state}, nil
}

func (c *Client) persistBrowserReloginAuth(opts BrowserReloginOptions, apiKey string, tokens exchangedOAuthTokens) (string, error) {
	data := authjson.ReloginData{
		OpenAIAPIKey: apiKey,
		IDToken:      tokens.IDToken,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		AccountID:    tokens.AccountID,
		RefreshedAt:  time.Now(),
	}

	target := strings.TrimSpace(opts.PersistPath)
	var encoded []byte
	var err error
	if target != "" {
		encoded, err = authjson.BuildReloginAuthFile(data)
	} else {
		state := c.AuthState()
		target = state.AuthPath
		if target == "" {
			return "", &ValidationError{Field: "auth_path", Message: "could not resolve auth path"}
		}
		if state.AuthFileParseError != "" || state.AuthFileReadError != "" {
			return "", &ValidationError{Field: "persist_path", Message: "current auth file is not safely rewriteable; pass BrowserReloginOptions.PersistPath"}
		}
		raw, readErr := os.ReadFile(target)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				encoded, err = authjson.BuildReloginAuthFile(data)
			} else {
				return "", readErr
			}
		} else {
			encoded, err = authjson.RewriteReloginAuthFile(raw, data)
		}
	}
	if err != nil {
		return "", err
	}
	if err := authjson.WritePrivateFile(target, encoded); err != nil {
		return "", err
	}
	return target, nil
}

func (c *Client) exchangeCodeForTokens(ctx context.Context, issuer, clientID, redirectURI string, pkce pkceCodes, code string) (exchangedOAuthTokens, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", pkce.verifier)
	raw, err := c.postTokenForm(ctx, issuer, form)
	if err != nil {
		return exchangedOAuthTokens{}, err
	}
	var payload struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return exchangedOAuthTokens{}, fmt.Errorf("goa: authorization-code exchange returned invalid JSON: %w", err)
	}
	payload.IDToken = strings.TrimSpace(payload.IDToken)
	payload.AccessToken = strings.TrimSpace(payload.AccessToken)
	payload.RefreshToken = strings.TrimSpace(payload.RefreshToken)
	if payload.IDToken == "" || payload.AccessToken == "" || payload.RefreshToken == "" {
		return exchangedOAuthTokens{}, &ValidationError{Field: "oauth.token", Message: "authorization-code exchange returned incomplete tokens"}
	}
	return exchangedOAuthTokens{
		IDToken:      payload.IDToken,
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		AccountID:    authjson.ChatGPTAccountIDFromJWT(payload.IDToken),
	}, nil
}

func (c *Client) obtainAPIKey(ctx context.Context, issuer, clientID, idToken string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("client_id", clientID)
	form.Set("requested_token", "openai-api-key")
	form.Set("subject_token", idToken)
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")
	raw, err := c.postTokenForm(ctx, issuer, form)
	if err != nil {
		return "", err
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("goa: API-key exchange returned invalid JSON: %w", err)
	}
	apiKey := strings.TrimSpace(payload.AccessToken)
	if apiKey == "" {
		return "", &ValidationError{Field: "oauth.api_key", Message: "API-key exchange returned empty access_token"}
	}
	return apiKey, nil
}

func (c *Client) postTokenForm(ctx context.Context, issuer string, form url.Values) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := strings.TrimRight(issuer, "/") + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("goa: token exchange: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("goa: token exchange read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{StatusCode: resp.StatusCode, RequestID: resp.Header.Get("x-request-id"), Body: summarizeBody(raw)}
	}
	return raw, nil
}

func buildBrowserAuthorizeURL(issuer, clientID, redirectURI string, pkce pkceCodes, state, allowedWorkspaceID string) string {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", clientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("scope", "openid profile email offline_access api.connectors.read api.connectors.invoke")
	values.Set("code_challenge", pkce.challenge)
	values.Set("code_challenge_method", "S256")
	values.Set("id_token_add_organizations", "true")
	values.Set("codex_cli_simplified_flow", "true")
	values.Set("state", state)
	values.Set("originator", "goa")
	if strings.TrimSpace(allowedWorkspaceID) != "" {
		values.Set("allowed_workspace_id", strings.TrimSpace(allowedWorkspaceID))
	}
	return strings.TrimRight(issuer, "/") + "/oauth/authorize?" + values.Encode()
}

func generatePKCE() (pkceCodes, error) {
	verifier, err := randomBase64URL(32)
	if err != nil {
		return pkceCodes{}, err
	}
	digest := sha256.Sum256([]byte(verifier))
	return pkceCodes{verifier: verifier, challenge: base64.RawURLEncoding.EncodeToString(digest[:])}, nil
}

func randomBase64URL(byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func ensureWorkspaceAllowed(expectedWorkspaceID, idToken string) error {
	expectedWorkspaceID = strings.TrimSpace(expectedWorkspaceID)
	if expectedWorkspaceID == "" {
		return nil
	}
	actual := authjson.ChatGPTAccountIDFromJWT(idToken)
	if actual == "" {
		return fmt.Errorf("%w: workspace restriction is active but the ID token did not include chatgpt_account_id", ErrReloginDenied)
	}
	if actual != expectedWorkspaceID {
		return fmt.Errorf("%w: login is restricted to workspace id %s", ErrReloginDenied, expectedWorkspaceID)
	}
	return nil
}

func oauthCallbackErrorMessage(errorCode, description string) string {
	if errorCode == "access_denied" && strings.Contains(strings.ToLower(description), "missing_codex_entitlement") {
		return "Codex is not enabled for this workspace. Contact your workspace administrator."
	}
	if strings.TrimSpace(description) != "" {
		return "Sign-in failed: " + strings.TrimSpace(description)
	}
	return "Sign-in failed: " + strings.TrimSpace(errorCode)
}

func apiKeyExchangeFailurePageMessage(err error) string {
	if err != nil && strings.Contains(err.Error(), "organization_id") {
		return "Sign-in succeeded, but OpenAI Platform setup is incomplete for credential issuance. Complete organization/project setup or provide a valid auth.json, then retry."
	}
	return "Sign-in succeeded but credential exchange failed. Return to the application for details."
}

func writeReloginHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "<html><body><p>%s</p></body></html>", html.EscapeString(body))
}

func openSystemBrowser(rawURL string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{rawURL}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		command = "xdg-open"
		args = []string{rawURL}
	}
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath(command); err != nil {
			return fmt.Errorf("goa: browser launcher not found: %w; retry with NoBrowser=true", err)
		}
	} else if os.Getenv("ComSpec") == "" {
		// Keep Windows behavior simple; rundll32 is normally available on supported systems.
	}
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("goa: open system browser: %w", err)
	}
	return nil
}
