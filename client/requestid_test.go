package client

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestValidateClientRequestID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "empty"},
		{name: "uuid", value: "123e4567-e89b-12d3-a456-426614174000"},
		{name: "trace", value: "trace_ABC-123"},
		{name: "too long", value: strings.Repeat("a", maxClientRequestIDLength+1), wantErr: true},
		{name: "non ascii", value: "요청-1", wantErr: true},
		{name: "control", value: "abc\n123", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClientRequestID(tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNewRequestRejectsInvalidClientRequestID(t *testing.T) {
	client, err := NewClient(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.newRequest(WithClientRequestID(context.Background(), "요청-1"), http.MethodGet, "/v1/models", nil)
	if err == nil {
		t.Fatal("expected invalid client request id error")
	}
}
