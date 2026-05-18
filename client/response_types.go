package client

import (
	"encoding/json"
	"strings"
)

// ResponseMeta contains transport metadata useful for debugging.
type ResponseMeta struct {
	RequestID string `json:"request_id,omitempty"`
}

// Model is a subset of the /v1/models schema.
type Model struct {
	ID      string `json:"id"`
	Created int64  `json:"created,omitempty"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// ListModelsResponse is a subset of the /v1/models response.
type ListModelsResponse struct {
	Object string       `json:"object,omitempty"`
	Data   []Model      `json:"data"`
	Meta   ResponseMeta `json:"meta,omitempty"`
}

// ResponseInputTokensDetails is a partial typed view of usage input-token details.
type ResponseInputTokensDetails struct {
	CachedTokens int64 `json:"cached_tokens,omitempty"`
}

// ResponseOutputTokensDetails is a partial typed view of usage output-token details.
type ResponseOutputTokensDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens,omitempty"`
}

// ResponseUsage is a partial typed view of token usage.
type ResponseUsage struct {
	InputTokens         int64                        `json:"input_tokens,omitempty"`
	InputTokensDetails  *ResponseInputTokensDetails  `json:"input_tokens_details,omitempty"`
	OutputTokens        int64                        `json:"output_tokens,omitempty"`
	OutputTokensDetails *ResponseOutputTokensDetails `json:"output_tokens_details,omitempty"`
	TotalTokens         int64                        `json:"total_tokens,omitempty"`
}

// ResponseIncompleteDetails is a partial typed view of incomplete termination details.
type ResponseIncompleteDetails struct {
	Reason string `json:"reason,omitempty"`
}

// ResponseReasoningSummaryPart is a partial typed view of reasoning summary content.
type ResponseReasoningSummaryPart struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// Response is a forward-compatible subset of the /v1/responses payload.
type Response struct {
	ID                string                     `json:"id"`
	Object            string                     `json:"object,omitempty"`
	Model             string                     `json:"model,omitempty"`
	Status            string                     `json:"status,omitempty"`
	CreatedAt         int64                      `json:"created_at,omitempty"`
	CompletedAt       int64                      `json:"completed_at,omitempty"`
	Instructions      string                     `json:"instructions,omitempty"`
	IncompleteDetails *ResponseIncompleteDetails `json:"incomplete_details,omitempty"`
	OutputTextField   string                     `json:"output_text,omitempty"`
	Output            []ResponseOutputItem       `json:"output,omitempty"`
	Usage             *ResponseUsage             `json:"usage,omitempty"`
	Error             *ResponseError             `json:"error,omitempty"`
	Meta              ResponseMeta               `json:"meta,omitempty"`
	Raw               json.RawMessage            `json:"-"`
}

// ResponseOutputItem is a partial typed view of response output items.
type ResponseOutputItem struct {
	ID               string                         `json:"id,omitempty"`
	Type             string                         `json:"type,omitempty"`
	Role             string                         `json:"role,omitempty"`
	Status           string                         `json:"status,omitempty"`
	Content          []ResponseContentPart          `json:"content,omitempty"`
	CallID           string                         `json:"call_id,omitempty"`
	Name             string                         `json:"name,omitempty"`
	Arguments        string                         `json:"arguments,omitempty"`
	Summary          []ResponseReasoningSummaryPart `json:"summary,omitempty"`
	EncryptedContent string                         `json:"encrypted_content,omitempty"`
}

// ResponseContentPart is a partial typed view of content parts.
type ResponseContentPart struct {
	Type        string            `json:"type,omitempty"`
	Text        string            `json:"text,omitempty"`
	Refusal     string            `json:"refusal,omitempty"`
	Annotations []json.RawMessage `json:"annotations,omitempty"`
	Logprobs    []json.RawMessage `json:"logprobs,omitempty"`
}

// ResponseError is a partial typed view of response errors.
type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// OutputText derives assistant text from known output shapes.
func (r Response) OutputText() string {
	if r.OutputTextField != "" {
		return r.OutputTextField
	}
	var b strings.Builder
	for _, item := range r.Output {
		for _, part := range item.Content {
			switch part.Type {
			case "output_text", "text":
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}

// RefusalText derives refusal text from known response shapes.
func (r Response) RefusalText() string {
	var b strings.Builder
	for _, item := range r.Output {
		for _, part := range item.Content {
			if part.Type == "refusal" || part.Refusal != "" {
				b.WriteString(part.Refusal)
			}
		}
	}
	return b.String()
}
