package appserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/axiom-orient/providers-goa/appserver"
)

type rpcMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func TestNewClientAccountReadAndNotifications(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	done := make(chan error, 1)
	go func() {
		defer close(done)
		defer serverConn.Close()
		dec := json.NewDecoder(serverConn)
		enc := json.NewEncoder(serverConn)

		var msg rpcMessage
		if err := dec.Decode(&msg); err != nil {
			done <- err
			return
		}
		if msg.Method != "initialize" {
			done <- errors.New("expected initialize request")
			return
		}
		if err := enc.Encode(map[string]any{"id": 0, "result": map[string]any{"userAgent": "codex-app-server/1.0", "platformFamily": "unix", "platformOs": "linux"}}); err != nil {
			done <- err
			return
		}
		if err := dec.Decode(&msg); err != nil {
			done <- err
			return
		}
		if msg.Method != "initialized" {
			done <- errors.New("expected initialized notification")
			return
		}
		if err := dec.Decode(&msg); err != nil {
			done <- err
			return
		}
		if msg.Method != "account/read" {
			done <- errors.New("expected account/read request")
			return
		}
		if err := enc.Encode(map[string]any{"id": 1, "result": map[string]any{"account": map[string]any{"type": "chatgpt", "email": "user@example.com", "planType": "pro"}, "requiresOpenaiAuth": true}}); err != nil {
			done <- err
			return
		}
		if err := enc.Encode(map[string]any{"method": "account/updated", "params": map[string]any{"authMode": "chatgpt", "planType": "pro"}}); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	client, err := appserver.NewClient(clientConn, appserver.ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	initResult := client.InitializeResult()
	if initResult.PlatformOS != "linux" {
		t.Fatalf("unexpected initialize result: %+v", initResult)
	}

	result, err := client.AccountRead(context.Background(), false)
	if err != nil {
		t.Fatalf("AccountRead() error = %v", err)
	}
	if result.Account == nil || result.Account.Type != "chatgpt" || result.Account.PlanType != "pro" {
		t.Fatalf("unexpected account result: %+v", result)
	}
	if !result.RequiresOpenAIAuth {
		t.Fatal("expected requiresOpenaiAuth=true")
	}

	select {
	case notification := <-client.Notifications():
		payload, ok, err := notification.AccountUpdated()
		if err != nil {
			t.Fatalf("AccountUpdated() error = %v", err)
		}
		if !ok {
			t.Fatalf("unexpected notification: %+v", notification)
		}
		if payload.AuthMode == nil || *payload.AuthMode != "chatgpt" {
			t.Fatalf("unexpected account/updated payload: %+v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for notification")
	}

	if err := <-done; err != nil {
		t.Fatalf("server error: %v", err)
	}
}

func TestLoginWithChatGPTAuthTokensHandlesRefreshRequest(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()

	refreshSeen := make(chan appserver.ChatGPTAuthTokensRefreshRequest, 1)
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		defer serverConn.Close()
		dec := json.NewDecoder(serverConn)
		enc := json.NewEncoder(serverConn)

		var msg rpcMessage
		if err := dec.Decode(&msg); err != nil {
			serverDone <- err
			return
		}
		if err := enc.Encode(map[string]any{"id": 0, "result": map[string]any{"userAgent": "codex-app-server/1.0"}}); err != nil {
			serverDone <- err
			return
		}
		if err := dec.Decode(&msg); err != nil {
			serverDone <- err
			return
		}
		if msg.Method != "initialized" {
			serverDone <- errors.New("expected initialized notification")
			return
		}
		if err := dec.Decode(&msg); err != nil {
			serverDone <- err
			return
		}
		if msg.Method != "account/login/start" {
			serverDone <- errors.New("expected account/login/start")
			return
		}
		if err := enc.Encode(map[string]any{"id": 1, "result": map[string]any{"type": "chatgptAuthTokens"}}); err != nil {
			serverDone <- err
			return
		}
		if err := enc.Encode(map[string]any{"method": "account/chatgptAuthTokens/refresh", "id": 99, "params": map[string]any{"reason": "unauthorized", "previousAccountId": "org-123"}}); err != nil {
			serverDone <- err
			return
		}
		if err := dec.Decode(&msg); err != nil {
			serverDone <- err
			return
		}
		if msg.Error != nil {
			serverDone <- errors.New(msg.Error.Message)
			return
		}
		if string(msg.ID) != "99" {
			serverDone <- errors.New("unexpected refresh response id")
			return
		}
		var tokens appserver.ChatGPTAuthTokens
		if err := json.Unmarshal(msg.Result, &tokens); err != nil {
			serverDone <- err
			return
		}
		if tokens.IDToken != "fresh-id" || tokens.AccessToken != "fresh-access" {
			serverDone <- errors.New("unexpected refresh tokens")
			return
		}
		serverDone <- nil
	}()

	client, err := appserver.NewClient(clientConn, appserver.ClientOptions{
		RefreshHandler: func(ctx context.Context, req appserver.ChatGPTAuthTokensRefreshRequest) (appserver.ChatGPTAuthTokens, error) {
			select {
			case refreshSeen <- req:
			default:
			}
			return appserver.ChatGPTAuthTokens{IDToken: "fresh-id", AccessToken: "fresh-access"}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	result, err := client.LoginWithChatGPTAuthTokens(context.Background(), "old-id", "old-access")
	if err != nil {
		t.Fatalf("LoginWithChatGPTAuthTokens() error = %v", err)
	}
	if result.Type != "chatgptAuthTokens" {
		t.Fatalf("unexpected login result: %+v", result)
	}
	select {
	case req := <-refreshSeen:
		if req.Reason != "unauthorized" || req.PreviousAccountID != "org-123" {
			t.Fatalf("unexpected refresh request: %+v", req)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for refresh handler")
	}
	if err := <-serverDone; err != nil {
		t.Fatalf("server error: %v", err)
	}
}

func TestDialStdioAccountRead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := appserver.DialStdio(ctx, appserver.StdioConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--", "stdio-account-read"},
		Env:     append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"),
	})
	if err != nil {
		t.Fatalf("DialStdio() error = %v", err)
	}
	defer client.Close()

	result, err := client.AccountRead(context.Background(), false)
	if err != nil {
		t.Fatalf("AccountRead() error = %v", err)
	}
	if result.Account == nil || result.Account.Type != "apiKey" {
		t.Fatalf("unexpected account result: %+v", result)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if len(os.Args) < 1 || os.Args[len(os.Args)-1] != "stdio-account-read" {
		os.Exit(0)
	}
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var msg rpcMessage
	if err := dec.Decode(&msg); err != nil {
		os.Exit(1)
	}
	if err := enc.Encode(map[string]any{"id": 0, "result": map[string]any{"userAgent": "codex-app-server/1.0"}}); err != nil {
		os.Exit(1)
	}
	if err := dec.Decode(&msg); err != nil {
		os.Exit(1)
	}
	if err := dec.Decode(&msg); err != nil {
		os.Exit(1)
	}
	if err := enc.Encode(map[string]any{"id": 1, "result": map[string]any{"account": map[string]any{"type": "apiKey"}, "requiresOpenaiAuth": true}}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
