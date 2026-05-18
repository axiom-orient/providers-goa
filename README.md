# goa

`goa` is a Go module with a public `client` SDK, the `appserver` stdio JSON-RPC client, and the `goa` CLI.

## quick start

```bash
go test ./...
go vet ./...
go list ./...
go build ./...
go run ./cmd/goa --help
go run ./cmd/goa version
```

Model listing:

```bash
export OPENAI_API_KEY=sk-...
go run ./cmd/goa models list
```

Single response:

```bash
go run ./cmd/goa responses create \
  --model <model-id> \
  --input "Say hello in one sentence."
```

Browser re-login:

```bash
go run ./cmd/goa relogin --no-browser --callback-port 0
```

Streaming response:

```bash
go run ./cmd/goa send \
  --model <model-id> \
  --input "Count from 1 to 5." \
  --stream
```

Optional request headers:

```bash
go run ./cmd/goa send \
  --model <model-id> \
  --input "Say hello." \
  --organization <org_id> \
  --project <project_id> \
  --client-request-id $(uuidgen)
```

## packages

- `github.com/axiom-orient/providers-goa/client`: OpenAI Responses + ChatGPT backend SDK
- `github.com/axiom-orient/providers-goa/appserver`: Codex app-server stdio JSON-RPC client
- `github.com/axiom-orient/providers-goa/cmd/goa`: CLI entrypoint

## supports

- `GET /v1/models`
- `POST /v1/responses`
- SSE response streaming
- structured output helpers for `text.format` JSON schema / JSON object modes
- response `request_id` metadata and optional org/project/client request headers
- auth discovery from `OPENAI_API_KEY` and `auth.json`
- ChatGPT backend model listing and response sends via `auth.json`
- safe ChatGPT auth refresh on model-list preflight with schema-aware `auth.json` persistence
- browser OAuth re-login via `client.ReloginBrowser` and `goa relogin`
- Codex app-server JSON-RPC client under `appserver/`

## module path policy

The public module path is `github.com/axiom-orient/providers-goa`.

## docs
- `docs/README.md`: current package notes, boundaries, change policy, and verification commands
