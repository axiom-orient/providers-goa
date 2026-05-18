package appserver_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/axiom-orient/providers-goa/appserver"
)

type stalledConn struct {
	mu     sync.Mutex
	writes bytes.Buffer
	closed chan struct{}
	once   sync.Once
}

func newStalledConn() *stalledConn {
	return &stalledConn{closed: make(chan struct{})}
}

func (c *stalledConn) Read(_ []byte) (int, error) {
	<-c.closed
	return 0, io.EOF
}

func (c *stalledConn) Write(p []byte) (int, error) {
	select {
	case <-c.closed:
		return 0, io.ErrClosedPipe
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes.Write(p)
}

func (c *stalledConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func TestNewClientWithContextBoundsInitializeHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	client, err := appserver.NewClientWithContext(ctx, newStalledConn(), appserver.ClientOptions{})
	if client != nil {
		_ = client.Close()
		t.Fatalf("expected nil client, got %#v", client)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}
