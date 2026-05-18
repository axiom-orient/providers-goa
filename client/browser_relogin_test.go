package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBrowserReloginCompletesCallbackExchangeAndPersistsAuthFile(t *testing.T) {
	tokenCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		tokenCalls++
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			if got := r.Form.Get("code"); got != "ok" {
				t.Fatalf("unexpected code: %q", got)
			}
			if got := r.Form.Get("client_id"); got != "client_123" {
				t.Fatalf("unexpected client_id: %q", got)
			}
			if strings.TrimSpace(r.Form.Get("code_verifier")) == "" {
				t.Fatalf("missing code_verifier")
			}
			fmt.Fprintf(w, `{"id_token":%q,"access_token":"access_new","refresh_token":"refresh_new"}`, fakeReloginIDToken("ws_123"))
		case "urn:ietf:params:oauth:grant-type:token-exchange":
			if got := r.Form.Get("requested_token"); got != "openai-api-key" {
				t.Fatalf("unexpected requested_token: %q", got)
			}
			fmt.Fprint(w, `{"access_token":"sk-from-relogin"}`)
		default:
			t.Fatalf("unexpected grant_type: %q", r.Form.Get("grant_type"))
		}
	}))
	defer server.Close()

	authPath := filepath.Join(t.TempDir(), "auth.json")
	client, err := NewClient(Config{
		AuthPath:      authPath,
		AuthIssuerURL: server.URL,
		OAuthClientID: "client_123",
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}

	opts := DefaultBrowserReloginOptions()
	opts.NoBrowser = true
	opts.CallbackPort = 0
	opts.Timeout = 5 * time.Second
	opts.AllowedWorkspaceID = "ws_123"

	session, err := client.StartBrowserReloginSession(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(session.AuthURL)
	if err != nil {
		t.Fatal(err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatal("missing state in auth URL")
	}

	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/auth/callback?state=%s&code=ok", session.CallbackPort, url.QueryEscape(state))
	resp, err := http.Get(callbackURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected callback status: %d", resp.StatusCode)
	}

	outcome, err := session.Wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.PersistedTo != authPath {
		t.Fatalf("unexpected persisted path: %q", outcome.PersistedTo)
	}
	if !outcome.AuthState.HasAPIKey || outcome.AuthState.APIKeySource != APIKeySourceAuthFile {
		t.Fatalf("unexpected auth state: %#v", outcome.AuthState)
	}
	if tokenCalls != 2 {
		t.Fatalf("expected two token endpoint calls, got %d", tokenCalls)
	}
	raw, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"sk-from-relogin", "access_new", "refresh_new", "ws_123", "auth_mode"} {
		if !strings.Contains(text, want) {
			t.Fatalf("persisted auth file missing %q: %s", want, text)
		}
	}
}

func fakeReloginIDToken(accountID string) string {
	payload := fmt.Sprintf(`{"https://api.openai.com/auth":{"chatgpt_account_id":%q}}`, accountID)
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}
