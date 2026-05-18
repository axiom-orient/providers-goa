package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunSendStreamsRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"refusal\",\"refusal\":\"cannot comply\"}]}]}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"send", "--model", "gpt-test", "--input", "hello", "--stream", "--api-key", "sk-test", "--base-url", srv.URL})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "cannot comply\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunSendStreamErrorReturnsNonZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_123")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: error\n")
		fmt.Fprint(w, "data: {\"type\":\"error\",\"error\":{\"code\":\"rate_limit_exceeded\",\"message\":\"quota hit\"}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"send", "--model", "gpt-test", "--input", "hello", "--stream", "--api-key", "sk-test", "--base-url", srv.URL})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "response error") || !strings.Contains(stderr.String(), "req_stream_123") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunResponsesCreateRefusalPrintsRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"cannot comply"}]}]}`)
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"responses", "create", "--model", "gpt-test", "--input", "hello", "--api-key", "sk-test", "--base-url", srv.URL})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "cannot comply\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
}

func TestRunResponsesCreateFailedReturnsNonZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_failed_123")
		fmt.Fprint(w, `{"id":"resp_1","object":"response","status":"failed","error":{"code":"server_error","message":"backend exploded"}}`)
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"responses", "create", "--model", "gpt-test", "--input", "hello", "--api-key", "sk-test", "--base-url", srv.URL})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "response failed") || !strings.Contains(stderr.String(), "req_failed_123") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunSendStreamJSONIncludesRequestID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_json_123")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}]}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"send", "--model", "gpt-test", "--input", "hello", "--stream", "--json", "--api-key", "sk-test", "--base-url", srv.URL})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"request_id":"req_stream_json_123"`) {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}
