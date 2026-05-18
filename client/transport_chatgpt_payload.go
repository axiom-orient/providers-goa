package client

import (
	"errors"

	"github.com/axiom-orient/providers-goa/client/internal/chatgptwire"
)

func marshalChatGPTResponseRequest(in CreateResponseRequest) ([]byte, error) {
	parallelToolCalls := false
	if in.ParallelToolCalls != nil {
		parallelToolCalls = *in.ParallelToolCalls
	}
	payload, err := chatgptwire.MarshalResponseRequest(chatgptwire.Request{
		Model:             in.Model,
		Input:             in.Input,
		Instructions:      in.Instructions,
		ParallelToolCalls: parallelToolCalls,
		ToolChoice:        in.ToolChoice,
		Tools:             toChatGPTWireTools(in.Tools),
		Reasoning:         in.Reasoning,
		Text:              in.Text,
		Extra:             in.Extra,
	})
	if err != nil {
		return nil, mapChatGPTWireError(err)
	}
	return payload, nil
}

func toChatGPTWireTools(tools []ResponseFunctionTool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		out = append(out, map[string]any{
			"type":        tool.Type,
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Parameters,
			"strict":      tool.Strict,
		})
	}
	return out
}

func mapChatGPTWireError(err error) error {
	if err == nil {
		return nil
	}
	var fieldErr *chatgptwire.FieldError
	if errors.As(err, &fieldErr) {
		return &ValidationError{Field: fieldErr.Field, Message: fieldErr.Message}
	}
	return err
}
