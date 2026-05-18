package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONSchemaFormatOptions configures json_schema structured outputs.
type JSONSchemaFormatOptions struct {
	Description string
	Strict      bool
}

// JSONSchemaTextFormat builds a typed Responses text.format configuration for structured outputs.
func JSONSchemaTextFormat(name string, schema map[string]any, opts JSONSchemaFormatOptions) *ResponseTextConfig {
	return &ResponseTextConfig{
		Format: &ResponseTextFormat{
			Type:        ResponseTextFormatTypeJSONSchema,
			Name:        name,
			Description: opts.Description,
			Schema:      cloneMap(schema),
			Strict:      opts.Strict,
		},
	}
}

// JSONObjectTextFormat builds a typed Responses text.format configuration for legacy JSON mode.
func JSONObjectTextFormat() *ResponseTextConfig {
	return &ResponseTextConfig{Format: &ResponseTextFormat{Type: ResponseTextFormatTypeJSONObject}}
}

// PlainTextFormat builds a typed Responses text.format configuration for normal text output.
func PlainTextFormat() *ResponseTextConfig {
	return &ResponseTextConfig{Format: &ResponseTextFormat{Type: ResponseTextFormatTypeText}}
}

// OutputJSON parses a structured-output response from output_text.
func (r Response) OutputJSON() (json.RawMessage, error) {
	if refusal := strings.TrimSpace(r.RefusalText()); refusal != "" {
		return nil, &RefusalError{Message: refusal}
	}
	text := strings.TrimSpace(r.OutputText())
	if text == "" {
		return nil, &StructuredOutputError{Message: "response did not contain structured JSON output"}
	}
	if !json.Valid([]byte(text)) {
		return nil, &StructuredOutputError{Message: "output_text is not valid JSON", OutputText: text}
	}
	return json.RawMessage(text), nil
}

// DecodeStructuredOutput decodes structured JSON output into a typed value.
func DecodeStructuredOutput[T any](resp Response) (T, error) {
	var out T
	raw, err := resp.OutputJSON()
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("goa: decode structured output: %w", err)
	}
	return out, nil
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
