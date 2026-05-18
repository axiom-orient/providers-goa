//go:build !unix

package client

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	authRefreshLockRetryDelay = 25 * time.Millisecond
	authRefreshLockTimeout    = 5 * time.Second
)

func withAuthRefreshFileLock(path string, fn func() error) error {
	if path == "" {
		return &ValidationError{Field: "path", Message: "must not be empty"}
	}
	lockPath := path + ".refresh.lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}

	deadline := time.Now().Add(authRefreshLockTimeout)
	for {
		lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			lockFile.Close()
			defer os.Remove(lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(authRefreshLockRetryDelay)
	}
}
