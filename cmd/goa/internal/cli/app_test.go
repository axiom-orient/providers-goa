package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goa "github.com/axiom-orient/providers-goa/client"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "codex send") {
		t.Fatalf("unexpected help output: %q", out)
	}
	if !strings.Contains(out, "goa [--json] gemini") {
		t.Fatalf("help should mention gemini provider: %q", out)
	}
	if !strings.Contains(out, "codex relogin") {
		t.Fatalf("help should mention browser relogin: %q", out)
	}
}

func TestRunSendStreamsText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello \"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"world\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello world\"}]}]}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{
		"codex",
		"--auth-path", writeCLIAuth(t),
		"--base-url", srv.URL,
		"send",
		"hello",
		"--model", "gpt-test",
		"--stream",
	})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "hello world\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func writeCLIAuth(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"tok-test","refresh_token":"refresh-test","account_id":"acc-test"}}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunHelpIncludesVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "goa version") {
		t.Fatalf("help should mention version command: %q", out)
	}
}

func TestRunVersion(t *testing.T) {
	prevVersion, prevCommit, prevBuildDate := goa.Version, goa.Commit, goa.BuildDate
	defer func() {
		goa.Version = prevVersion
		goa.Commit = prevCommit
		goa.BuildDate = prevBuildDate
	}()
	goa.Version = "v0.4.0"
	goa.Commit = "abc123"
	goa.BuildDate = "2026-04-18"

	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{"version"})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"version: v0.4.0", "commit: abc123", "build_date: 2026-04-18"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q: %q", want, out)
		}
	}
}
