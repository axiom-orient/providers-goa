package client

import (
	"net/http"
	"testing"
)

func TestWithClientRequestIDNilContextReturnsBackground(t *testing.T) {
	ctx := WithClientRequestID(nil, "cid-123")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if got := clientRequestIDFromContext(ctx); got != "cid-123" {
		t.Fatalf("unexpected request id: %q", got)
	}
}

func TestNewRequestNilContextUsesBackground(t *testing.T) {
	client, err := NewClient(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	req, err := client.newRequest(nil, http.MethodGet, "/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.Context() == nil {
		t.Fatal("expected non-nil request context")
	}
}
