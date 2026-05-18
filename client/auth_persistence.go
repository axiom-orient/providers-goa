package client

import (
	"errors"
	"os"

	"github.com/axiom-orient/providers-goa/client/internal/authjson"
)

// AuthFileWriteResult summarizes a file-based auth cache write.
type AuthFileWriteResult struct {
	Path    string `json:"path"`
	Written bool   `json:"written"`
}

// ReadAuthFileBytes reads a file-based auth cache without interpreting its schema.
func ReadAuthFileBytes(path string) ([]byte, error) {
	if path == "" {
		return nil, &ValidationError{Field: "path", Message: "must not be empty"}
	}
	return os.ReadFile(path)
}

// WriteAuthFileBytes writes a file-based auth cache as opaque bytes.
func WriteAuthFileBytes(path string, data []byte) (AuthFileWriteResult, error) {
	if path == "" {
		return AuthFileWriteResult{}, &ValidationError{Field: "path", Message: "must not be empty"}
	}
	if err := authjson.WritePrivateFile(path, data); err != nil {
		return AuthFileWriteResult{}, err
	}
	return AuthFileWriteResult{Path: path, Written: true}, nil
}

// SeedAuthFileIfMissing writes a file-based auth cache only when the target file does not already exist.
func SeedAuthFileIfMissing(path string, data []byte) (AuthFileWriteResult, error) {
	if path == "" {
		return AuthFileWriteResult{}, &ValidationError{Field: "path", Message: "must not be empty"}
	}
	_, statErr := os.Stat(path)
	if statErr == nil {
		return AuthFileWriteResult{Path: path, Written: false}, nil
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return AuthFileWriteResult{}, statErr
	}
	if err := authjson.WritePrivateFile(path, data); err != nil {
		return AuthFileWriteResult{}, err
	}
	return AuthFileWriteResult{Path: path, Written: true}, nil
}

// WriteResolvedAuthFile resolves auth.json and writes it as opaque bytes.
func WriteResolvedAuthFile(opts ResolveAuthOptions, data []byte) (AuthFileWriteResult, error) {
	path, err := ResolveAuthPath(opts)
	if err != nil {
		return AuthFileWriteResult{}, err
	}
	return WriteAuthFileBytes(path, data)
}

// SeedResolvedAuthFileIfMissing resolves auth.json and seeds it only when missing.
func SeedResolvedAuthFileIfMissing(opts ResolveAuthOptions, data []byte) (AuthFileWriteResult, error) {
	path, err := ResolveAuthPath(opts)
	if err != nil {
		return AuthFileWriteResult{}, err
	}
	return SeedAuthFileIfMissing(path, data)
}
