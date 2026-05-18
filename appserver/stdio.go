package appserver

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// StdioConfig configures DialStdio.
type StdioConfig struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Options ClientOptions
}

// DialStdio starts codex app-server on stdio and performs initialization.
func DialStdio(ctx context.Context, cfg StdioConfig) (*Client, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, errors.New("appserver: command is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = cfg.Dir
	if cfg.Env != nil {
		cmd.Env = append([]string(nil), cfg.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	conn := &stdioConn{stdin: stdin, stdout: stdout, cmd: cmd}
	client, err := NewClientWithContext(ctx, conn, cfg.Options)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

type stdioConn struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	cmd    *exec.Cmd

	closeOnce sync.Once
}

func (c *stdioConn) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *stdioConn) Write(p []byte) (int, error) { return c.stdin.Write(p) }
func (c *stdioConn) Close() error {
	c.closeOnce.Do(func() {
		_ = c.stdin.Close()
		_ = c.stdout.Close()
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
			_, _ = c.cmd.Process.Wait()
		}
	})
	return nil
}
