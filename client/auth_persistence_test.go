package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAuthFileBytesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".codex", "auth.json")
	want := []byte(`{"token":"abc"}`)

	result, err := WriteAuthFileBytes(path, want)
	if err != nil {
		t.Fatalf("WriteAuthFileBytes() error = %v", err)
	}
	if !result.Written || result.Path != path {
		t.Fatalf("unexpected result: %+v", result)
	}
	got, err := ReadAuthFileBytes(path)
	if err != nil {
		t.Fatalf("ReadAuthFileBytes() error = %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("unexpected file contents: %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perms := info.Mode().Perm(); perms != 0o600 {
		t.Fatalf("unexpected file permissions: %o", perms)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perms := dirInfo.Mode().Perm(); perms != 0o700 {
		t.Fatalf("unexpected dir permissions: %o", perms)
	}
}

func TestSeedAuthFileIfMissingDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if _, err := WriteAuthFileBytes(path, []byte(`{"token":"first"}`)); err != nil {
		t.Fatalf("initial write: %v", err)
	}
	result, err := SeedAuthFileIfMissing(path, []byte(`{"token":"second"}`))
	if err != nil {
		t.Fatalf("SeedAuthFileIfMissing() error = %v", err)
	}
	if result.Written {
		t.Fatalf("expected seed to skip existing file: %+v", result)
	}
	got, err := ReadAuthFileBytes(path)
	if err != nil {
		t.Fatalf("ReadAuthFileBytes() error = %v", err)
	}
	if string(got) != `{"token":"first"}` {
		t.Fatalf("unexpected file contents after seed: %q", got)
	}
}

func TestWriteResolvedAuthFileUsesAuthHome(t *testing.T) {
	dir := t.TempDir()
	result, err := WriteResolvedAuthFile(ResolveAuthOptions{AuthHome: dir}, []byte(`{"token":"abc"}`))
	if err != nil {
		t.Fatalf("WriteResolvedAuthFile() error = %v", err)
	}
	wantPath := filepath.Join(dir, "auth.json")
	if result.Path != wantPath {
		t.Fatalf("unexpected path: %q", result.Path)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("stat written file: %v", err)
	}
}
