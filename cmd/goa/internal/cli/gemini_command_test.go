package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGeminiGenerateRunsAdapterProcess(t *testing.T) {
	adapter := writeCLIGeminiAdapter(t, `
read line
printf '%s\n' '{"id":"1","result":{"text":"OK","provider":"test-adapter","model":"gemini-test"}}'
`)
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{
		"gemini",
		"generate",
		"hello",
		"--node-path", "/bin/sh",
		"--adapter-path", adapter,
	})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if got := stdout.String(); got != "OK\n" {
		t.Fatalf("unexpected stdout: %q", got)
	}
}

func TestRunGeminiModelsRunsAdapterProcess(t *testing.T) {
	adapter := writeCLIGeminiAdapter(t, `
read line
printf '%s\n' '{"id":"1","result":{"provider":"gemini-cli-core","releaseChannel":"stable","models":[{"id":"auto","name":"Auto","description":"Let Gemini CLI decide","tier":"auto","source":"gemini-cli-core","quota":null}]}}'
`)
	var stdout, stderr bytes.Buffer
	app := New(&stdout, &stderr)
	code := app.Run(context.Background(), []string{
		"gemini",
		"models",
		"--node-path", "/bin/sh",
		"--adapter-path", adapter,
	})
	if code != 0 {
		t.Fatalf("unexpected exit code: %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "auto") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
}

func writeCLIGeminiAdapter(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "adapter.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
