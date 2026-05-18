package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"

	goa "github.com/axiom-orient/providers-goa/client"
)

type authStatusInput struct {
	authPath string
	authHome string
	asJSON   bool
}

func (a *App) runAuth(_ context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "missing subcommand: auth status")
		return 2
	}
	switch args[0] {
	case "status":
		input, err := parseAuthStatusInput(a.Stderr, args[1:])
		if err != nil {
			return 2
		}
		state, err := executeAuthStatus(input)
		if err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		return a.renderAuthStatus(state, input.asJSON)
	default:
		fmt.Fprintf(a.Stderr, "unknown auth subcommand: %s\n", args[0])
		return 2
	}
}

func parseAuthStatusInput(stderr io.Writer, args []string) (authStatusInput, error) {
	fs := flag.NewFlagSet("goa auth status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	authPath := fs.String("auth-path", "", "path to auth.json")
	authHome := fs.String("auth-home", "", "directory containing auth.json")
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return authStatusInput{}, err
	}
	return authStatusInput{
		authPath: *authPath,
		authHome: *authHome,
		asJSON:   *asJSON,
	}, nil
}

func executeAuthStatus(input authStatusInput) (goa.AuthState, error) {
	client, err := goa.NewClient(goa.Config{AuthPath: input.authPath, AuthHome: input.authHome})
	if err != nil {
		return goa.AuthState{}, err
	}
	return client.AuthState(), nil
}

func (a *App) renderAuthStatus(state goa.AuthState, asJSON bool) int {
	if asJSON {
		return a.writeJSON(state)
	}
	fmt.Fprintf(a.Stdout, "auth_path: %s\n", state.AuthPath)
	fmt.Fprintf(a.Stdout, "auth_file_found: %t\n", state.AuthFileFound)
	fmt.Fprintf(a.Stdout, "has_api_key: %t\n", state.HasAPIKey)
	fmt.Fprintf(a.Stdout, "api_key_source: %s\n", state.APIKeySource)
	if len(state.AuthFileKeys) > 0 {
		keys := append([]string(nil), state.AuthFileKeys...)
		sort.Strings(keys)
		fmt.Fprintf(a.Stdout, "auth_file_keys: %v\n", keys)
	}
	if state.AuthFileKnownAPIKeyRef != "" {
		fmt.Fprintf(a.Stdout, "auth_file_known_api_key_ref: %s\n", state.AuthFileKnownAPIKeyRef)
	}
	if state.AuthFileReadError != "" {
		fmt.Fprintf(a.Stdout, "auth_file_read_error: %s\n", state.AuthFileReadError)
	}
	if state.AuthFileParseError != "" {
		fmt.Fprintf(a.Stdout, "auth_file_parse_error: %s\n", state.AuthFileParseError)
	}
	return 0
}
