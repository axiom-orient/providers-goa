package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRewriteRefreshedChatGPTAuthFilePreservesUnrelatedFields(t *testing.T) {
	raw := []byte(`{
  "auth_mode": "chatgpt",
  "profile": "default",
  "tokens": {
    "access_token": "old-access",
    "refresh_token": "old-refresh",
    "custom": "keep-me"
  },
  "settings": {
    "theme": "dark"
  }
}`)

	rewritten, err := rewriteRefreshedChatGPTAuthFile(raw, refreshedChatGPTTokens{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		IDToken:      "new-id",
		AccountID:    "acc-123",
	}, time.Date(2026, 4, 20, 7, 8, 9, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rewritten, &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["profile"]; got != "default" {
		t.Fatalf("unexpected profile: %#v", got)
	}
	settings, ok := payload["settings"].(map[string]any)
	if !ok || settings["theme"] != "dark" {
		t.Fatalf("unexpected settings payload: %#v", payload["settings"])
	}
	tokens, ok := payload["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected tokens payload: %#v", payload["tokens"])
	}
	if got := tokens["access_token"]; got != "new-access" {
		t.Fatalf("unexpected access token: %#v", got)
	}
	if got := tokens["refresh_token"]; got != "new-refresh" {
		t.Fatalf("unexpected refresh token: %#v", got)
	}
	if got := tokens["id_token"]; got != "new-id" {
		t.Fatalf("unexpected id token: %#v", got)
	}
	if got := tokens["account_id"]; got != "acc-123" {
		t.Fatalf("unexpected account id: %#v", got)
	}
	if got := tokens["custom"]; got != "keep-me" {
		t.Fatalf("unexpected custom token payload: %#v", got)
	}
	if got := payload["last_refresh"]; got != "2026-04-20T07:08:09Z" {
		t.Fatalf("unexpected last_refresh: %#v", got)
	}
}

func TestPersistRefreshedChatGPTTokensLocksAndWritesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := osWriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"old","refresh_token":"old-refresh"},"profile":"default"}`)); err != nil {
		t.Fatal(err)
	}

	if err := persistRefreshedChatGPTTokens(path, "new-access", "new-refresh", "new-id", "acc-123"); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload["profile"]; got != "default" {
		t.Fatalf("unexpected profile: %#v", got)
	}
	tokens, ok := payload["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected tokens payload: %#v", payload["tokens"])
	}
	if got := tokens["access_token"]; got != "new-access" {
		t.Fatalf("unexpected access token: %#v", got)
	}
	if got := tokens["refresh_token"]; got != "new-refresh" {
		t.Fatalf("unexpected refresh token: %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perms := info.Mode().Perm(); perms != 0o600 {
		t.Fatalf("unexpected auth file permissions: %o", perms)
	}
	lockInfo, err := os.Stat(path + ".refresh.lock")
	if err != nil {
		t.Fatal(err)
	}
	if perms := lockInfo.Mode().Perm(); perms != 0o600 {
		t.Fatalf("unexpected lock file permissions: %o", perms)
	}
}
