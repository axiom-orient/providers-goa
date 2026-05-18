package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponsesCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Method; got != http.MethodPost {
			t.Fatalf("unexpected method: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if got := r.Header.Get("X-Client-Request-Id"); got != "cid-123" {
			t.Fatalf("unexpected client request id: %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "gpt-test" {
			t.Fatalf("unexpected model payload: %#v", body)
		}
		if body["input"] != "hello" {
			t.Fatalf("unexpected input payload: %#v", body)
		}
		if body["instructions"] != "Be concise." {
			t.Fatalf("unexpected instructions payload: %#v", body)
		}
		if body["temperature"] != float64(0) {
			t.Fatalf("unexpected extra payload: %#v", body)
		}

		w.Header().Set("x-request-id", "req_resp_123")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_123",
			"object": "response",
			"model":  "gpt-test",
			"output": []map[string]any{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "output_text",
					"text": "hello world",
				}},
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(
		WithClientRequestID(context.Background(), "cid-123"),
		CreateResponseRequest{
			Model:        "gpt-test",
			Input:        "hello",
			Instructions: "Be concise.",
			Extra:        map[string]any{"temperature": 0},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Meta.RequestID != "req_resp_123" {
		t.Fatalf("unexpected request id: %q", resp.Meta.RequestID)
	}
	if got := resp.OutputText(); got != "hello world" {
		t.Fatalf("unexpected output text: %q", got)
	}
	if len(resp.Raw) == 0 {
		t.Fatal("expected raw payload")
	}
}

func TestResponsesCreateAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-request-id", "req_error_123")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad auth"}}`))
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.RequestID != "req_error_123" {
		t.Fatalf("unexpected request id: %q", apiErr.RequestID)
	}
}

func TestResponsesCreateChatGPTTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultChatGPTResponses {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-test" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if got := r.Header.Get("version"); got != defaultChatGPTVersion {
			t.Fatalf("unexpected version header: %q", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("unexpected accept header: %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if got := body["stream"]; got != true {
			t.Fatalf("expected stream=true body, got %#v", body)
		}
		format := body["text"].(map[string]any)["format"].(map[string]any)
		if got := format["type"]; got != "json_schema" {
			t.Fatalf("unexpected text format: %#v", body["text"])
		}
		if _, ok := body["metadata"]; ok {
			t.Fatalf("chatgpt request should not include unsupported metadata: %#v", body["metadata"])
		}
		input, ok := body["input"].([]any)
		if !ok || len(input) != 1 {
			t.Fatalf("unexpected input payload: %#v", body["input"])
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_chatgpt_create")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_chatgpt\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}]}}\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		BaseURL:  srv.URL,
		AuthPath: writeTempAuthFile(t, `{"auth_mode":"chatgpt","tokens":{"access_token":"tok-test","account_id":"acc-test"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(context.Background(), CreateResponseRequest{
		Model:    "gpt-5.4",
		Input:    "hello",
		Text:     JSONSchemaTextFormat("provider_status", map[string]any{"type": "object"}, JSONSchemaFormatOptions{Strict: true}),
		Metadata: map[string]string{"trace": "openai-only"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Meta.RequestID; got != "req_chatgpt_create" {
		t.Fatalf("unexpected request id: %q", got)
	}
	if got := resp.OutputText(); got != "hello" {
		t.Fatalf("unexpected output text: %q", got)
	}
}
