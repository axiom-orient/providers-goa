package appserver

import (
	"context"
	"errors"
	"io"
)

// NewClient initializes a stable app-server client on an existing transport.
func NewClient(conn io.ReadWriteCloser, opts ClientOptions) (*Client, error) {
	return NewClientWithContext(context.Background(), conn, opts)
}

// NewClientWithContext initializes a stable app-server client and bounds the startup handshake with ctx.
func NewClientWithContext(ctx context.Context, conn io.ReadWriteCloser, opts ClientOptions) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if conn == nil {
		return nil, errors.New("appserver: nil transport")
	}
	client := newClientRuntime(conn, opts)
	if err := client.initialize(ctx, opts); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func newClientRuntime(conn io.ReadWriteCloser, opts ClientOptions) *Client {
	client := &Client{
		conn:           conn,
		pending:        make(map[int64]chan pendingResponse),
		notifications:  make(chan Notification, 32),
		closed:         make(chan struct{}),
		refreshHandler: opts.RefreshHandler,
	}
	go client.readLoop()
	return client
}

func (c *Client) initialize(ctx context.Context, opts ClientOptions) error {
	params := initializeParams{ClientInfo: normalizeClientInfo(opts.ClientInfo)}
	if caps := normalizeCapabilities(opts.Capabilities); caps != nil {
		params.Capabilities = caps
	}
	var initResult InitializeResult
	if err := c.callWithID(ctx, 0, "initialize", params, &initResult); err != nil {
		return err
	}
	c.initResult = initResult
	return c.send(map[string]any{"method": "initialized", "params": map[string]any{}})
}
