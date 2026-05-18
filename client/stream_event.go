package client

import "encoding/json"

// StreamEvent is a partial typed view of Responses SSE events.
type StreamEvent struct {
	Event        string               `json:"event,omitempty"`
	Type         string               `json:"type,omitempty"`
	ItemID       string               `json:"item_id,omitempty"`
	OutputIndex  int                  `json:"output_index,omitempty"`
	ContentIndex int                  `json:"content_index,omitempty"`
	Delta        string               `json:"delta,omitempty"`
	Text         string               `json:"text,omitempty"`
	Refusal      string               `json:"refusal,omitempty"`
	Response     *Response            `json:"response,omitempty"`
	Item         *ResponseOutputItem  `json:"item,omitempty"`
	Part         *ResponseContentPart `json:"part,omitempty"`
	Error        *ResponseError       `json:"error,omitempty"`
	Raw          json.RawMessage      `json:"-"`
}

// TextChunk returns the text fragment exposed by known output text events.
func (e StreamEvent) TextChunk() string {
	switch e.Type {
	case "response.output_text.delta":
		return e.Delta
	case "response.output_text.done":
		return e.Text
	default:
		return ""
	}
}

// RefusalChunk returns the refusal fragment exposed by known refusal events.
func (e StreamEvent) RefusalChunk() string {
	switch e.Type {
	case "response.refusal.delta":
		return e.Delta
	case "response.refusal.done":
		if e.Refusal != "" {
			return e.Refusal
		}
		return e.Text
	case "response.content_part.added":
		if e.Part != nil && (e.Part.Type == "refusal" || e.Part.Refusal != "") {
			return e.Part.Refusal
		}
		return ""
	default:
		return ""
	}
}

// IsTerminal reports whether the event closes the stream lifecycle.
func (e StreamEvent) IsTerminal() bool {
	switch e.Type {
	case "response.completed", "response.failed", "response.incomplete", "response.cancelled", "response.error", "error":
		return true
	default:
		return false
	}
}

type rawStreamEvent struct {
	Type         string          `json:"type"`
	ItemID       string          `json:"item_id,omitempty"`
	OutputIndex  int             `json:"output_index,omitempty"`
	ContentIndex int             `json:"content_index,omitempty"`
	Delta        string          `json:"delta,omitempty"`
	Text         string          `json:"text,omitempty"`
	Refusal      string          `json:"refusal,omitempty"`
	Response     json.RawMessage `json:"response,omitempty"`
	Item         json.RawMessage `json:"item,omitempty"`
	Part         json.RawMessage `json:"part,omitempty"`
	Error        json.RawMessage `json:"error,omitempty"`
}

func parseStreamEvent(msg sseMessage) (StreamEvent, error) {
	var raw rawStreamEvent
	if err := json.Unmarshal(msg.Data, &raw); err != nil {
		return StreamEvent{}, err
	}
	event := StreamEvent{
		Event:        msg.Event,
		Type:         raw.Type,
		ItemID:       raw.ItemID,
		OutputIndex:  raw.OutputIndex,
		ContentIndex: raw.ContentIndex,
		Delta:        raw.Delta,
		Text:         raw.Text,
		Refusal:      raw.Refusal,
		Raw:          append(json.RawMessage(nil), msg.Data...),
	}
	if event.Event == "" {
		event.Event = event.Type
	}
	if event.Type == "" {
		event.Type = event.Event
	}
	if len(raw.Response) > 0 {
		var resp Response
		if err := json.Unmarshal(raw.Response, &resp); err != nil {
			return StreamEvent{}, err
		}
		resp.Raw = append(json.RawMessage(nil), raw.Response...)
		event.Response = &resp
	}
	if len(raw.Item) > 0 {
		var item ResponseOutputItem
		if err := json.Unmarshal(raw.Item, &item); err != nil {
			return StreamEvent{}, err
		}
		event.Item = &item
	}
	if len(raw.Part) > 0 {
		var part ResponseContentPart
		if err := json.Unmarshal(raw.Part, &part); err != nil {
			return StreamEvent{}, err
		}
		event.Part = &part
	}
	if len(raw.Error) > 0 {
		var respErr ResponseError
		if err := json.Unmarshal(raw.Error, &respErr); err != nil {
			return StreamEvent{}, err
		}
		event.Error = &respErr
	}
	if event.Error == nil && event.Response != nil && event.Response.Error != nil {
		event.Error = event.Response.Error
	}
	return event, nil
}
