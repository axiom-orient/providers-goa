//go:build unix

package client

import (
	"os"
	"path/filepath"
	"syscall"
)

func withAuthRefreshFileLock(path string, fn func() error) error {
	if path == "" {
		return &ValidationError{Field: "path", Message: "must not be empty"}
	}
	lockPath := path + ".refresh.lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return err
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := lockFile.Chmod(0o600); err != nil {
		_ = lockFile.Close()
		return err
	}
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		_ = lockFile.Close()
		return err
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		_ = lockFile.Close()
	}()
	return fn()
}
