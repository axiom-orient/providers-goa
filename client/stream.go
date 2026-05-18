package client

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// ResponseStream materializes official Responses SSE events.
type ResponseStream struct {
	client *Client
	body   io.ReadCloser
	dec    *sseDecoder
	meta   ResponseMeta

	mu          sync.Mutex
	closed      bool
	final       Response
	finalSet    bool
	outputText  strings.Builder
	refusalText strings.Builder
}

func newResponseStream(client *Client, body io.ReadCloser, meta ResponseMeta) *ResponseStream {
	return &ResponseStream{
		client: client,
		body:   body,
		dec:    newSSEDecoder(body),
		meta:   meta,
	}
}

// Meta returns transport metadata captured when the stream was opened.
func (s *ResponseStream) Meta() ResponseMeta {
	if s == nil {
		return ResponseMeta{}
	}
	return s.meta
}

// Next returns the next parsed SSE event.
func (s *ResponseStream) Next() (StreamEvent, error) {
	var zero StreamEvent
	if s == nil {
		return zero, io.EOF
	}
	if s.isClosed() {
		return zero, io.EOF
	}

	msg, err := s.dec.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			_ = s.Close()
			return zero, io.EOF
		}
		_ = s.Close()
		return zero, fmt.Errorf("goa: stream decode: %w", err)
	}

	event, err := parseStreamEvent(msg)
	if err != nil {
		_ = s.Close()
		return zero, fmt.Errorf("goa: stream parse: %w", err)
	}
	if event.Response != nil {
		resp := *event.Response
		resp.Meta = s.meta
		event.Response = &resp
	}
	s.apply(event)
	if event.IsTerminal() {
		_ = s.Close()
	}
	return event, nil
}

// FinalResponse returns the completed response if a terminal event carried one.
func (s *ResponseStream) FinalResponse() (Response, bool) {
	if s == nil {
		return Response{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.final, s.finalSet
}

// OutputText returns the accumulated known text deltas or the final response text.
func (s *ResponseStream) OutputText() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	live := s.outputText.String()
	final := ""
	if s.finalSet {
		final = s.final.OutputText()
	}
	return preferMaterializedText(live, final)
}

// RefusalText returns accumulated refusal text or the final response refusal.
func (s *ResponseStream) RefusalText() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	live := s.refusalText.String()
	final := ""
	if s.finalSet {
		final = s.final.RefusalText()
	}
	return preferMaterializedText(live, final)
}

// Close releases the stream and the client's send guard.
func (s *ResponseStream) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	body := s.body
	s.mu.Unlock()
	if s.client != nil {
		s.client.endSend()
	}
	if body != nil {
		return body.Close()
	}
	return nil
}

func (s *ResponseStream) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *ResponseStream) apply(event StreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if chunk := streamTextDelta(event); chunk != "" {
		s.outputText.WriteString(chunk)
	} else if event.Type == "response.output_text.done" && s.outputText.Len() == 0 && event.Text != "" {
		s.outputText.WriteString(event.Text)
	}
	if chunk := streamRefusalDelta(event); chunk != "" {
		s.refusalText.WriteString(chunk)
	} else if event.Type == "response.refusal.done" && s.refusalText.Len() == 0 {
		if chunk := event.RefusalChunk(); chunk != "" {
			s.refusalText.WriteString(chunk)
		}
	}
	if event.Response != nil {
		resp := *event.Response
		s.final = resp
		s.finalSet = event.IsTerminal()
		if event.IsTerminal() {
			mergePreferredBuilder(&s.outputText, resp.OutputText())
			mergePreferredBuilder(&s.refusalText, resp.RefusalText())
		}
	}
}

func streamTextDelta(event StreamEvent) string {
	if event.Type == "response.output_text.delta" {
		return event.Delta
	}
	return ""
}

func streamRefusalDelta(event StreamEvent) string {
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

func preferMaterializedText(live, final string) string {
	switch {
	case final == "":
		return live
	case live == "":
		return final
	case len(final) > len(live):
		return final
	default:
		return live
	}
}

func mergePreferredBuilder(b *strings.Builder, final string) {
	if b == nil {
		return
	}
	preferred := preferMaterializedText(b.String(), final)
	if preferred == b.String() {
		return
	}
	b.Reset()
	b.WriteString(preferred)
}
