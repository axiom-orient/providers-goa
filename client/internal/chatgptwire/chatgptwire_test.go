package chatgptwire

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMarshalChatGPTResponseRequestPreservesWhitespace(t *testing.T) {
	body, err := MarshalResponseRequest(Request{
		Model:        "gpt-5.4",
		Instructions: "  keep outer padding  ",
		Input: []any{
			map[string]any{
				"role": "system",
				"content": []any{
					map[string]any{"type": "text", "text": "  system line 1\n\nsystem line 2  "},
				},
			},
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "  user prompt  "},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	if got := payload["instructions"]; got != "  keep outer padding  \n\n  system line 1\n\nsystem line 2  " {
		t.Fatalf("unexpected instructions payload: %#v", got)
	}
	input, ok := payload["input"].([]any)
	if !ok || len(input) != 1 {
		t.Fatalf("unexpected input payload: %#v", payload["input"])
	}
	message, ok := input[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected message payload: %#v", input[0])
	}
	content, ok := message["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("unexpected content payload: %#v", message["content"])
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected content part: %#v", content[0])
	}
	if got := part["text"]; got != "  user prompt  " {
		t.Fatalf("unexpected text payload: %#v", got)
	}
}

func TestMarshalChatGPTResponseRequestIncludesText(t *testing.T) {
	body, err := MarshalResponseRequest(Request{
		Model: "gpt-5.4",
		Input: "extract",
		Text: map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "provider_status",
				"schema": map[string]any{"type": "object"},
				"strict": true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	format := payload["text"].(map[string]any)["format"].(map[string]any)
	if got := format["type"]; got != "json_schema" {
		t.Fatalf("unexpected text format: %#v", payload["text"])
	}
	if _, ok := payload["metadata"]; ok {
		t.Fatalf("chatgpt wire payload should not include unsupported metadata: %#v", payload["metadata"])
	}
}

func TestNormalizeChatGPTInputRejectsNonObjectContentItem(t *testing.T) {
	_, _, err := NormalizeInput([]any{
		map[string]any{
			"role":    "user",
			"content": []any{"bad"},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var fieldErr *FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T", err)
	}
	if fieldErr.Field != "input.content" {
		t.Fatalf("unexpected field: %#v", fieldErr)
	}
}

func TestNormalizeChatGPTInputRejectsMalformedImageContent(t *testing.T) {
	_, _, err := NormalizeInput([]any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": 123}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var fieldErr *FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T", err)
	}
	if fieldErr.Field != "input.content" {
		t.Fatalf("unexpected field: %#v", fieldErr)
	}
}

func TestNormalizeChatGPTInputRejectsUnsupportedContentType(t *testing.T) {
	_, _, err := NormalizeInput([]any{
		map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "audio", "url": "https://example.com/a.wav"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var fieldErr *FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T", err)
	}
	if fieldErr.Field != "input.content" {
		t.Fatalf("unexpected field: %#v", fieldErr)
	}
}

func TestNormalizeChatGPTInputRejectsSystemImageContent(t *testing.T) {
	_, _, err := NormalizeInput([]any{
		map[string]any{
			"role": "system",
			"content": []any{
				map[string]any{"type": "image_url", "url": "https://example.com/image.png"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var fieldErr *FieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected FieldError, got %T", err)
	}
	if fieldErr.Field != "input.content" {
		t.Fatalf("unexpected field: %#v", fieldErr)
	}
}
