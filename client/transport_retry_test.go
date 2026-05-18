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
	"time"
)

func TestResponsesCreateRetriesOnRetryableStatusAndEmitsHook(t *testing.T) {
	var calls atomic.Int32
	var events []TransportEvent

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		call := calls.Add(1)
		if call == 1 {
			w.Header().Set("x-request-id", "req_retry_1")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"temporary failure"}}`))
			return
		}
		w.Header().Set("x-request-id", "req_retry_2")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_1",
			"object": "response",
			"output": []map[string]any{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "output_text",
					"text": "retried ok",
				}},
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		RetryPolicy: &RetryPolicy{
			MaxRetries: 1,
			BaseDelay:  time.Nanosecond,
			MaxDelay:   time.Nanosecond,
		},
		Hook: func(event TransportEvent) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
	if resp.Meta.RequestID != "req_retry_2" {
		t.Fatalf("unexpected final request id: %q", resp.Meta.RequestID)
	}
	if got := resp.OutputText(); got != "retried ok" {
		t.Fatalf("unexpected output text: %q", got)
	}
	if len(events) != 5 {
		t.Fatalf("unexpected hook event count: %d %#v", len(events), events)
	}
	wantTypes := []TransportEventType{
		TransportEventAttemptStart,
		TransportEventAttemptComplete,
		TransportEventRetryScheduled,
		TransportEventAttemptStart,
		TransportEventAttemptComplete,
	}
	for i, want := range wantTypes {
		if events[i].Type != want {
			t.Fatalf("unexpected event type at %d: got %q want %q", i, events[i].Type, want)
		}
	}
	if events[1].StatusCode != http.StatusInternalServerError || events[1].RequestID != "req_retry_1" {
		t.Fatalf("unexpected first completion event: %#v", events[1])
	}
	if events[2].RetryDelay < 0 {
		t.Fatalf("unexpected retry delay: %#v", events[2])
	}
	if events[4].StatusCode != http.StatusOK || events[4].RequestID != "req_retry_2" {
		t.Fatalf("unexpected final completion event: %#v", events[4])
	}
}

func TestResponsesCreateRetriesOnPerAttemptTimeout(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(250 * time.Millisecond):
				w.Header().Set("x-request-id", "req_timeout_1")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "late"})
				return
			}
		}
		w.Header().Set("x-request-id", "req_timeout_2")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_2",
			"object": "response",
			"output": []map[string]any{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "output_text",
					"text": "timeout recovered",
				}},
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		BaseURL:        srv.URL,
		APIKey:         "sk-test",
		RequestTimeout: 100 * time.Millisecond,
		RetryPolicy: &RetryPolicy{
			MaxRetries: 1,
			BaseDelay:  time.Nanosecond,
			MaxDelay:   time.Nanosecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
	if resp.Meta.RequestID != "req_timeout_2" {
		t.Fatalf("unexpected final request id: %q", resp.Meta.RequestID)
	}
	if got := resp.OutputText(); got != "timeout recovered" {
		t.Fatalf("unexpected output text: %q", got)
	}
}

func TestResponsesStreamRetriesOnRetryableStatus(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		if call == 1 {
			w.Header().Set("x-request-id", "req_stream_retry_1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_retry_2")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_3\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}]}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client, err := NewClient(Config{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		RetryPolicy: &RetryPolicy{
			MaxRetries: 1,
			BaseDelay:  time.Nanosecond,
			MaxDelay:   time.Nanosecond,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := client.Responses().Stream(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var text string
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		text += event.TextChunk()
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
	if stream.Meta().RequestID != "req_stream_retry_2" {
		t.Fatalf("unexpected stream request id: %q", stream.Meta().RequestID)
	}
	if text != "hello" {
		t.Fatalf("unexpected stream text: %q", text)
	}
}
