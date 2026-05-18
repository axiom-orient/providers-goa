package client

import "encoding/json"

const (
	ResponseTextFormatTypeText       = "text"
	ResponseTextFormatTypeJSONSchema = "json_schema"
	ResponseTextFormatTypeJSONObject = "json_object"
)

// ResponseTextConfig configures text generation options on Responses requests.
type ResponseTextConfig struct {
	Format    *ResponseTextFormat `json:"format,omitempty"`
	Verbosity string              `json:"verbosity,omitempty"`
}

// ResponseTextFormat is a typed subset of the Responses text.format union.
type ResponseTextFormat struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
}

// ResponseFunctionTool is the supported function-tool request shape for official Responses and ChatGPT backend calls.
type ResponseFunctionTool struct {
	Type        string         `json:"type,omitempty"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Strict      bool           `json:"strict,omitempty"`
}

// CreateResponseRequest is a stable subset with an escape hatch for new optional fields.
type CreateResponseRequest struct {
	Model             string                 `json:"-"`
	Input             any                    `json:"-"`
	Instructions      string                 `json:"-"`
	Stream            bool                   `json:"-"`
	Prompt            any                    `json:"-"`
	Reasoning         any                    `json:"-"`
	Text              *ResponseTextConfig    `json:"-"`
	Tools             []ResponseFunctionTool `json:"-"`
	ToolChoice        any                    `json:"-"`
	ParallelToolCalls *bool                  `json:"-"`
	Metadata          map[string]string      `json:"-"`
	Extra             map[string]any         `json:"-"`
}

// MarshalJSON merges the stable subset with Extra for additive request fields.
func (r CreateResponseRequest) MarshalJSON() ([]byte, error) {
	payload := map[string]any{}
	if r.Model != "" {
		payload["model"] = r.Model
	}
	if r.Input != nil {
		payload["input"] = r.Input
	}
	if r.Instructions != "" {
		payload["instructions"] = r.Instructions
	}
	if r.Stream {
		payload["stream"] = true
	}
	if r.Prompt != nil {
		payload["prompt"] = r.Prompt
	}
	if r.Reasoning != nil {
		payload["reasoning"] = r.Reasoning
	}
	if r.Text != nil {
		payload["text"] = r.Text
	}
	if len(r.Tools) > 0 {
		payload["tools"] = r.Tools
	}
	if r.ToolChoice != nil {
		payload["tool_choice"] = r.ToolChoice
	}
	if r.ParallelToolCalls != nil {
		payload["parallel_tool_calls"] = *r.ParallelToolCalls
	}
	if len(r.Metadata) > 0 {
		payload["metadata"] = r.Metadata
	}
	for k, v := range r.Extra {
		if _, exists := payload[k]; !exists {
			payload[k] = v
		}
	}
	return json.Marshal(payload)
}
