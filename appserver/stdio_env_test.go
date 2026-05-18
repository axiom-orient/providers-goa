package appserver_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/axiom-orient/providers-goa/appserver"
)

func TestDialStdioInheritsParentEnvironmentWhenEnvUnset(t *testing.T) {
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("GOA_INHERITED_ENV_TEST", "present")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := appserver.DialStdio(ctx, appserver.StdioConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcessEnvInheritance", "--", "stdio-inherit-env"},
	})
	if err != nil {
		t.Fatalf("DialStdio() error = %v", err)
	}
	defer client.Close()

	if client.InitializeResult().UserAgent != "codex-app-server/1.0" {
		t.Fatalf("unexpected initialize result: %+v", client.InitializeResult())
	}
}

func TestHelperProcessEnvInheritance(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if len(os.Args) < 1 || os.Args[len(os.Args)-1] != "stdio-inherit-env" {
		return
	}
	if os.Getenv("GOA_INHERITED_ENV_TEST") != "present" {
		os.Exit(2)
	}
	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(os.Stdout)
	var msg rpcMessage
	if err := dec.Decode(&msg); err != nil {
		os.Exit(1)
	}
	if msg.Method != "initialize" {
		os.Exit(1)
	}
	if err := enc.Encode(map[string]any{"id": 0, "result": map[string]any{"userAgent": "codex-app-server/1.0"}}); err != nil {
		os.Exit(1)
	}
	if err := dec.Decode(&msg); err != nil {
		os.Exit(1)
	}
	if msg.Method != "initialized" {
		os.Exit(1)
	}
	os.Exit(0)
}
