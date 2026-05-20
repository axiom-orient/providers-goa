package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeGeminiAdapterResponseReturnsResultAfterLogLines(t *testing.T) {
	raw := []byte("Loaded cached credentials.\n{\"id\":\"1\",\"result\":{\"text\":\"OK\",\"provider\":\"gemini-cli-core\",\"model\":\"gemini-2.5-pro\"}}\n")
	var response GeminiGenerateResponse
	if err := DecodeGeminiAdapterResponse(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response.Text != "OK" || response.Provider != "gemini-cli-core" || response.Model != "gemini-2.5-pro" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestDecodeGeminiAdapterResponseMapsRPCError(t *testing.T) {
	var response GeminiGenerateResponse
	err := DecodeGeminiAdapterResponse([]byte(`{"id":"1","error":{"code":-32601,"message":"unsupported method"}}`), &response)
	if err == nil || !strings.Contains(err.Error(), "code=-32601") || !strings.Contains(err.Error(), "unsupported method") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeminiGenerateRejectsEmptyPromptBeforeStartingAdapter(t *testing.T) {
	_, err := NewGeminiClient("/missing/node", "/missing/adapter.js").Generate(GeminiGenerateRequest{Prompt: "  "})
	if err != ErrGeminiMissingPrompt {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeminiGenerateRoundTripsThroughAdapterProcess(t *testing.T) {
	adapter := writeGeminiAdapter(t, `
read line
case "$line" in
  *'"prompt":"hello"'*) ;;
  *) printf '%s\n' '{"id":"1","error":{"code":-32602,"message":"bad prompt"}}'; exit 0 ;;
esac
printf '%s\n' '{"id":"1","result":{"text":"OK","provider":"test-adapter","model":"gemini-test"}}'
`)
	response, err := NewGeminiClient("/bin/sh", adapter).Generate(GeminiGenerateRequest{
		Prompt: "hello",
		Model:  "gemini-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "OK" || response.Provider != "test-adapter" || response.Model != "gemini-test" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestGeminiModelsRoundTripsThroughAdapterProcess(t *testing.T) {
	adapter := writeGeminiAdapter(t, `
read line
case "$line" in
  *'"method":"models"'*) ;;
  *) printf '%s\n' '{"id":"1","error":{"code":-32601,"message":"bad method"}}'; exit 0 ;;
esac
printf '%s\n' '{"id":"1","result":{"provider":"gemini-cli-core","releaseChannel":"stable","models":[{"id":"auto","name":"Auto","description":"Let Gemini CLI decide","tier":"auto","source":"gemini-cli-core","quota":null}]}}'
`)
	response, err := NewGeminiClient("/bin/sh", adapter).Models()
	if err != nil {
		t.Fatal(err)
	}
	if response.Provider != "gemini-cli-core" || response.ReleaseChannel != "stable" || len(response.Models) != 1 || response.Models[0].ID != "auto" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func writeGeminiAdapter(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "adapter.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
