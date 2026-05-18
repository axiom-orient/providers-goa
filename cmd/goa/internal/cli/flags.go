package cli

import (
	"context"
	"flag"

	goa "github.com/axiom-orient/providers-goa/client"
)

type clientFlags struct {
	apiKey          string
	baseURL         string
	authPath        string
	authHome        string
	organization    string
	project         string
	clientRequestID string
}

func bindClientFlags(fs *flag.FlagSet, includeRequestID bool) *clientFlags {
	flags := &clientFlags{}
	fs.StringVar(&flags.apiKey, "api-key", "", "OpenAI API key")
	fs.StringVar(&flags.baseURL, "base-url", "", "override API base URL")
	fs.StringVar(&flags.organization, "organization", "", "OpenAI organization header")
	fs.StringVar(&flags.project, "project", "", "OpenAI project header")
	if includeRequestID {
		fs.StringVar(&flags.clientRequestID, "client-request-id", "", "X-Client-Request-Id header")
	}
	fs.StringVar(&flags.authPath, "auth-path", "", "path to auth.json")
	fs.StringVar(&flags.authHome, "auth-home", "", "directory containing auth.json")
	return flags
}

func (f *clientFlags) config() goa.Config {
	if f == nil {
		return goa.Config{}
	}
	return goa.Config{
		APIKey:       f.apiKey,
		BaseURL:      f.baseURL,
		AuthPath:     f.authPath,
		AuthHome:     f.authHome,
		Organization: f.organization,
		Project:      f.project,
	}
}

func (f *clientFlags) context(ctx context.Context) context.Context {
	if f == nil {
		return goa.WithClientRequestID(ctx, "")
	}
	return goa.WithClientRequestID(ctx, f.clientRequestID)
}
