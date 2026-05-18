package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	goa "github.com/axiom-orient/providers-goa/client"
)

type modelsListInput struct {
	client clientFlags
	asJSON bool
}

func (a *App) runModels(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "missing subcommand: models list")
		return 2
	}
	switch args[0] {
	case "list":
		input, err := parseModelsListInput(a.Stderr, args[1:])
		if err != nil {
			return 2
		}
		resp, err := executeModelsList(ctx, input)
		if err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		return a.renderModelsList(resp, input.asJSON)
	default:
		fmt.Fprintf(a.Stderr, "unknown models subcommand: %s\n", args[0])
		return 2
	}
}

func parseModelsListInput(stderr io.Writer, args []string) (modelsListInput, error) {
	fs := flag.NewFlagSet("goa models list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := bindClientFlags(fs, true)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return modelsListInput{}, err
	}
	return modelsListInput{client: *flags, asJSON: *asJSON}, nil
}

func executeModelsList(ctx context.Context, input modelsListInput) (goa.ListModelsResponse, error) {
	ctx = input.client.context(ctx)
	client, err := goa.NewClient(input.client.config())
	if err != nil {
		return goa.ListModelsResponse{}, err
	}
	return client.Models().List(ctx)
}

func (a *App) renderModelsList(resp goa.ListModelsResponse, asJSON bool) int {
	if asJSON {
		return a.writeJSON(resp)
	}
	for _, model := range resp.Data {
		fmt.Fprintln(a.Stdout, model.ID)
	}
	return 0
}
