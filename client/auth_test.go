package client

import (
	"path/filepath"
	"testing"
)

func writeTempAuthFile(t *testing.T, payload string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := osWriteFile(path, []byte(payload)); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveAuthPathPrecedence(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "codex-home")
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", home)

	got, err := ResolveAuthPath(ResolveAuthOptions{AuthPath: "/tmp/custom/auth.json"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Clean("/tmp/custom/auth.json"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	got, err = ResolveAuthPath(ResolveAuthOptions{AuthHome: "/tmp/auth-home"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/tmp/auth-home", "auth.json"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}

	got, err = ResolveAuthPath(ResolveAuthOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(codexHome, "auth.json"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestResolveAuthPathFallsBackToHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", home)

	got, err := ResolveAuthPath(ResolveAuthOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".codex", "auth.json"); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestInspectAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	payload := `{"openai":{"api_key":"sk-test"},"profile":"default"}`
	if err := osWriteFile(path, []byte(payload)); err != nil {
		t.Fatal(err)
	}

	summary := InspectAuthFile(path)
	if !summary.Exists {
		t.Fatal("expected auth file to exist")
	}
	if !summary.HasKnownAPIKey {
		t.Fatal("expected known api key field")
	}
	if summary.KnownAPIKeyRef != "openai.api_key" {
		t.Fatalf("unexpected key ref: %q", summary.KnownAPIKeyRef)
	}
	if len(summary.TopLevelKeys) != 2 {
		t.Fatalf("unexpected keys: %#v", summary.TopLevelKeys)
	}
}

func TestInspectAuthFileParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := osWriteFile(path, []byte(`{"broken":`)); err != nil {
		t.Fatal(err)
	}

	summary := InspectAuthFile(path)
	if !summary.Exists {
		t.Fatal("expected auth file to exist")
	}
	if summary.ParseError == "" {
		t.Fatal("expected parse error")
	}
}

func TestInspectAuthFileMissingDoesNotReportReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-auth.json")
	summary := InspectAuthFile(path)
	if summary.Exists {
		t.Fatal("expected auth file to be absent")
	}
	if summary.ReadError != "" {
		t.Fatalf("expected no read error, got %q", summary.ReadError)
	}
}

func TestAPIKeyResolutionPrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := osWriteFile(path, []byte(`{"api_key":"sk-auth"}`)); err != nil {
		t.Fatal(err)
	}

	t.Setenv(defaultAPIKeyEnv, "sk-env")
	client, err := NewClient(Config{AuthPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got := client.AuthState().APIKeySource; got != APIKeySourceEnv {
		t.Fatalf("want env source, got %q", got)
	}

	client, err = NewClient(Config{AuthPath: path, APIKey: "sk-config"})
	if err != nil {
		t.Fatal(err)
	}
	if got := client.AuthState().APIKeySource; got != APIKeySourceConfig {
		t.Fatalf("want config source, got %q", got)
	}

	t.Setenv(defaultAPIKeyEnv, "")
	client, err = NewClient(Config{AuthPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if got := client.AuthState().APIKeySource; got != APIKeySourceAuthFile {
		t.Fatalf("want auth_file source, got %q", got)
	}
}

func TestChatGPTTokenResolutionFromAuthFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := osWriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"tok-123","account_id":"acc-456"}}`)); err != nil {
		t.Fatal(err)
	}

	client, err := NewClient(Config{AuthPath: path})
	if err != nil {
		t.Fatal(err)
	}
	state := client.AuthState()
	if state.Transport != "chatgpt" {
		t.Fatalf("transport = %q, want chatgpt", state.Transport)
	}
	if !state.HasAccessToken {
		t.Fatal("expected access token")
	}
	if !state.HasAccountID {
		t.Fatal("expected account id")
	}
}
