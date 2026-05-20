package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	goa "github.com/axiom-orient/providers-goa/client"
)

type codexInput struct {
	client        clientFlags
	defaultModel  string
	defaultEffort string
	asJSON        bool
}

func (a *App) runCodex(ctx context.Context, args []string, globalJSON bool) int {
	fs := flag.NewFlagSet("goa codex", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	flags := bindClientFlags(fs, true)
	defaultModel := fs.String("default-model", "", "default request model")
	defaultEffort := fs.String("default-effort", "", "default reasoning effort")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(a.Stderr, "codex requires a command")
		return 2
	}
	input := codexInput{
		client:        *flags,
		defaultModel:  *defaultModel,
		defaultEffort: *defaultEffort,
		asJSON:        globalJSON,
	}
	switch rest[0] {
	case "send":
		return a.runCodexSend(ctx, input, rest[1:])
	case "models":
		return a.runCodexModels(ctx, input, rest[1:])
	case "auth":
		return a.runCodexAuth(input, rest[1:])
	case "relogin":
		return a.runCodexRelogin(ctx, input, rest[1:])
	default:
		fmt.Fprintf(a.Stderr, "unknown codex command %s\n", rest[0])
		return 2
	}
}

type codexSendInput struct {
	model        string
	effort       string
	prompt       string
	stdin        bool
	stream       bool
	instructions string
	asJSON       bool
	client       clientFlags
}

func (a *App) runCodexSend(ctx context.Context, codex codexInput, args []string) int {
	input, err := parseCodexSendInput(codex, args)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 2
	}
	client, ctx, err := executeCodexClient(ctx, input.client)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	prompt := input.prompt
	if input.stdin {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		prompt = string(raw)
	}
	request := goa.CreateResponseRequest{
		Model:        input.model,
		Input:        prompt,
		Instructions: input.instructions,
	}
	if input.effort != "" {
		request.Reasoning = map[string]any{"effort": input.effort}
	}
	if input.stream {
		return a.runStreamingResponse(ctx, client, request, input.asJSON)
	}
	resp, err := executeResponseCreate(ctx, client, request)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return a.renderResponseCreate(resp, input.asJSON)
}

func parseCodexSendInput(codex codexInput, args []string) (codexSendInput, error) {
	input := codexSendInput{
		model:  codex.defaultModel,
		effort: codex.defaultEffort,
		asJSON: codex.asJSON,
		client: codex.client,
	}
	for len(args) > 0 {
		current := args[0]
		args = args[1:]
		switch current {
		case "--model":
			if len(args) == 0 {
				return input, errors.New("missing value for --model")
			}
			input.model = args[0]
			args = args[1:]
		case "--effort":
			if len(args) == 0 {
				return input, errors.New("missing value for --effort")
			}
			input.effort = args[0]
			args = args[1:]
		case "--instructions":
			if len(args) == 0 {
				return input, errors.New("missing value for --instructions")
			}
			input.instructions = args[0]
			args = args[1:]
		case "--stdin":
			input.stdin = true
		case "--stream":
			input.stream = true
		case "--json":
			input.asJSON = true
		default:
			if strings.HasPrefix(current, "--") {
				return input, fmt.Errorf("unknown codex send option %s", current)
			}
			if input.prompt == "" {
				input.prompt = current
			} else {
				input.prompt += " " + current
			}
		}
	}
	if input.prompt == "" && !input.stdin {
		return input, errors.New("codex send requires a prompt or --stdin")
	}
	if input.prompt != "" && input.stdin {
		return input, errors.New("prompt argument and --stdin cannot be used together")
	}
	return input, nil
}

func (a *App) runCodexModels(ctx context.Context, codex codexInput, args []string) int {
	if len(args) != 1 || args[0] != "list" {
		fmt.Fprintln(a.Stderr, "codex models requires the subcommand list")
		return 2
	}
	ctx = codex.client.context(ctx)
	client, err := goa.NewClient(codex.client.config())
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	resp, err := client.Models().List(ctx)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return a.renderModelsList(resp, codex.asJSON)
}

