package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	goa "github.com/axiom-orient/providers-goa/client"
)

type responseCommandInput struct {
	model        string
	input        string
	instructions string
	stream       bool
	asJSON       bool
	client       clientFlags
}

type usageError struct {
	message string
}

func (e usageError) Error() string { return e.message }

func (a *App) runResponses(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Stderr, "missing subcommand: codex send")
		return 2
	}
	switch args[0] {
	case "create":
		return a.runResponseCommand(ctx, "goa codex send", args[1:])
	default:
		fmt.Fprintf(a.Stderr, "unknown responses subcommand: %s\n", args[0])
		return 2
	}
}

func (a *App) runSend(ctx context.Context, args []string) int {
	return a.runResponseCommand(ctx, "goa codex send", args)
}

func (a *App) runResponseCommand(ctx context.Context, name string, args []string) int {
	input, err := parseResponseCommandInput(name, a.Stderr, args)
	if err != nil {
		var usage usageError
		if errors.As(err, &usage) {
			fmt.Fprintln(a.Stderr, usage.message)
		}
		return 2
	}
	client, ctx, err := executeResponseClient(ctx, input)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	request := input.request()
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

func (a *App) runStreamingResponse(ctx context.Context, client *goa.Client, request goa.CreateResponseRequest, asJSON bool) int {
	stream, err := executeResponseStream(ctx, client, request)
	if err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	defer stream.Close()

	if asJSON {
		return a.renderStreamingResponseJSON(stream)
	}
	return a.renderStreamingResponseText(stream)
}

func parseResponseCommandInput(name string, stderr io.Writer, args []string) (responseCommandInput, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "", "model id")
	input := fs.String("input", "", "text input")
	instructions := fs.String("instructions", "", "optional instructions")
	stream := fs.Bool("stream", false, "emit text deltas from SSE")
	flags := bindClientFlags(fs, true)
	asJSON := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return responseCommandInput{}, err
	}
	if *model == "" {
		return responseCommandInput{}, usageError{message: "--model is required"}
	}
	if *input == "" {
		return responseCommandInput{}, usageError{message: "--input is required"}
	}
	return responseCommandInput{
		model:        *model,
		input:        *input,
		instructions: *instructions,
		stream:       *stream,
		asJSON:       *asJSON,
		client:       *flags,
	}, nil
}

func (in responseCommandInput) request() goa.CreateResponseRequest {
	return goa.CreateResponseRequest{
		Model:        in.model,
		Input:        in.input,
		Instructions: in.instructions,
	}
}

func executeResponseClient(ctx context.Context, input responseCommandInput) (*goa.Client, context.Context, error) {
	ctx = input.client.context(ctx)
	client, err := goa.NewClient(input.client.config())
	if err != nil {
		return nil, nil, err
	}
	return client, ctx, nil
}

func executeResponseCreate(ctx context.Context, client *goa.Client, request goa.CreateResponseRequest) (goa.Response, error) {
	return client.Responses().Create(ctx, request)
}

func executeResponseStream(ctx context.Context, client *goa.Client, request goa.CreateResponseRequest) (*goa.ResponseStream, error) {
	return client.Responses().Stream(ctx, request)
}

func (a *App) renderResponseCreate(resp goa.Response, asJSON bool) int {
	if asJSON {
		if code := a.writeJSON(resp); code != 0 {
			return code
		}
		if err := resp.TerminalError(); err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		return 0
	}
	text := responseDisplayText(resp)
	if text != "" || resp.TerminalError() == nil {
		fmt.Fprintln(a.Stdout, text)
	}
	if err := resp.TerminalError(); err != nil {
		fmt.Fprintln(a.Stderr, err)
		return 1
	}
	return 0
}

func (a *App) renderStreamingResponseJSON(stream *goa.ResponseStream) int {
	enc := json.NewEncoder(a.Stdout)
	var terminalErr error
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			if terminalErr != nil {
				fmt.Fprintln(a.Stderr, terminalErr)
				return 1
			}
			if final, ok := stream.FinalResponse(); ok {
				if err := final.TerminalError(); err != nil {
					fmt.Fprintln(a.Stderr, err)
					return 1
				}
			}
			return 0
		}
		if err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		if err := enc.Encode(event); err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		if err := streamEventTerminalError(event, stream.Meta()); err != nil {
			terminalErr = err
		}
	}
}

func (a *App) renderStreamingResponseText(stream *goa.ResponseStream) int {
	var printed strings.Builder
	printedMode := ""
	var terminalErr error
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			return a.finishStreamingResponseText(stream, printed.String(), printedMode, terminalErr)
		}
		if err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
		if chunk := liveTextChunk(event); chunk != "" {
			fmt.Fprint(a.Stdout, chunk)
			printed.WriteString(chunk)
			printedMode = "text"
		}
		if chunk := liveRefusalChunk(event); chunk != "" {
			fmt.Fprint(a.Stdout, chunk)
			printed.WriteString(chunk)
			printedMode = "refusal"
		}
		if err := streamEventTerminalError(event, stream.Meta()); err != nil {
			terminalErr = err
		}
	}
}

func (a *App) finishStreamingResponseText(stream *goa.ResponseStream, printed, printedMode string, terminalErr error) int {
	if terminalErr != nil {
		if printed != "" {
			fmt.Fprintln(a.Stdout)
		}
		fmt.Fprintln(a.Stderr, terminalErr)
		return 1
	}
	switch printedMode {
	case "text":
		appendUnprintedSuffix(a.Stdout, printed, stream.OutputText())
		fmt.Fprintln(a.Stdout)
	case "refusal":
		appendUnprintedSuffix(a.Stdout, printed, stream.RefusalText())
		fmt.Fprintln(a.Stdout)
	default:
		if text := stream.OutputText(); text != "" {
			fmt.Fprintln(a.Stdout, text)
		} else if refusal := stream.RefusalText(); refusal != "" {
			fmt.Fprintln(a.Stdout, refusal)
		} else if final, ok := stream.FinalResponse(); ok && final.TerminalError() == nil {
			fmt.Fprintln(a.Stdout)
		}
	}
	if final, ok := stream.FinalResponse(); ok {
		if err := final.TerminalError(); err != nil {
			fmt.Fprintln(a.Stderr, err)
			return 1
		}
	}
	return 0
}

func responseDisplayText(resp goa.Response) string {
	return resp.VisibleText()
}

func liveTextChunk(event goa.StreamEvent) string {
	if event.Type == "response.output_text.delta" {
		return event.Delta
	}
	return ""
}

func liveRefusalChunk(event goa.StreamEvent) string {
	switch event.Type {
	case "response.refusal.delta":
		return event.Delta
	case "response.content_part.added":
		if event.Part != nil && (event.Part.Type == "refusal" || event.Part.Refusal != "") {
			return event.Part.Refusal
		}
	}
	return ""
}

func appendUnprintedSuffix(w io.Writer, printed, final string) {
	if printed == "" || final == "" {
		return
	}
	if !strings.HasPrefix(final, printed) {
		return
	}
	suffix := strings.TrimPrefix(final, printed)
	if suffix != "" {
		fmt.Fprint(w, suffix)
	}
}

func streamEventTerminalError(event goa.StreamEvent, meta goa.ResponseMeta) error {
	if event.Response != nil {
		if err := event.Response.TerminalError(); err != nil {
			return err
		}
	}
	if event.Error == nil {
		return nil
	}
	detail := event.Error.Error()
	if meta.RequestID != "" {
		return fmt.Errorf("response error (request_id=%s): %s", meta.RequestID, detail)
	}
	return fmt.Errorf("response error: %s", detail)
}
