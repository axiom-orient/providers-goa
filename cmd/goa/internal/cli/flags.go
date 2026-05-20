package cli

import (
	"context"
	"flag"

	goa "github.com/axiom-orient/providers-goa/client"
)

type clientFlags struct {
	baseURL         string
	authPath        string
	authHome        string
	issuer          string
	clientVersion   string
	organization    string
	project         string
	clientRequestID string
}

func bindClientFlags(fs *flag.FlagSet, includeRequestID bool) *clientFlags {
	flags := &clientFlags{}
	fs.StringVar(&flags.baseURL, "base-url", "", "override Responses base URL")
	fs.StringVar(&flags.organization, "organization", "", "optional organization header")
	fs.StringVar(&flags.project, "project", "", "optional project header")
	if includeRequestID {
		fs.StringVar(&flags.clientRequestID, "client-request-id", "", "X-Client-Request-Id header")
	}
	fs.StringVar(&flags.authPath, "auth-path", "", "path to auth.json")
	fs.StringVar(&flags.authHome, "auth-home", "", "directory containing auth.json")
	fs.StringVar(&flags.issuer, "issuer", "", "OAuth issuer URL")
	fs.StringVar(&flags.clientVersion, "client-version", "", "ChatGPT backend models client_version")
	return flags
}

func (f *clientFlags) config() goa.Config {
	if f == nil {
		return goa.Config{}
	}
	return goa.Config{
		BaseURL:              f.baseURL,
		AuthPath:             f.authPath,
		AuthHome:             f.authHome,
		AuthIssuerURL:        f.issuer,
		ChatGPTClientVersion: f.clientVersion,
		PreferChatGPT:        true,
		Organization:         f.organization,
		Project:              f.project,
	}
}

func (f *clientFlags) context(ctx context.Context) context.Context {
	if f == nil {
		return goa.WithClientRequestID(ctx, "")
	}
	return goa.WithClientRequestID(ctx, f.clientRequestID)
}
