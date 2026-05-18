package client

import "context"

const maxClientRequestIDLength = 512

type clientRequestIDContextKey struct{}

// ValidateClientRequestID validates the X-Client-Request-Id value before it is sent.
func ValidateClientRequestID(requestID string) error {
	if requestID == "" {
		return nil
	}
	if len(requestID) > maxClientRequestIDLength {
		return &ValidationError{Field: "client_request_id", Message: "must be at most 512 ASCII bytes"}
	}
	for i := 0; i < len(requestID); i++ {
		if requestID[i] < 0x20 || requestID[i] > 0x7e {
			return &ValidationError{Field: "client_request_id", Message: "must contain only printable ASCII characters"}
		}
	}
	return nil
}

// WithClientRequestID attaches X-Client-Request-Id to subsequent API calls.
func WithClientRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, clientRequestIDContextKey{}, requestID)
}

func clientRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(clientRequestIDContextKey{}).(string)
	return v
}
