package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestResponsesCreateExclusiveSend(t *testing.T) {
	var responsesCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		call := responsesCalls.Add(1)
		if call == 1 {
			close(started)
			<-release
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_1",
			"object": "response",
			"output": []map[string]any{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "output_text",
					"text": "ok",
				}},
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
		errCh <- err
	}()

	<-started

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "second"})
	if !errors.Is(err, ErrSendInProgress) {
		t.Fatalf("want ErrSendInProgress, got %v", err)
	}
	if got := responsesCalls.Load(); got != 1 {
		t.Fatalf("expected one in-flight request, got %d", got)
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("first create returned error: %v", err)
	}

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "third"})
	if err != nil {
		t.Fatalf("expected guard release after completion, got %v", err)
	}
	if got := responsesCalls.Load(); got != 2 {
		t.Fatalf("expected second network request after release, got %d", got)
	}
}

func TestModelsListBlockedWhileSendActive(t *testing.T) {
	var modelsCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			close(started)
			<-release
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "resp_1",
				"object": "response",
			})
		case "/v1/models":
			modelsCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"object": "list",
				"data":   []map[string]any{},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
		errCh <- err
	}()

	<-started

	_, err = client.Models().List(context.Background())
	if !errors.Is(err, ErrClientBusy) {
		t.Fatalf("want ErrClientBusy, got %v", err)
	}
	if got := modelsCalls.Load(); got != 0 {
		t.Fatalf("models endpoint should not be hit while send is active, got %d calls", got)
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("response create returned error: %v", err)
	}
}
