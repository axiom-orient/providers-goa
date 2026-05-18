package appserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type rpcEnvelope struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type pendingResponse struct {
	env rpcEnvelope
	err error
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	id := c.nextID.Add(1)
	return c.callWithID(ctx, id, method, params, out)
}

func (c *Client) callWithID(ctx context.Context, id int64, method string, params any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.currentErr(); err != nil {
		return err
	}
	respCh := make(chan pendingResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if err := c.send(map[string]any{"method": method, "id": id, "params": params}); err != nil {
		return err
	}

	select {
	case resp := <-respCh:
		if resp.err != nil {
			return resp.err
		}
		if resp.env.Error != nil {
			return resp.env.Error
		}
		if out == nil || len(resp.env.Result) == 0 {
			return nil
		}
		return json.Unmarshal(resp.env.Result, out)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) send(v any) error {
	if err := c.currentErr(); err != nil {
		return err
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	c.encMu.Lock()
	defer c.encMu.Unlock()
	_, err = c.conn.Write(payload)
	if err != nil {
		c.setReadErr(err)
		c.finishShutdown(err)
	}
	return err
}

func (c *Client) readLoop() {
	dec := json.NewDecoder(c.conn)
	for {
		var env rpcEnvelope
		if err := dec.Decode(&env); err != nil {
			c.setReadErr(err)
			c.finishShutdown(err)
			return
		}
		switch {
		case env.Method != "" && len(env.ID) > 0:
			c.handleServerRequest(env)
		case env.Method != "":
			notification := Notification{Method: env.Method, Params: cloneBytes(env.Params)}
			select {
			case c.notifications <- notification:
			case <-c.closed:
				return
			}
		default:
			id, err := decodeNumericID(env.ID)
			if err != nil {
				c.setReadErr(err)
				c.finishShutdown(err)
				return
			}
			c.pendingMu.Lock()
			respCh := c.pending[id]
			delete(c.pending, id)
			c.pendingMu.Unlock()
			if respCh != nil {
				respCh <- pendingResponse{env: env}
				close(respCh)
			}
		}
	}
}

func (c *Client) currentErr() error {
	select {
	case <-c.closed:
		c.errMu.RLock()
		defer c.errMu.RUnlock()
		if c.readErr == nil {
			return ErrClosed
		}
		return c.readErr
	default:
		return nil
	}
}

func (c *Client) setReadErr(err error) {
	if err == nil {
		return
	}
	c.errMu.Lock()
	if c.readErr == nil || errors.Is(c.readErr, ErrClosed) {
		c.readErr = err
	}
	c.errMu.Unlock()
}

func (c *Client) finishShutdown(err error) {
	c.shutdown.Do(func() {
		close(c.closed)
		c.pendingMu.Lock()
		for id, respCh := range c.pending {
			delete(c.pending, id)
			respCh <- pendingResponse{err: err}
			close(respCh)
		}
		c.pendingMu.Unlock()
		close(c.notifications)
	})
}

func decodeNumericID(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, errors.New("appserver: missing response id")
	}
	var id int64
	if err := json.Unmarshal(raw, &id); err != nil {
		return 0, fmt.Errorf("appserver: decode response id: %w", err)
	}
	return id, nil
}

func decodeRawID(raw json.RawMessage) any {
	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func cloneBytes(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
