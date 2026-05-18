package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (c *Client) doJSON(req *http.Request, out any) (ResponseMeta, error) {
	raw, meta, err := c.doRawJSON(req)
	if err != nil {
		return meta, err
	}
	if out == nil || len(raw) == 0 {
		return meta, nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return meta, fmt.Errorf("goa: decode response: %w", err)
	}
	return meta, nil
}

func (c *Client) doRawJSON(req *http.Request) ([]byte, ResponseMeta, error) {
	resp, cancel, err := c.do(req)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if cancel != nil {
			cancel()
		}
	}()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, ResponseMeta{}, err
	}

	meta := ResponseMeta{RequestID: resp.Header.Get("x-request-id")}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, meta, &APIError{
			StatusCode: resp.StatusCode,
			RequestID:  meta.RequestID,
			Body:       summarizeBody(raw),
		}
	}
	return raw, meta, nil
}

func (c *Client) openStream(req *http.Request) (io.ReadCloser, ResponseMeta, error) {
	resp, cancel, err := c.do(req)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	meta := ResponseMeta{RequestID: resp.Header.Get("x-request-id")}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() {
			_ = resp.Body.Close()
			if cancel != nil {
				cancel()
			}
		}()
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, meta, readErr
		}
		return nil, meta, &APIError{
			StatusCode: resp.StatusCode,
			RequestID:  meta.RequestID,
			Body:       summarizeBody(raw),
		}
	}
	return &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}, meta, nil
}

func (c *Client) do(req *http.Request) (*http.Response, context.CancelFunc, error) {
	if err := c.validate(); err != nil {
		return nil, nil, err
	}
	for attempt := 1; ; attempt++ {
		attemptReq, cancel, err := c.cloneRequestForAttempt(req)
		if err != nil {
			return nil, nil, err
		}
		event := TransportEvent{
			Method:          attemptReq.Method,
			Path:            attemptReq.URL.Path,
			Attempt:         attempt,
			MaxRetries:      c.retryPolicy.MaxRetries,
			ClientRequestID: attemptReq.Header.Get("X-Client-Request-Id"),
		}
		c.emit(TransportEvent{
			Type:            TransportEventAttemptStart,
			Method:          event.Method,
			Path:            event.Path,
			Attempt:         event.Attempt,
			MaxRetries:      event.MaxRetries,
			ClientRequestID: event.ClientRequestID,
		})

		started := time.Now()
		resp, err := c.httpClient.Do(attemptReq)
		duration := time.Since(started)
		if err != nil {
			c.emit(TransportEvent{
				Type:            TransportEventAttemptComplete,
				Method:          event.Method,
				Path:            event.Path,
				Attempt:         event.Attempt,
				MaxRetries:      event.MaxRetries,
				ClientRequestID: event.ClientRequestID,
				Duration:        duration,
				Err:             err,
			})
			if cancel != nil {
				cancel()
			}
			if !transportRetryDisabled(req.Context()) && attempt <= c.retryPolicy.MaxRetries && shouldRetryTransportError(req, err) {
				delay := retryDelay(c.retryPolicy, attempt)
				c.emit(TransportEvent{
					Type:            TransportEventRetryScheduled,
					Method:          event.Method,
					Path:            event.Path,
					Attempt:         event.Attempt,
					MaxRetries:      event.MaxRetries,
					ClientRequestID: event.ClientRequestID,
					Duration:        duration,
					RetryDelay:      delay,
					Err:             err,
				})
				if sleepErr := sleepContext(req.Context(), delay); sleepErr != nil {
					return nil, nil, sleepErr
				}
				continue
			}
			return nil, nil, err
		}

		requestID := resp.Header.Get("x-request-id")
		c.emit(TransportEvent{
			Type:            TransportEventAttemptComplete,
			Method:          event.Method,
			Path:            event.Path,
			Attempt:         event.Attempt,
			MaxRetries:      event.MaxRetries,
			StatusCode:      resp.StatusCode,
			RequestID:       requestID,
			ClientRequestID: event.ClientRequestID,
			Duration:        duration,
		})
		if !transportRetryDisabled(req.Context()) && attempt <= c.retryPolicy.MaxRetries && retryableStatusCode(resp.StatusCode) {
			closeResponse(resp)
			if cancel != nil {
				cancel()
			}
			delay := retryDelay(c.retryPolicy, attempt)
			c.emit(TransportEvent{
				Type:            TransportEventRetryScheduled,
				Method:          event.Method,
				Path:            event.Path,
				Attempt:         event.Attempt,
				MaxRetries:      event.MaxRetries,
				StatusCode:      resp.StatusCode,
				RequestID:       requestID,
				ClientRequestID: event.ClientRequestID,
				Duration:        duration,
				RetryDelay:      delay,
			})
			if sleepErr := sleepContext(req.Context(), delay); sleepErr != nil {
				return nil, nil, sleepErr
			}
			continue
		}
		return resp, cancel, nil
	}
}

func (c *Client) cloneRequestForAttempt(req *http.Request) (*http.Request, context.CancelFunc, error) {
	attemptReq := req.Clone(req.Context())
	if req.Body != nil {
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, nil, err
			}
			attemptReq.Body = body
		} else {
			attemptReq.Body = req.Body
		}
	}
	if c.requestTimeout > 0 {
		ctx, cancel := context.WithTimeout(attemptReq.Context(), c.requestTimeout)
		attemptReq = attemptReq.WithContext(ctx)
		return attemptReq, cancel, nil
	}
	return attemptReq, nil, nil
}

func summarizeBody(raw []byte) string {
	body := strings.TrimSpace(string(raw))
	if len(body) > 512 {
		return body[:512] + "…"
	}
	return body
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelReadCloser) Close() error {
	if r == nil {
		return nil
	}
	var err error
	if r.ReadCloser != nil {
		err = r.ReadCloser.Close()
	}
	if r.cancel != nil {
		r.cancel()
	}
	return err
}

func closeResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
