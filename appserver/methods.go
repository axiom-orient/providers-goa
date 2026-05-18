package appserver

import "context"

// InitializeResult returns the stable initialize response subset captured during startup.
func (c *Client) InitializeResult() InitializeResult {
	if c == nil {
		return InitializeResult{}
	}
	return c.initResult
}

// Notifications exposes stable JSON-RPC notifications.
func (c *Client) Notifications() <-chan Notification {
	if c == nil {
		ch := make(chan Notification)
		close(ch)
		return ch
	}
	return c.notifications
}

// AccountRead calls account/read.
func (c *Client) AccountRead(ctx context.Context, refreshToken bool) (AccountReadResult, error) {
	var out AccountReadResult
	err := c.call(ctx, "account/read", map[string]any{"refreshToken": refreshToken}, &out)
	return out, err
}

// LoginWithAPIKey calls account/login/start with type apiKey.
func (c *Client) LoginWithAPIKey(ctx context.Context, apiKey string) (LoginStartResult, error) {
	var out LoginStartResult
	err := c.call(ctx, "account/login/start", map[string]any{"type": "apiKey", "apiKey": apiKey}, &out)
	return out, err
}

// StartChatGPTLogin starts the managed ChatGPT browser flow.
func (c *Client) StartChatGPTLogin(ctx context.Context) (LoginStartResult, error) {
	var out LoginStartResult
	err := c.call(ctx, "account/login/start", map[string]any{"type": "chatgpt"}, &out)
	return out, err
}

// StartChatGPTDeviceCodeLogin starts the managed ChatGPT device-code flow.
func (c *Client) StartChatGPTDeviceCodeLogin(ctx context.Context) (LoginStartResult, error) {
	var out LoginStartResult
	err := c.call(ctx, "account/login/start", map[string]any{"type": "chatgptDeviceCode"}, &out)
	return out, err
}

// LoginWithChatGPTAuthTokens calls account/login/start with externally managed ChatGPT tokens.
func (c *Client) LoginWithChatGPTAuthTokens(ctx context.Context, idToken, accessToken string) (LoginStartResult, error) {
	var out LoginStartResult
	err := c.call(ctx, "account/login/start", map[string]any{"type": "chatgptAuthTokens", "idToken": idToken, "accessToken": accessToken}, &out)
	return out, err
}

// CancelLogin calls account/login/cancel.
func (c *Client) CancelLogin(ctx context.Context, loginID string) error {
	return c.call(ctx, "account/login/cancel", map[string]any{"loginId": loginID}, nil)
}

// Logout calls account/logout.
func (c *Client) Logout(ctx context.Context) error {
	return c.call(ctx, "account/logout", map[string]any{}, nil)
}

// ReadRateLimits calls account/rateLimits/read.
func (c *Client) ReadRateLimits(ctx context.Context) (RateLimitsReadResult, error) {
	var out RateLimitsReadResult
	err := c.call(ctx, "account/rateLimits/read", map[string]any{}, &out)
	return out, err
}
