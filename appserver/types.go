package appserver

import (
	"context"
	"encoding/json"
	"fmt"
)

// ClientInfo identifies the integrating client during app-server initialization.
type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

// Capabilities configures optional app-server initialization features.
type Capabilities struct {
	ExperimentalAPI           bool     `json:"experimentalApi,omitempty"`
	OptOutNotificationMethods []string `json:"optOutNotificationMethods,omitempty"`
}

// ClientOptions configures a Client.
type ClientOptions struct {
	ClientInfo     ClientInfo
	Capabilities   Capabilities
	RefreshHandler ChatGPTAuthTokensRefreshHandler
}

// InitializeResult is the stable initialize response subset.
type InitializeResult struct {
	UserAgent      string `json:"userAgent,omitempty"`
	CodexHome      string `json:"codexHome,omitempty"`
	PlatformFamily string `json:"platformFamily,omitempty"`
	PlatformOS     string `json:"platformOs,omitempty"`
}

// Account describes the stable account/read response union subset.
type Account struct {
	Type     string `json:"type"`
	Email    string `json:"email,omitempty"`
	PlanType string `json:"planType,omitempty"`
}

// AccountReadResult is the stable account/read result subset.
type AccountReadResult struct {
	Account            *Account `json:"account,omitempty"`
	RequiresOpenAIAuth bool     `json:"requiresOpenaiAuth"`
}

// LoginStartResult is the stable account/login/start result subset.
type LoginStartResult struct {
	Type            string `json:"type"`
	LoginID         string `json:"loginId,omitempty"`
	AuthURL         string `json:"authUrl,omitempty"`
	VerificationURL string `json:"verificationUrl,omitempty"`
	UserCode        string `json:"userCode,omitempty"`
}

// Notification is a stable JSON-RPC notification envelope.
type Notification struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// AccountLoginCompletedNotification is emitted after account/login/start finishes.
type AccountLoginCompletedNotification struct {
	LoginID *string `json:"loginId"`
	Success bool    `json:"success"`
	Error   *string `json:"error"`
}

// AccountUpdatedNotification is emitted after auth mode changes.
type AccountUpdatedNotification struct {
	AuthMode *string `json:"authMode"`
	PlanType *string `json:"planType,omitempty"`
}

// RateLimitWindow describes a quota window.
type RateLimitWindow struct {
	UsedPercent        int   `json:"usedPercent"`
	WindowDurationMins int   `json:"windowDurationMins"`
	ResetsAt           int64 `json:"resetsAt"`
}

// RateLimit describes one ChatGPT rate limit bucket.
type RateLimit struct {
	LimitID   string           `json:"limitId,omitempty"`
	LimitName *string          `json:"limitName,omitempty"`
	Primary   *RateLimitWindow `json:"primary,omitempty"`
	Secondary *RateLimitWindow `json:"secondary,omitempty"`
}

// RateLimitsReadResult is the stable account/rateLimits/read result subset.
type RateLimitsReadResult struct {
	RateLimits          *RateLimit           `json:"rateLimits,omitempty"`
	RateLimitsByLimitID map[string]RateLimit `json:"rateLimitsByLimitId,omitempty"`
}

// RateLimitsUpdatedNotification is emitted when ChatGPT rate limits change.
type RateLimitsUpdatedNotification = RateLimitsReadResult

// ChatGPTAuthTokensRefreshRequest is the stable external-token refresh request payload.
type ChatGPTAuthTokensRefreshRequest struct {
	Reason            string `json:"reason,omitempty"`
	PreviousAccountID string `json:"previousAccountId,omitempty"`
}

// ChatGPTAuthTokens is the stable external-token refresh response payload.
type ChatGPTAuthTokens struct {
	IDToken     string `json:"idToken"`
	AccessToken string `json:"accessToken"`
}

// ChatGPTAuthTokensRefreshHandler serves account/chatgptAuthTokens/refresh requests.
type ChatGPTAuthTokensRefreshHandler func(context.Context, ChatGPTAuthTokensRefreshRequest) (ChatGPTAuthTokens, error)

// RPCError is returned for JSON-RPC error responses.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return fmt.Sprintf("appserver: RPC error code=%d", e.Code)
	}
	return fmt.Sprintf("appserver: RPC error code=%d message=%s", e.Code, e.Message)
}

// AccountLoginCompleted decodes account/login/completed notifications.
func (n Notification) AccountLoginCompleted() (AccountLoginCompletedNotification, bool, error) {
	if n.Method != "account/login/completed" {
		return AccountLoginCompletedNotification{}, false, nil
	}
	var payload AccountLoginCompletedNotification
	if err := json.Unmarshal(n.Params, &payload); err != nil {
		return AccountLoginCompletedNotification{}, true, err
	}
	return payload, true, nil
}

// AccountUpdated decodes account/updated notifications.
func (n Notification) AccountUpdated() (AccountUpdatedNotification, bool, error) {
	if n.Method != "account/updated" {
		return AccountUpdatedNotification{}, false, nil
	}
	var payload AccountUpdatedNotification
	if err := json.Unmarshal(n.Params, &payload); err != nil {
		return AccountUpdatedNotification{}, true, err
	}
	return payload, true, nil
}

// RateLimitsUpdated decodes account/rateLimits/updated notifications.
func (n Notification) RateLimitsUpdated() (RateLimitsUpdatedNotification, bool, error) {
	if n.Method != "account/rateLimits/updated" {
		return RateLimitsUpdatedNotification{}, false, nil
	}
	var payload RateLimitsUpdatedNotification
	if err := json.Unmarshal(n.Params, &payload); err != nil {
		return RateLimitsUpdatedNotification{}, true, err
	}
	return payload, true, nil
}
