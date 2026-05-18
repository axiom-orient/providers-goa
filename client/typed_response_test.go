package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponsesCreateTypedCoverage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("x-request-id", "req_typed_123")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                 "resp_123",
			"object":             "response",
			"model":              "gpt-test",
			"status":             "incomplete",
			"incomplete_details": map[string]any{"reason": "max_output_tokens"},
			"output": []map[string]any{
				{
					"id":     "msg_1",
					"type":   "message",
					"role":   "assistant",
					"status": "completed",
					"content": []map[string]any{
						{
							"type":        "output_text",
							"text":        "hello",
							"annotations": []map[string]any{{"type": "file_citation", "file_id": "file_1", "filename": "doc.txt", "index": 0}},
							"logprobs":    []map[string]any{{"token": "hello", "bytes": []int{104}, "logprob": -0.1, "top_logprobs": []map[string]any{{"token": "hello", "bytes": []int{104}, "logprob": -0.1}}}},
						},
						{
							"type":    "refusal",
							"refusal": "policy block",
						},
					},
				},
				{
					"id":        "fc_1",
					"type":      "function_call",
					"status":    "completed",
					"call_id":   "call_1",
					"name":      "get_weather",
					"arguments": `{"location":"Seoul"}`,
				},
				{
					"id":     "rs_1",
					"type":   "reasoning",
					"status": "completed",
					"summary": []map[string]any{
						{"type": "summary_text", "text": "looked up weather"},
					},
					"content": []map[string]any{
						{"type": "reasoning_text", "text": "internal reasoning text"},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":         10,
				"input_tokens_details": map[string]any{"cached_tokens": 2},
				"output_tokens":        5,
				"output_tokens_details": map[string]any{
					"reasoning_tokens": 1,
				},
				"total_tokens": 15,
			},
		})
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.Responses().Create(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Meta.RequestID != "req_typed_123" {
		t.Fatalf("unexpected request id: %q", resp.Meta.RequestID)
	}
	if resp.IncompleteDetails == nil || resp.IncompleteDetails.Reason != "max_output_tokens" {
		t.Fatalf("unexpected incomplete details: %#v", resp.IncompleteDetails)
	}
	if resp.Usage == nil {
		t.Fatal("expected usage")
	}
	if resp.Usage.InputTokensDetails == nil || resp.Usage.InputTokensDetails.CachedTokens != 2 {
		t.Fatalf("unexpected input token details: %#v", resp.Usage.InputTokensDetails)
	}
	if resp.Usage.OutputTokensDetails == nil || resp.Usage.OutputTokensDetails.ReasoningTokens != 1 {
		t.Fatalf("unexpected output token details: %#v", resp.Usage.OutputTokensDetails)
	}
	if got := resp.OutputText(); got != "hello" {
		t.Fatalf("unexpected output text: %q", got)
	}
	if len(resp.Output) != 3 {
		t.Fatalf("unexpected output item count: %d", len(resp.Output))
	}

	msg := resp.Output[0]
	if msg.Type != "message" || msg.Role != "assistant" || msg.Status != "completed" {
		t.Fatalf("unexpected message item: %#v", msg)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("unexpected message content: %#v", msg.Content)
	}
	if len(msg.Content[0].Annotations) != 1 {
		t.Fatalf("unexpected annotations: %#v", msg.Content[0].Annotations)
	}
	if len(msg.Content[0].Logprobs) != 1 {
		t.Fatalf("unexpected logprobs: %#v", msg.Content[0].Logprobs)
	}
	if msg.Content[1].Type != "refusal" || msg.Content[1].Refusal != "policy block" {
		t.Fatalf("unexpected refusal part: %#v", msg.Content[1])
	}

	call := resp.Output[1]
	if call.Type != "function_call" || call.CallID != "call_1" || call.Name != "get_weather" || call.Arguments != `{"location":"Seoul"}` {
		t.Fatalf("unexpected function call item: %#v", call)
	}

	reasoning := resp.Output[2]
	if reasoning.Type != "reasoning" || len(reasoning.Summary) != 1 || reasoning.Summary[0].Text != "looked up weather" {
		t.Fatalf("unexpected reasoning item: %#v", reasoning)
	}
	if len(reasoning.Content) != 1 || reasoning.Content[0].Type != "reasoning_text" || reasoning.Content[0].Text != "internal reasoning text" {
		t.Fatalf("unexpected reasoning content: %#v", reasoning.Content)
	}
}

