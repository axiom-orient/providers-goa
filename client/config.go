package client

import (
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL          = "https://api.openai.com"
	defaultChatGPTBaseURL   = "https://chatgpt.com"
	defaultChatGPTResponses = "/backend-api/codex/responses"
	defaultChatGPTModels    = "/backend-api/codex/models"
	defaultChatGPTVersion   = "0.130.0"
	defaultChatGPTClientVer = "0.130.0"
	defaultAuthIssuerURL    = "https://auth.openai.com"
	defaultOAuthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
)

// Config configures a Client.
type Config struct {
	// BaseURL defaults to https://api.openai.com.
	BaseURL string

	// APIKey takes highest precedence when non-empty unless PreferChatGPT is set.
	APIKey string

	// APIKeyEnv is optional. Leave empty to avoid process environment API-key lookup.
	APIKeyEnv string

	// PreferChatGPT selects file-backed ChatGPT/Codex credentials over API-key sources.
	PreferChatGPT bool

	// ChatGPTClientVersion overrides the backend models client_version query value.
	ChatGPTClientVersion string

	// AuthPath points directly to auth.json.
	AuthPath string

	// AuthHome points to a directory containing auth.json.
	AuthHome string

	// AuthIssuerURL defaults to https://auth.openai.com for file-backed ChatGPT auth refresh.
	AuthIssuerURL string

	// OAuthClientID defaults to the Codex desktop OAuth client id used by auth.json refresh.
	OAuthClientID string

	// HTTPClient overrides the default client.
	HTTPClient *http.Client

	// RetryPolicy configures transport retries. Nil uses the default client policy.
	RetryPolicy *RetryPolicy

	// RequestTimeout limits each HTTP attempt. The caller context still bounds the whole request lifecycle.
	RequestTimeout time.Duration

	// Hook observes transport attempts and retries.
	Hook TransportHook

	// Optional OpenAI headers.
	Organization string
	Project      string

	// UserAgent defaults to the current build user agent.
	UserAgent string
}

func (c Config) normalized() Config {
	if c.BaseURL == "" {
		c.BaseURL = defaultBaseURL
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent()
	}
	if c.AuthIssuerURL == "" {
		c.AuthIssuerURL = defaultAuthIssuerURL
	}
	c.AuthIssuerURL = strings.TrimRight(c.AuthIssuerURL, "/")
	if c.OAuthClientID == "" {
		c.OAuthClientID = defaultOAuthClientID
	}
	return c
}
