package client

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

const (
	defaultMaxRetries     = 2
	defaultRetryBaseDelay = 200 * time.Millisecond
	defaultRetryMaxDelay  = 2 * time.Second
)

// RetryPolicy configures retry behavior for transport-level failures.
type RetryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// TransportEventType identifies hook callbacks emitted by the HTTP transport.
type TransportEventType string

const (
	TransportEventAttemptStart    TransportEventType = "attempt_start"
	TransportEventAttemptComplete TransportEventType = "attempt_complete"
	TransportEventRetryScheduled  TransportEventType = "retry_scheduled"
)

// TransportEvent is emitted to the configured transport hook.
type TransportEvent struct {
	Type            TransportEventType
	Method          string
	Path            string
	Attempt         int
	MaxRetries      int
	StatusCode      int
	RequestID       string
	ClientRequestID string
	Duration        time.Duration
	RetryDelay      time.Duration
	Err             error
}

// TransportHook observes request attempts, retry scheduling, and completion.
type TransportHook func(TransportEvent)

type retryPolicy struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func normalizeRetryPolicy(cfg *RetryPolicy) (retryPolicy, error) {
	policy := retryPolicy{
		MaxRetries: defaultMaxRetries,
		BaseDelay:  defaultRetryBaseDelay,
		MaxDelay:   defaultRetryMaxDelay,
	}
	if cfg == nil {
		return policy, nil
	}
	policy.MaxRetries = cfg.MaxRetries
	if cfg.BaseDelay > 0 {
		policy.BaseDelay = cfg.BaseDelay
	}
	if cfg.MaxDelay > 0 {
		policy.MaxDelay = cfg.MaxDelay
	}
	if policy.MaxRetries < 0 {
		return retryPolicy{}, &ValidationError{Field: "retry_policy.max_retries", Message: "must be zero or greater"}
	}
	if policy.BaseDelay <= 0 {
		return retryPolicy{}, &ValidationError{Field: "retry_policy.base_delay", Message: "must be greater than zero"}
	}
	if policy.MaxDelay <= 0 {
		return retryPolicy{}, &ValidationError{Field: "retry_policy.max_delay", Message: "must be greater than zero"}
	}
	if policy.MaxDelay < policy.BaseDelay {
		policy.MaxDelay = policy.BaseDelay
	}
	return policy, nil
}

func retryableStatusCode(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return true
	default:
		return status >= 500
	}
}

func shouldRetryTransportError(req *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if req != nil && req.Context() != nil && req.Context().Err() != nil {
		return false
	}
	return true
}

func retryDelay(policy retryPolicy, retryNumber int) time.Duration {
	delay := policy.BaseDelay
	for i := 1; i < retryNumber; i++ {
		if delay >= policy.MaxDelay/2 {
			delay = policy.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	if delay <= 0 {
		return policy.BaseDelay
	}
	jitterSpan := int64(delay / 2)
	if jitterSpan <= 0 {
		return delay
	}
	return delay/2 + time.Duration(rand.Int63n(jitterSpan+1))
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