func TestResponsesStreamTypedItemAndPartCoverage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_stream_typed_123")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}
		fmt.Fprint(w, "event: response.created\n")
		fmt.Fprint(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"in_progress\",\"model\":\"gpt-test\",\"output\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_item.added\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"status\":\"in_progress\",\"role\":\"assistant\",\"content\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.content_part.added\n")
		fmt.Fprint(w, "data: {\"type\":\"response.content_part.added\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"part\":{\"type\":\"output_text\",\"text\":\"\",\"annotations\":[{\"type\":\"file_citation\",\"file_id\":\"file_1\",\"filename\":\"doc.txt\",\"index\":0}],\"logprobs\":[]}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_text.delta\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"item_id\":\"msg_1\",\"output_index\":0,\"content_index\":0,\"delta\":\"hello\"}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.output_item.done\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_item.done\",\"output_index\":0,\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\",\"annotations\":[{\"type\":\"file_citation\",\"file_id\":\"file_1\",\"filename\":\"doc.txt\",\"index\":0}],\"logprobs\":[{\"token\":\"hello\",\"bytes\":[104],\"logprob\":-0.1,\"top_logprobs\":[{\"token\":\"hello\",\"bytes\":[104],\"logprob\":-0.1}]}]}]}}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "event: response.completed\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"object\":\"response\",\"status\":\"completed\",\"model\":\"gpt-test\",\"output\":[{\"id\":\"msg_1\",\"type\":\"message\",\"status\":\"completed\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\",\"annotations\":[{\"type\":\"file_citation\",\"file_id\":\"file_1\",\"filename\":\"doc.txt\",\"index\":0}],\"logprobs\":[{\"token\":\"hello\",\"bytes\":[104],\"logprob\":-0.1,\"top_logprobs\":[{\"token\":\"hello\",\"bytes\":[104],\"logprob\":-0.1}]}]}]}],\"usage\":{\"input_tokens\":1,\"input_tokens_details\":{\"cached_tokens\":0},\"output_tokens\":1,\"output_tokens_details\":{\"reasoning_tokens\":0},\"total_tokens\":2}}}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client, err := NewClient(Config{BaseURL: srv.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatal(err)
	}

	stream, err := client.Responses().Stream(context.Background(), CreateResponseRequest{Model: "gpt-test", Input: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var sawItemAdded bool
	var sawPartAdded bool
	var sawItemDone bool
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch event.Type {
		case "response.output_item.added":
			sawItemAdded = true
			if event.Item == nil || event.Item.ID != "msg_1" || event.Item.Type != "message" || event.Item.Status != "in_progress" || event.Item.Role != "assistant" {
				t.Fatalf("unexpected added item: %#v", event.Item)
			}
		case "response.content_part.added":
			sawPartAdded = true
			if event.Part == nil || event.Part.Type != "output_text" || len(event.Part.Annotations) != 1 || len(event.Part.Logprobs) != 0 {
				t.Fatalf("unexpected added part: %#v", event.Part)
			}
		case "response.output_item.done":
			sawItemDone = true
			if event.Item == nil || event.Item.Status != "completed" || len(event.Item.Content) != 1 || len(event.Item.Content[0].Annotations) != 1 || len(event.Item.Content[0].Logprobs) != 1 {
				t.Fatalf("unexpected completed item: %#v", event.Item)
			}
		}
	}

	if !sawItemAdded || !sawPartAdded || !sawItemDone {
		t.Fatalf("missing expected stream events: item_added=%v part_added=%v item_done=%v", sawItemAdded, sawPartAdded, sawItemDone)
	}
	if got := stream.OutputText(); got != "hello" {
		t.Fatalf("unexpected output text: %q", got)
	}
	final, ok := stream.FinalResponse()
	if !ok {
		t.Fatal("expected final response")
	}
	if final.Usage == nil || final.Usage.OutputTokensDetails == nil || final.Usage.OutputTokensDetails.ReasoningTokens != 0 {
		t.Fatalf("unexpected final usage: %#v", final.Usage)
	}
	if len(final.Output) != 1 || len(final.Output[0].Content) != 1 || len(final.Output[0].Content[0].Annotations) != 1 || len(final.Output[0].Content[0].Logprobs) != 1 {
		t.Fatalf("unexpected final output item: %#v", final.Output)
	}
}
