package client

import "testing"

func TestBuildInfoSnapshotDefaults(t *testing.T) {
	prevVersion, prevCommit, prevBuildDate := Version, Commit, BuildDate
	defer func() {
		Version = prevVersion
		Commit = prevCommit
		BuildDate = prevBuildDate
	}()

	Version = ""
	Commit = ""
	BuildDate = ""

	info := BuildInfoSnapshot()
	if info.Version != "dev" {
		t.Fatalf("unexpected version: %q", info.Version)
	}
	if info.Commit != "unknown" {
		t.Fatalf("unexpected commit: %q", info.Commit)
	}
	if info.BuildDate != "unknown" {
		t.Fatalf("unexpected build date: %q", info.BuildDate)
	}
	if got := DefaultUserAgent(); got != "github.com/axiom-orient/providers-goa/dev" {
		t.Fatalf("unexpected user agent: %q", got)
	}
}
