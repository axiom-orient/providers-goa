package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelsList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Method; got != http.MethodGet {
			t.Fatalf("unexpected method: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		w.Header().Set("x-request-id", "req_models_123")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{{
				"id":       "gpt-test",
				"object":   "model",
				"owned_by": "openai",
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Models().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Meta.RequestID != "req_models_123" {
		t.Fatalf("unexpected request id: %q", resp.Meta.RequestID)
	}
	if len(resp.Data) != 1 || resp.Data[0].ID != "gpt-test" {
		t.Fatalf("unexpected models payload: %#v", resp.Data)
	}
}

func TestModelsListChatGPTTransport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != defaultChatGPTModels {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("client_version"); got != defaultChatGPTClientVer {
			t.Fatalf("unexpected client_version: %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok-test" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		if got := r.Header.Get("version"); got != defaultChatGPTVersion {
			t.Fatalf("unexpected version header: %q", got)
		}
		if got := r.Header.Get("ChatGPT-Account-ID"); got != "acc-test" {
			t.Fatalf("unexpected account header: %q", got)
		}
		w.Header().Set("x-request-id", "req_models_chatgpt")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"slug": "gpt-5.4"},
				{"slug": "gpt-5.4-mini"},
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		BaseURL:  srv.URL,
		AuthPath: writeTempAuthFile(t, `{"auth_mode":"chatgpt","tokens":{"access_token":"tok-test","account_id":"acc-test"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Models().List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Meta.RequestID; got != "req_models_chatgpt" {
		t.Fatalf("unexpected request id: %q", got)
	}
	if len(resp.Data) != 2 || resp.Data[0].ID != "gpt-5.4" || resp.Data[1].ID != "gpt-5.4-mini" {
		t.Fatalf("unexpected models payload: %#v", resp.Data)
	}
}