func (a *App) runCodexAuth(codex codexInput, args []string) int {
	if len(args) != 1 || args[0] != "status" {
		fmt.Fprintln(a.Stderr, "codex auth requires the subcommand status")
		return 2
	}
	client, err := goa.NewClient(codex.client.config())
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return a.renderCodexAuthStatus(client.AuthState(), codex.asJSON)
}

func (a *App) renderCodexAuthStatus(state goa.AuthState, asJSON bool) int {
	shape := "unknown"
	readiness := "auth_refresh_required"
	if state.HasAccessToken || state.HasRefreshToken {
		shape = "chatgpt_managed"
		readiness = "ready_chatgpt"
	}
	view := map[string]any{
		"auth_path":         state.AuthPath,
		"credential_shape":  shape,
		"readiness":         readiness,
		"transport":         state.Transport,
		"has_refresh_token": state.HasRefreshToken,
		"has_account_id":    state.HasAccountID,
	}
	if asJSON {
		return a.writeJSON(view)
	}
	fmt.Fprintf(a.Stdout, "auth_path=%s\n", view["auth_path"])
	fmt.Fprintf(a.Stdout, "credential_shape=%s\n", view["credential_shape"])
	fmt.Fprintf(a.Stdout, "readiness=%s\n", view["readiness"])
	fmt.Fprintf(a.Stdout, "transport=%s\n", view["transport"])
	fmt.Fprintf(a.Stdout, "has_refresh_token=%t\n", view["has_refresh_token"])
	fmt.Fprintf(a.Stdout, "has_account_id=%t\n", view["has_account_id"])
	return 0
}

func (a *App) runCodexRelogin(ctx context.Context, codex codexInput, args []string) int {
	input, err := parseCodexReloginInput(codex, args)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 2
	}
	if err := executeRelogin(ctx, input, a); err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return 0
}

func parseCodexReloginInput(codex codexInput, args []string) (reloginInput, error) {
	input := reloginInput{
		authPath:       codex.client.authPath,
		authHome:       codex.client.authHome,
		issuer:         codex.client.issuer,
		callbackPort:   1455,
		timeoutSeconds: 180,
		asJSON:         codex.asJSON,
	}
	for len(args) > 0 {
		current := args[0]
		args = args[1:]
		switch current {
		case "--no-browser":
			input.noBrowser = true
		case "--json":
			input.asJSON = true
		case "--callback-port":
			port, rest, err := parsePositiveIntFlag("--callback-port", args)
			if err != nil {
				return input, err
			}
			input.callbackPort = port
			args = rest
		case "--timeout-seconds":
			seconds, rest, err := parsePositiveIntFlag("--timeout-seconds", args)
			if err != nil {
				return input, err
			}
			input.timeoutSeconds = seconds
			args = rest
		case "--persist-path":
			value, rest, err := parseStringFlag("--persist-path", args)
			if err != nil {
				return input, err
			}
			input.persistPath = value
			args = rest
		case "--issuer":
			value, rest, err := parseStringFlag("--issuer", args)
			if err != nil {
				return input, err
			}
			input.issuer = value
			args = rest
		case "--client-id":
			value, rest, err := parseStringFlag("--client-id", args)
			if err != nil {
				return input, err
			}
			input.clientID = value
			args = rest
		case "--allowed-workspace-id":
			value, rest, err := parseStringFlag("--allowed-workspace-id", args)
			if err != nil {
				return input, err
			}
			input.allowedWorkspaceID = value
			args = rest
		default:
			return input, fmt.Errorf("unknown relogin option %s", current)
		}
	}
	return input, nil
}

func executeCodexClient(ctx context.Context, flags clientFlags) (*goa.Client, context.Context, error) {
	ctx = flags.context(ctx)
	client, err := goa.NewClient(flags.config())
	if err != nil {
		return nil, nil, err
	}
	return client, ctx, nil
}

func parseStringFlag(name string, args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", args, fmt.Errorf("missing value for %s", name)
	}
	return args[0], args[1:], nil
}

func parsePositiveIntFlag(name string, args []string) (int, []string, error) {
	value, rest, err := parseStringFlag(name, args)
	if err != nil {
		return 0, args, err
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return 0, args, fmt.Errorf("%s must be an integer", name)
	}
	if parsed < 0 {
		return 0, args, fmt.Errorf("%s must be zero or greater", name)
	}
	return parsed, rest, nil
}
