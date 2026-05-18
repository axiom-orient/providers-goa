package cli

import (
	"context"
	"fmt"
	"io"
)

// App is the CLI adapter.
type App struct {
	Stdout io.Writer
	Stderr io.Writer
}

// New constructs a CLI app.
func New(stdout, stderr io.Writer) *App {
	return &App{Stdout: stdout, Stderr: stderr}
}

// Run executes the CLI and returns a process exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		a.printRootHelp()
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.printRootHelp()
		return 0
	case "auth":
		return a.runAuth(ctx, args[1:])
	case "relogin":
		return a.runRelogin(ctx, args[1:])
	case "version":
		return a.runVersion(args[1:])
	case "models":
		return a.runModels(ctx, args[1:])
	case "responses":
		return a.runResponses(ctx, args[1:])
	case "send":
		return a.runSend(ctx, args[1:])
	default:
		fmt.Fprintf(a.Stderr, "unknown command: %s\n\n", args[0])
		a.printRootHelp()
		return 2
	}
}

func (a *App) printRootHelp() {
	fmt.Fprint(a.Stdout, `goa

Usage:
  goa auth status [--auth-path <path>] [--auth-home <dir>] [--json]
  goa relogin [--no-browser] [--callback-port <port>] [--timeout-seconds <seconds>] [--persist-path <path>] [--issuer <url>] [--client-id <id>] [--allowed-workspace-id <id>] [--auth-path <path>] [--auth-home <dir>] [--json]
  goa version [--json]
  goa models list [--api-key <key>] [--base-url <url>] [--organization <org>] [--project <project>] [--client-request-id <id>] [--auth-path <path>] [--auth-home <dir>] [--json]
  goa responses create --model <model-id> --input <text> [--instructions <text>] [--stream] [--api-key <key>] [--base-url <url>] [--organization <org>] [--project <project>] [--client-request-id <id>] [--auth-path <path>] [--auth-home <dir>] [--json]
  goa send --model <model-id> --input <text> [--instructions <text>] [--stream] [--api-key <key>] [--base-url <url>] [--organization <org>] [--project <project>] [--client-request-id <id>] [--auth-path <path>] [--auth-home <dir>] [--json]

Commands:
  auth status        Inspect resolved auth state
  relogin            Refresh auth cache through browser OAuth
  version            Print build version metadata
  models list        Call GET /v1/models
  responses create   Call POST /v1/responses
  send               Alias for responses create
`)
}
