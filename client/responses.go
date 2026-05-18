package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// ResponsesService wraps /v1/responses.
type ResponsesService struct {
	client *Client
}

// Create calls POST /v1/responses.
func (s *ResponsesService) Create(ctx context.Context, in CreateResponseRequest) (Response, error) {
	var out Response
	if err := in.validateForCreate(); err != nil {
		return out, err
	}
	if err := s.client.requireCredential(); err != nil {
		return out, err
	}
	if s.client.isChatGPTTransport() {
		return s.createViaChatGPTStream(ctx, in)
	}
	if err := s.client.requireAPIKey(); err != nil {
		return out, err
	}
	if err := s.client.beginSend(); err != nil {
		return out, err
	}
	defer s.client.endSend()

	in.Stream = false
	payload, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	req, err := s.client.newRequest(ctx, http.MethodPost, "/v1/responses", payload)
	if err != nil {
		return out, err
	}
	raw, meta, err := s.client.doRawJSON(req)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	out.Meta = meta
	out.Raw = append(json.RawMessage(nil), raw...)
	return out, nil
}

// Stream opens a streaming POST /v1/responses request using official SSE events.
func (s *ResponsesService) Stream(ctx context.Context, in CreateResponseRequest) (*ResponseStream, error) {
	if err := in.validateForStream(); err != nil {
		return nil, err
	}
	if err := s.client.requireCredential(); err != nil {
		return nil, err
	}
	if s.client.isChatGPTTransport() {
		return s.openChatGPTStream(ctx, in)
	}
	if err := s.client.requireAPIKey(); err != nil {
		return nil, err
	}
	if err := s.client.beginSend(); err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			s.client.endSend()
		}
	}()

	in.Stream = true
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := s.client.newRequest(ctx, http.MethodPost, "/v1/responses", payload)
	if err != nil {
		return nil, err
	}
	body, meta, err := s.client.openStream(req)
	if err != nil {
		return nil, err
	}
	opened = true
	return newResponseStream(s.client, body, meta), nil
}

func (s *ResponsesService) createViaChatGPTStream(ctx context.Context, in CreateResponseRequest) (Response, error) {
	stream, err := s.openChatGPTStream(ctx, in)
	if err != nil {
		return Response{}, err
	}
	defer stream.Close()

	var terminalErr error
	for {
		event, err := stream.Next()
		if errors.Is(err, io.EOF) {
			if terminalErr != nil {
				return Response{}, terminalErr
			}
			final, ok := stream.FinalResponse()
			if !ok {
				return Response{}, &ValidationError{Field: "response", Message: "chatgpt stream ended without completed response"}
			}
			mergeChatGPTFinalResponse(&final, stream.OutputText())
			if err := final.TerminalError(); err != nil {
				return Response{}, err
			}
			return final, nil
		}
		if err != nil {
			return Response{}, err
		}
		if event.Response != nil {
			if err := event.Response.TerminalError(); err != nil {
				terminalErr = err
			}
		}
	}
}

func mergeChatGPTFinalResponse(resp *Response, text string) {
	if resp == nil || strings.TrimSpace(text) == "" || resp.OutputText() != "" {
		return
	}
	resp.Output = append(resp.Output, ResponseOutputItem{
		Type:   "message",
		Role:   "assistant",
		Status: "completed",
		Content: []ResponseContentPart{{
			Type: "output_text",
			Text: text,
		}},
	})
}

func (s *ResponsesService) openChatGPTStream(ctx context.Context, in CreateResponseRequest) (*ResponseStream, error) {
	if err := s.client.ensureChatGPTAccessToken(ctx); err != nil {
		return nil, err
	}
	if err := s.client.beginSend(); err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			s.client.endSend()
		}
	}()

	payload, err := marshalChatGPTResponseRequest(in)
	if err != nil {
		return nil, err
	}
	req, err := s.client.buildChatGPTResponsesRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	body, meta, err := s.client.openStream(req)
	if err != nil {
		return nil, err
	}
	opened = true
	return newResponseStream(s.client, body, meta), nil
}
