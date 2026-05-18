package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponsesCreateStructuredOutput(t *testing.T) {
	type calendarEvent struct {
		Name         string   `json:"name"`
		Date         string   `json:"date"`
		Participants []string `json:"participants"`
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"date": map[string]any{"type": "string"},
			"participants": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required":             []string{"name", "date", "participants"},
		"additionalProperties": false,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		text, ok := body["text"].(map[string]any)
		if !ok {
			t.Fatalf("missing text config: %#v", body)
		}
		format, ok := text["format"].(map[string]any)
		if !ok {
			t.Fatalf("missing text.format: %#v", text)
		}
		if got := format["type"]; got != ResponseTextFormatTypeJSONSchema {
			t.Fatalf("unexpected format type: %#v", format)
		}
		if got := format["name"]; got != "calendar_event" {
			t.Fatalf("unexpected format name: %#v", format)
		}
		if got := format["description"]; got != "calendar event" {
			t.Fatalf("unexpected description: %#v", format)
		}
		if got := format["strict"]; got != true {
			t.Fatalf("unexpected strict value: %#v", format)
		}
		if _, ok := format["schema"].(map[string]any); !ok {
			t.Fatalf("missing schema: %#v", format)
		}

		w.Header().Set("x-request-id", "req_structured_123")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_123",
			"object": "response",
			"model":  "gpt-test",
			"output": []map[string]any{{
				"type": "message",
				"role": "assistant",
				"content": []map[string]any{{
					"type": "output_text",
					"text": `{"name":"Team Sync","date":"2026-04-18","participants":["ax","kim"]}`,
				}},
			}},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "Extract a calendar event.",
		Text: JSONSchemaTextFormat("calendar_event", schema, JSONSchemaFormatOptions{
			Description: "calendar event",
			Strict:      true,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Meta.RequestID != "req_structured_123" {
		t.Fatalf("unexpected request id: %q", resp.Meta.RequestID)
	}
	var event calendarEvent
	event, err = DecodeStructuredOutput[calendarEvent](resp)
	if err != nil {
		t.Fatal(err)
	}
	if event.Name != "Team Sync" || event.Date != "2026-04-18" || len(event.Participants) != 2 {
		t.Fatalf("unexpected structured output: %#v", event)
	}
	raw, err := resp.OutputJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"name":"Team Sync","date":"2026-04-18","participants":["ax","kim"]}` {
		t.Fatalf("unexpected structured raw json: %s", raw)
	}
}

func TestStructuredOutputValidationRejectsInvalidJSONSchemaFormat(t *testing.T) {
	client, err := NewClient(Config{BaseURL: "https://api.openai.com", APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Responses().Create(context.Background(), CreateResponseRequest{
		Model: "gpt-test",
		Input: "hello",
		Text:  JSONSchemaTextFormat("bad name!", map[string]any{"type": "object"}, JSONSchemaFormatOptions{}),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "text.format.name" {
		t.Fatalf("unexpected field: %#v", validationErr)
	}
}

func TestDecodeStructuredOutputReturnsRefusalError(t *testing.T) {
	resp := Response{
		Output: []ResponseOutputItem{{
			Type: "message",
			Role: "assistant",
			Content: []ResponseContentPart{{
				Type:    "refusal",
				Refusal: "cannot comply",
			}},
		}},
	}

	_, err := DecodeStructuredOutput[map[string]any](resp)
	if err == nil {
		t.Fatal("expected error")
	}
	var refusalErr *RefusalError
	if !errors.As(err, &refusalErr) {
		t.Fatalf("expected RefusalError, got %T", err)
	}
	if refusalErr.Message != "cannot comply" {
		t.Fatalf("unexpected refusal message: %#v", refusalErr)
	}
}
