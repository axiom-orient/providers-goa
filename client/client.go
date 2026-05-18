package client

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// Client is the long-lived runtime object for official OpenAI API access.
type Client struct {
	cfg            Config
	httpClient     *http.Client
	apiKey         string
	accessToken    string
	refreshToken   string
	idToken        string
	accountID      string
	transport      authTransport
	authState      AuthState
	authPath       string
	models         ModelsService
	responses      ResponsesService
	retryPolicy    retryPolicy
	requestTimeout time.Duration
	hook           TransportHook

	sendMu     sync.Mutex
	sendActive bool
	authMu     sync.Mutex
}

// NewClient builds a client with a stable, documented surface.
func NewClient(cfg Config) (*Client, error) {
	cfg = cfg.normalized()
	if cfg.RequestTimeout < 0 {
		return nil, &ValidationError{Field: "request_timeout", Message: "must be zero or greater"}
	}
	retryPolicy, err := normalizeRetryPolicy(cfg.RetryPolicy)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			CheckRedirect: noRedirect,
		}
	} else if httpClient.CheckRedirect == nil {
		clone := *httpClient
		clone.CheckRedirect = noRedirect
		httpClient = &clone
	}

	authState, resolved, err := resolveAuthState(cfg)
	if err != nil {
		return nil, err
	}

	client := &Client{
		cfg:            cfg,
		httpClient:     httpClient,
		apiKey:         resolved.apiKey,
		accessToken:    resolved.accessToken,
		refreshToken:   resolved.refreshToken,
		idToken:        resolved.idToken,
		accountID:      resolved.accountID,
		transport:      resolved.transport,
		authState:      authState,
		authPath:       resolved.authPath,
		retryPolicy:    retryPolicy,
		requestTimeout: cfg.RequestTimeout,
		hook:           cfg.Hook,
	}
	client.models = ModelsService{client: client}
	client.responses = ResponsesService{client: client}
	return client, nil
}

func noRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}

// AuthState returns a copy of the resolved auth state.
func (c *Client) AuthState() AuthState {
	return c.authState.clone()
}

// Models returns the models service.
func (c *Client) Models() *ModelsService {
	return &c.models
}

// Responses returns the responses service.
func (c *Client) Responses() *ResponsesService {
	return &c.responses
}

func (c *Client) requireAPIKey() error {
	if c.apiKey == "" {
		return ErrMissingAPIKey
	}
	return nil
}

func (c *Client) requireCredential() error {
	if c == nil {
		return ErrMissingCredential
	}
	if c.apiKey == "" && c.accessToken == "" && c.refreshToken == "" {
		return ErrMissingCredential
	}
	return nil
}

func (c *Client) validate() error {
	if c == nil {
		return errors.New("goa: nil client")
	}
	return nil
}

func (c *Client) beginSend() error {
	if err := c.validate(); err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.sendActive {
		return ErrSendInProgress
	}
	c.sendActive = true
	return nil
}

func (c *Client) endSend() {
	if c == nil {
		return
	}
	c.sendMu.Lock()
	c.sendActive = false
	c.sendMu.Unlock()
}

func (c *Client) ensureNoActiveSend() error {
	if err := c.validate(); err != nil {
		return err
	}
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.sendActive {
		return ErrClientBusy
	}
	return nil
}

func (c *Client) emit(event TransportEvent) {
	if c == nil || c.hook == nil {
		return
	}
	c.hook(event)
}
