package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestResponsesCreateRejectsReservedExtraKeyWithoutNetwork(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_1", "object": "response"})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "hello",
		Extra: map[string]any{"stream": true},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "extra.stream" {
		t.Fatalf("unexpected field: %q", validationErr.Field)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("request should not hit network, got %d calls", got)
	}
}

func TestResponsesCreateRejectsDocumentedMetadataLimits(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_1", "object": "response"})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	metadata := map[string]string{}
	for i := 0; i < 17; i++ {
		metadata[fmt.Sprintf("k%d", i)] = "v"
	}
	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{
		Model:    "gpt-test",
		Input:    "hello",
		Metadata: metadata,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "metadata" {
		t.Fatalf("unexpected field: %q", validationErr.Field)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("request should not hit network, got %d calls", got)
	}
}

func TestResponsesCreateRejectsConversationWithPreviousResponseID(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_1", "object": "response"})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "hello",
		Extra: map[string]any{
			"previous_response_id": "resp_prev",
			"conversation":         map[string]any{"id": "conv_1"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "extra.previous_response_id" {
		t.Fatalf("unexpected field: %q", validationErr.Field)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("request should not hit network, got %d calls", got)
	}
}

func TestResponsesCreateRejectsStreamOptionsOutsideStreamingMode(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "resp_1", "object": "response"})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "hello",
		Extra: map[string]any{
			"stream_options": map[string]any{"include_obfuscation": false},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "extra.stream_options" {
		t.Fatalf("unexpected field: %q", validationErr.Field)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("request should not hit network, got %d calls", got)
	}
}

func TestResponsesStreamAllowsDocumentedStreamOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["stream"] != true {
			t.Fatalf("expected stream=true payload, got %#v", body)
		}
		streamOptions, ok := body["stream_options"].(map[string]any)
		if !ok {
			t.Fatalf("expected stream_options payload, got %#v", body["stream_options"])
		}
		if streamOptions["include_obfuscation"] != false {
			t.Fatalf("unexpected stream_options payload: %#v", streamOptions)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}]}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := client.Responses().Stream(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "hello",
		Extra: map[string]any{
			"stream_options": map[string]any{"include_obfuscation": false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	for {
		_, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.TrimSpace(stream.OutputText()); got != "ok" {
		t.Fatalf("unexpected stream output text: %q", got)
	}
}
