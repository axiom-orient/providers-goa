package client

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrMissingAPIKey is returned when a network call requires an API key but none is resolved.
	ErrMissingAPIKey = errors.New("goa: missing API key")

	// ErrMissingCredential is returned when a network call requires auth but none is resolved.
	ErrMissingCredential = errors.New("goa: missing credential")

	// ErrSendInProgress is returned when the same Client already has an active response send.
	ErrSendInProgress = errors.New("goa: response send already in progress")

	// ErrClientBusy is returned when an operation is blocked by an active response send.
	ErrClientBusy = errors.New("goa: client busy with active response send")

	// ErrReloginTimeout is returned when browser relogin does not receive a callback before the timeout.
	ErrReloginTimeout = errors.New("goa: browser relogin timed out")

	// ErrReloginDenied is returned when the OAuth callback denies or cancels browser relogin.
	ErrReloginDenied = errors.New("goa: browser relogin denied")
)

// ValidationError is returned for client-side request validation failures.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"goa: invalid request"}
	if e.Field != "" {
		parts = append(parts, "field="+e.Field)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

// StructuredOutputError is returned when a response cannot be decoded as structured JSON.
type StructuredOutputError struct {
	Message    string
	OutputText string
}

func (e *StructuredOutputError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"goa: structured output error"}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.OutputText != "" {
		parts = append(parts, "output_text="+e.OutputText)
	}
	return strings.Join(parts, " ")
}

// RefusalError is returned when a structured-output response contains an explicit refusal instead of JSON.
type RefusalError struct {
	Message string
}

func (e *RefusalError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"goa: model refusal"}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, " ")
}

// APIError is returned for non-2xx API responses.
type APIError struct {
	StatusCode int
	RequestID  string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("goa: API error status=%d", e.StatusCode)}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	if e.Body != "" {
		parts = append(parts, "body="+e.Body)
	}
	return strings.Join(parts, " ")
}

// isAPIStatus reports whether err is an APIError with the given HTTP status.
func isAPIStatus(err error, statusCode int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == statusCode
	}
	return false
}
