package appserver

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var (
	// ErrClosed is returned when the app-server client is already closed.
	ErrClosed = errors.New("appserver: client closed")
)

// Client is a stable stdio/json-rpc client for codex app-server account methods.
type Client struct {
	conn io.ReadWriteCloser

	encMu sync.Mutex

	nextID atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan pendingResponse

	notifications chan Notification

	closeOnce sync.Once
	shutdown  sync.Once
	closed    chan struct{}

	errMu   sync.RWMutex
	readErr error

	initResult     InitializeResult
	refreshHandler ChatGPTAuthTokensRefreshHandler
}

// Close closes the transport and ends the read loop.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.setReadErr(ErrClosed)
		_ = c.conn.Close()
		c.finishShutdown(ErrClosed)
	})
	return nil
}
