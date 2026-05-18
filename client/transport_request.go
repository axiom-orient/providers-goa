package client

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
)

type noRetryContextKey struct{}

func withNoTransportRetry(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, noRetryContextKey{}, true)
}

func transportRetryDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	value, _ := ctx.Value(noRetryContextKey{}).(bool)
	return value
}

func (c *Client) newRequest(ctx context.Context, method, path string, body []byte) (*http.Request, error) {
	return c.newRequestURL(ctx, method, c.cfg.BaseURL+path, body)
}

func (c *Client) newRequestURL(ctx context.Context, method, rawURL string, body []byte) (*http.Request, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.cfg.UserAgent != "" {
		req.Header.Set("User-Agent", c.cfg.UserAgent)
	}
	if token := c.authBearerToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.cfg.Organization != "" {
		req.Header.Set("OpenAI-Organization", c.cfg.Organization)
	}
	if c.cfg.Project != "" {
		req.Header.Set("OpenAI-Project", c.cfg.Project)
	}
	if clientRequestID := clientRequestIDFromContext(ctx); clientRequestID != "" {
		if err := ValidateClientRequestID(clientRequestID); err != nil {
			return nil, err
		}
		req.Header.Set("X-Client-Request-Id", clientRequestID)
	}
	return req, nil
}

func (c *Client) authBearerToken() string {
	if c == nil {
		return ""
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	if c.apiKey != "" {
		return c.apiKey
	}
	return c.accessToken
}

func (c *Client) isChatGPTTransport() bool {
	return c != nil && c.transport == authTransportChatGPT
}

func (c *Client) chatGPTURL(path string) string {
	base := defaultChatGPTBaseURL
	if c != nil {
		baseURL := strings.TrimSpace(c.cfg.BaseURL)
		if baseURL != "" && baseURL != defaultBaseURL {
			base = strings.TrimRight(baseURL, "/")
		}
	}
	return strings.TrimRight(base, "/") + path
}

func (c *Client) applyChatGPTHeaders(req *http.Request, stream bool) {
	if req == nil {
		return
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("version", defaultChatGPTVersion)
	if c != nil {
		c.authMu.Lock()
		accountID := strings.TrimSpace(c.accountID)
		c.authMu.Unlock()
		if accountID != "" {
			req.Header.Set("ChatGPT-Account-ID", accountID)
		}
	}
}

func (c *Client) buildChatGPTModelsRequest(ctx context.Context) (*http.Request, error) {
	rawURL := c.chatGPTURL(defaultChatGPTModels)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("client_version", defaultChatGPTClientVer)
	parsed.RawQuery = query.Encode()
	req, err := c.newRequestURL(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	c.applyChatGPTHeaders(req, false)
	return req, nil
}

func (c *Client) buildChatGPTResponsesRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := c.newRequestURL(withNoTransportRetry(ctx), http.MethodPost, c.chatGPTURL(defaultChatGPTResponses), body)
	if err != nil {
		return nil, err
	}
	c.applyChatGPTHeaders(req, true)
	return req, nil
}
