# goa docs

`goa` is the Go provider package. It contains the `client` SDK, the `goa` CLI, the package-local Gemini Core Adapter, and the auxiliary `appserver` JSON-RPC package.

## Package Map

- `go.mod`: public module path `github.com/axiom-orient/providers-goa`.
- `client/`: SDK core for auth discovery, model listing, Responses create/send, streaming, structured output, diagnostics, refresh, browser relogin, and Gemini adapter calls.
- `client/internal/authjson/`: schema-aware `auth.json` rewrite helpers.
- `client/internal/chatgptwire/`: ChatGPT backend request and SSE wire parsing.
- `cmd/goa/`: CLI adapter over `client`.
- `gemini-core-adapter/`: TypeScript JSON-RPC adapter around upstream `@google/gemini-cli-core`.
- `appserver/`: separate public package for Codex app-server stdio JSON-RPC.

## Boundaries

- Public CLI commands live under `goa codex ...` and `goa gemini ...`.
- Codex CLI LLM calls use ChatGPT/Codex credentials from the resolved `auth.json`.
- CLI code must call the SDK instead of duplicating auth or HTTP transport.
- Only `client/internal/authjson` should rewrite known `auth.json` fields.
- Gemini code must not read Codex auth, invoke Codex relogin, or use Codex Responses transports.
- ChatGPT response sends must not auto-retry mutation `POST /responses` after a 401.
- Browser relogin is explicit SDK/CLI behavior, not a hidden send side effect.
- `appserver` is public but auxiliary; provider core behavior lives in `client`.
- GitHub CI is not part of this repository's verification workflow.

## Verification

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*' -not -path './gemini-core-adapter/node_modules/*')
go test ./...
go vet ./...
go list ./...
go build ./...
cd gemini-core-adapter && npm install && npm run build && npm audit --omit=dev && cd ..
go run ./cmd/goa --help
```
