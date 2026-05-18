package client

import (
	"errors"
	"fmt"
	"strings"
)

func (e *ResponseError) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Code != "" && e.Message != "":
		return e.Code + ": " + e.Message
	case e.Message != "":
		return e.Message
	case e.Code != "":
		return e.Code
	default:
		return "unknown response error"
	}
}

// VisibleText returns the best user-visible text derived from the response.
func (r Response) VisibleText() string {
	if text := r.OutputText(); text != "" {
		return text
	}
	return r.RefusalText()
}

// TerminalError reports lifecycle failures surfaced by the Responses API payload.
func (r Response) TerminalError() error {
	status := strings.TrimSpace(r.Status)
	switch status {
	case "", "completed", "in_progress", "queued":
		return nil
	case "failed", "cancelled":
		detail := ""
		if r.Error != nil {
			detail = r.Error.Error()
		}
		return responseLifecycleError(status, detail, r.Meta.RequestID)
	case "incomplete":
		detail := ""
		if r.IncompleteDetails != nil {
			detail = strings.TrimSpace(r.IncompleteDetails.Reason)
		}
		if detail == "" && r.Error != nil {
			detail = r.Error.Error()
		}
		return responseLifecycleError(status, detail, r.Meta.RequestID)
	default:
		if r.Error != nil {
			return responseLifecycleError(status, r.Error.Error(), r.Meta.RequestID)
		}
		return nil
	}
}

func responseLifecycleError(status, detail, requestID string) error {
	var b strings.Builder
	b.WriteString("response ")
	if status == "" {
		b.WriteString("error")
	} else {
		b.WriteString(status)
	}
	if requestID != "" {
		fmt.Fprintf(&b, " (request_id=%s)", requestID)
	}
	if detail != "" {
		b.WriteString(": ")
		b.WriteString(detail)
	}
	return errors.New(b.String())
}
