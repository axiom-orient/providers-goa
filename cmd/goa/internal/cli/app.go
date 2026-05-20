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
	globalJSON := false
	if len(args) > 0 && args[0] == "--json" {
		globalJSON = true
		args = args[1:]
	}
	if len(args) == 0 {
		a.printRootHelp()
		return 0
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.printRootHelp()
		return 0
	case "codex":
		return a.runCodex(ctx, args[1:], globalJSON)
	case "gemini":
		return a.runGemini(args[1:], globalJSON)
	case "version":
		return a.runVersion(args[1:])
	default:
		fmt.Fprintf(a.Stderr, "unknown provider: %s\n\n", args[0])
		a.printRootHelp()
		return 2
	}
}

func (a *App) printRootHelp() {
	fmt.Fprint(a.Stdout, `goa

Usage:
  goa [--json] codex [--auth-path <path>] [--auth-home <dir>] [--base-url <url>] [--client-version <version>] <command>
  goa [--json] gemini <command>
  goa version [--json]

Commands:
  codex send         Send a prompt with the resolved auth.json credential
  codex models list  List Codex models
  codex auth status  Inspect resolved auth.json state
  codex relogin      Refresh auth.json through browser OAuth
  gemini generate    Generate text through Gemini CLI Core
  gemini models      List Gemini models through Gemini CLI Core
  version            Print build version metadata

Rules:
  codex uses ChatGPT/Codex credentials from resolved auth.json
  gemini uses package-local gemini-core-adapter and Gemini CLI Core auth
`)
}
