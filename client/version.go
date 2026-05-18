package client

import "strings"

// These variables may be overridden at build time with -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// BuildInfo describes the current build metadata.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

// BuildInfoSnapshot returns the current build metadata.
func BuildInfoSnapshot() BuildInfo {
	info := BuildInfo{
		Version:   strings.TrimSpace(Version),
		Commit:    strings.TrimSpace(Commit),
		BuildDate: strings.TrimSpace(BuildDate),
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	if info.Commit == "" {
		info.Commit = "unknown"
	}
	if info.BuildDate == "" {
		info.BuildDate = "unknown"
	}
	return info
}

// DefaultUserAgent reports the default HTTP user agent for this build.
func DefaultUserAgent() string {
	return "github.com/axiom-orient/providers-goa/" + BuildInfoSnapshot().Version
}
