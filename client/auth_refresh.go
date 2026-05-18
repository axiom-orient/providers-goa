package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/axiom-orient/providers-goa/client/internal/authjson"
)

type refreshedChatGPTTokens = authjson.RefreshedTokens

func (c *Client) hasChatGPTAccessToken() bool {
	if c == nil {
		return false
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return strings.TrimSpace(c.accessToken) != ""
}

func (c *Client) hasChatGPTRefreshToken() bool {
	if c == nil {
		return false
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	return strings.TrimSpace(c.refreshToken) != ""
}

func (c *Client) ensureChatGPTAccessToken(ctx context.Context) error {
	if c == nil || !c.isChatGPTTransport() {
		return nil
	}
	if c.hasChatGPTAccessToken() {
		return nil
	}
	if !c.hasChatGPTRefreshToken() {
		return ErrMissingCredential
	}
	return c.refreshChatGPTAuth(ctx)
}

func (c *Client) refreshChatGPTAuth(ctx context.Context) error {
	if c == nil {
		return ErrMissingCredential
	}
	if ctx == nil {
		ctx = context.Background()
	}

	c.authMu.Lock()
	defer c.authMu.Unlock()

	refreshToken := strings.TrimSpace(c.refreshToken)
	if refreshToken == "" {
		return &ValidationError{Field: "auth.refresh_token", Message: "missing ChatGPT refresh token"}
	}

	issuer := strings.TrimRight(strings.TrimSpace(c.cfg.AuthIssuerURL), "/")
	if issuer == "" {
		issuer = defaultAuthIssuerURL
	}
	clientID := strings.TrimSpace(c.cfg.OAuthClientID)
	if clientID == "" {
		clientID = defaultOAuthClientID
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issuer+"/oauth/token", bytes.NewBufferString(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("goa: refresh ChatGPT auth: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("goa: refresh ChatGPT auth: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, RequestID: resp.Header.Get("x-request-id"), Body: summarizeBody(raw)}
	}

	refreshed, err := parseRefreshedChatGPTTokens(raw)
	if err != nil {
		return err
	}
	c.accessToken = refreshed.AccessToken
	c.refreshToken = refreshed.RefreshToken
	c.idToken = refreshed.IDToken
	if refreshed.AccountID != "" {
		c.accountID = refreshed.AccountID
	}
	c.authState.HasAccessToken = true
	c.authState.HasRefreshToken = true
	c.authState.HasIDToken = c.idToken != ""
	c.authState.HasAccountID = c.accountID != ""
	c.authState.Transport = string(authTransportChatGPT)

	if c.authPath != "" {
		_ = persistRefreshedChatGPTTokens(c.authPath, c.accessToken, c.refreshToken, c.idToken, c.accountID)
	}
	return nil
}

func parseRefreshedChatGPTTokens(raw []byte) (authjson.RefreshedTokens, error) {
	out, err := authjson.ParseRefreshedTokens(raw)
	if err == nil {
		return out, nil
	}
	if errors.Is(err, authjson.ErrInvalidRefreshPayload) {
		return authjson.RefreshedTokens{}, &ValidationError{Field: "auth.refresh", Message: "refresh-token exchange returned invalid JSON"}
	}
	return authjson.RefreshedTokens{}, fmt.Errorf("goa: refresh ChatGPT auth: invalid JSON: %w", err)
}
