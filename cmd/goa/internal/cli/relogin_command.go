package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	goa "github.com/axiom-orient/providers-goa/client"
)

type reloginInput struct {
	authPath           string
	authHome           string
	issuer             string
	clientID           string
	persistPath        string
	allowedWorkspaceID string
	callbackPort       int
	timeoutSeconds     int
	noBrowser          bool
	asJSON             bool
}

func (a *App) runRelogin(ctx context.Context, args []string) int {
	input, err := parseReloginInput(a.Stderr, args)
	if err != nil {
		return 2
	}
	if err := executeRelogin(ctx, input, a); err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return 0
}

func parseReloginInput(stderr io.Writer, args []string) (reloginInput, error) {
	fs := flag.NewFlagSet("goa relogin", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := reloginInput{callbackPort: 1455, timeoutSeconds: 180}
	fs.StringVar(&input.authPath, "auth-path", "", "path to auth.json")
	fs.StringVar(&input.authHome, "auth-home", "", "directory containing auth.json")
	fs.StringVar(&input.issuer, "issuer", "", "OAuth issuer URL")
	fs.StringVar(&input.clientID, "client-id", "", "OAuth client id")
	fs.StringVar(&input.persistPath, "persist-path", "", "path to write relogin auth.json")
	fs.StringVar(&input.allowedWorkspaceID, "allowed-workspace-id", "", "restrict login to a ChatGPT workspace/account id")
	fs.IntVar(&input.callbackPort, "callback-port", 1455, "localhost callback port; use 0 for an ephemeral port")
	fs.IntVar(&input.timeoutSeconds, "timeout-seconds", 180, "seconds to wait for the browser callback")
	fs.BoolVar(&input.noBrowser, "no-browser", false, "print auth URL without opening the browser")
	fs.BoolVar(&input.asJSON, "json", false, "emit JSON lines")
	if err := fs.Parse(args); err != nil {
		return reloginInput{}, err
	}
	if fs.NArg() != 0 {
		return reloginInput{}, fmt.Errorf("unexpected relogin arguments: %v", fs.Args())
	}
	if input.timeoutSeconds <= 0 {
		return reloginInput{}, fmt.Errorf("--timeout-seconds must be greater than zero")
	}
	return input, nil
}

func executeRelogin(ctx context.Context, input reloginInput, app *App) error {
	client, err := goa.NewClient(goa.Config{
		AuthPath:      input.authPath,
		AuthHome:      input.authHome,
		AuthIssuerURL: input.issuer,
		OAuthClientID: input.clientID,
	})
	if err != nil {
		return err
	}
	opts := goa.DefaultBrowserReloginOptions()
	opts.NoBrowser = input.noBrowser
	opts.CallbackPort = input.callbackPort
	opts.Timeout = time.Duration(input.timeoutSeconds) * time.Second
	opts.PersistPath = input.persistPath
	opts.Issuer = input.issuer
	opts.ClientID = input.clientID
	opts.AllowedWorkspaceID = input.allowedWorkspaceID

	session, err := client.StartBrowserReloginSession(ctx, opts)
	if err != nil {
		return err
	}
	started := map[string]any{
		"event":          "relogin_started",
		"auth_url":       session.AuthURL,
		"callback_url":   fmt.Sprintf("http://localhost:%d/auth/callback", session.CallbackPort),
		"callback_port":  session.CallbackPort,
		"browser_opened": !input.noBrowser,
	}
	if input.asJSON {
		if code := app.writeJSON(started); code != 0 {
			return fmt.Errorf("failed to write JSON output")
		}
	} else {
		fmt.Fprintf(app.Stdout, "Starting browser re-login.\nOpen this URL to continue:\n%s\nWaiting for callback on http://localhost:%d/auth/callback\n", session.AuthURL, session.CallbackPort)
	}

	outcome, err := session.Wait(ctx)
	if err != nil {
		return err
	}
	if input.asJSON {
		completed := map[string]any{
			"event":             "relogin_completed",
			"auth_url":          outcome.AuthURL,
			"callback_port":     outcome.CallbackPort,
			"persisted_to":      outcome.PersistedTo,
			"transport":         outcome.AuthState.Transport,
			"has_refresh_token": outcome.AuthState.HasRefreshToken,
			"has_account_id":    outcome.AuthState.HasAccountID,
		}
		if code := app.writeJSON(completed); code != 0 {
			return fmt.Errorf("failed to write JSON output")
		}
		return nil
	}
	fmt.Fprintf(app.Stdout, "relogin=completed\npersisted_to=%s\ntransport=%s\nhas_refresh_token=%t\n", outcome.PersistedTo, outcome.AuthState.Transport, outcome.AuthState.HasRefreshToken)
	return nil
}
