package cli

import (
	"fmt"
	"strings"

	goa "github.com/axiom-orient/providers-goa/client"
)

func (a *App) runGemini(args []string, globalJSON bool) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "gemini requires a command")
		return 2
	}
	switch args[0] {
	case "generate":
		return a.runGeminiGenerate(args[1:], globalJSON)
	case "models":
		return a.runGeminiModels(args[1:], globalJSON)
	default:
		fmt.Fprintf(a.Stderr, "unknown gemini command %s\n", args[0])
		return 2
	}
}

func (a *App) runGeminiGenerate(args []string, globalJSON bool) int {
	prompt := ""
	model := ""
	nodePath := goa.DefaultGeminiNodePath
	adapterPath := goa.DefaultGeminiAdapterPath
	asJSON := globalJSON
	for len(args) > 0 {
		current := args[0]
		args = args[1:]
		switch current {
		case "--model":
			value, rest, err := parseStringFlag("--model", args)
			if err != nil {
				fmt.Fprintln(a.Stderr, err)
				return 2
			}
			model = value
			args = rest
		case "--node-path":
			value, rest, err := parseStringFlag("--node-path", args)
			if err != nil {
				fmt.Fprintln(a.Stderr, err)
				return 2
			}
			nodePath = value
			args = rest
		case "--adapter-path":
			value, rest, err := parseStringFlag("--adapter-path", args)
			if err != nil {
				fmt.Fprintln(a.Stderr, err)
				return 2
			}
			adapterPath = value
			args = rest
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(current, "--") {
				fmt.Fprintf(a.Stderr, "unknown gemini generate option %s\n", current)
				return 2
			}
			if prompt == "" {
				prompt = current
			} else {
				prompt += " " + current
			}
		}
	}
	response, err := goa.NewGeminiClient(nodePath, adapterPath).Generate(goa.GeminiGenerateRequest{
		Prompt: prompt,
		Model:  model,
	})
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	if asJSON {
		return a.writeJSON(response)
	}
	fmt.Fprintln(a.Stdout, response.Text)
	return 0
}

func (a *App) runGeminiModels(args []string, globalJSON bool) int {
	nodePath := goa.DefaultGeminiNodePath
	adapterPath := goa.DefaultGeminiAdapterPath
	asJSON := globalJSON
	for len(args) > 0 {
		current := args[0]
		args = args[1:]
		switch current {
		case "--node-path":
			value, rest, err := parseStringFlag("--node-path", args)
			if err != nil {
				fmt.Fprintln(a.Stderr, err)
				return 2
			}
			nodePath = value
			args = rest
		case "--adapter-path":
			value, rest, err := parseStringFlag("--adapter-path", args)
			if err != nil {
				fmt.Fprintln(a.Stderr, err)
				return 2
			}
			adapterPath = value
			args = rest
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(a.Stderr, "unknown gemini models option %s\n", current)
			return 2
		}
	}
	response, err := goa.NewGeminiClient(nodePath, adapterPath).Models()
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	if asJSON {
		return a.writeJSON(response)
	}
	fmt.Fprintf(a.Stdout, "Gemini models from %s (%s)\n", response.Provider, response.ReleaseChannel)
	for i, model := range response.Models {
		fmt.Fprintf(a.Stdout, "%d. %s [%s]", i+1, model.ID, model.Tier)
		if model.Description != "" {
			fmt.Fprintf(a.Stdout, " - %s", model.Description)
		}
		fmt.Fprintln(a.Stdout)
	}
	return 0
}
