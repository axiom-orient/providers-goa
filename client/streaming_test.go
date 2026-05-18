package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestResponsesStream(t *testing.T) {
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
		if got := r.Header.Get("X-Client-Request-Id"); got != "cid-stream-123" {
			t.Fatalf("unexpected client request id: %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["stream"] != true {
			t.Fatalf("expected stream=true payload, got %#v", body)
		}
		if body["model"] != "gpt-test" {
			t.Fatalf("unexpected model payload: %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_123")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.created\n")
		fmt.Fprint(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello \"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"world\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\"}]}],\"usage\":{\"input_tokens\":1,\"output_tokens\":2,\"total_tokens\":3}}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := client.Responses().Stream(
		WithClientRequestID(context.Background(), "cid-stream-123"),
		CreateResponseRequest{Model: "gpt-test", Input: "hello"},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var events []string
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event.Type)
	}

	wantEvents := []string{
		"response.created",
		"response.output_text.delta",
		"response.output_text.delta",
		"response.completed",
	}
	if fmt.Sprint(events) != fmt.Sprint(wantEvents) {
		t.Fatalf("unexpected events: got %v want %v", events, wantEvents)
	}
	if got := stream.Meta().RequestID; got != "req_stream_123" {
		t.Fatalf("unexpected request id: %q", got)
	}
	if got := stream.OutputText(); got != "hello world" {
		t.Fatalf("unexpected output text: %q", got)
	}
	final, ok := stream.FinalResponse()
	if !ok {
		t.Fatal("expected final response")
	}
	if final.Meta.RequestID != "req_stream_123" {
		t.Fatalf("unexpected final request id: %q", final.Meta.RequestID)
	}
	if got := final.OutputText(); got != "hello world" {
		t.Fatalf("unexpected final output text: %q", got)
	}
	if final.Usage == nil || final.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected usage payload: %#v", final.Usage)
	}
	if len(final.Raw) == 0 {
		t.Fatal("expected raw response payload from terminal event")
	}
}

func TestResponsesStreamBlocksConcurrentOperationsUntilCompletion(t *testing.T) {
	var responsesCalls atomic.Int32
	var modelsCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			call := responsesCalls.Add(1)
			if call != 1 {
				t.Fatalf("unexpected extra responses call: %d", call)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("expected http.Flusher")
			}
			fmt.Fprint(w, "event: response.created\n")
			fmt.Fprint(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n")
			flusher.Flush()
			close(started)
			<-release
			fmt.Fprint(w, "event: response.completed\n")
			fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"done\"}]}]}}\n\n")
			flusher.Flush()
		case "/v1/models":
			modelsCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": []map[string]any{}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := client.Responses().Stream(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	event, err := stream.Next()
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != "response.created" {
		t.Fatalf("unexpected first event: %#v", event)
	}
	<-started

	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "second"})
	if !errors.Is(err, ErrSendInProgress) {
		t.Fatalf("want ErrSendInProgress, got %v", err)
	}
	_, err = client.Models().List(context.Background())
	if !errors.Is(err, ErrClientBusy) {
		t.Fatalf("want ErrClientBusy, got %v", err)
	}
	if got := modelsCalls.Load(); got != 0 {
		t.Fatalf("models endpoint should not be hit during active stream, got %d calls", got)
	}

	close(release)
	for {
		_, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err = client.Models().List(context.Background())
	if err != nil {
		t.Fatalf("expected models list after stream completion, got %v", err)
	}
	if got := modelsCalls.Load(); got != 1 {
		t.Fatalf("expected one models call after release, got %d", got)
	}
}
