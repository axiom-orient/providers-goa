# goa

`goa` is a Go module and CLI for explicit provider paths:

- `goa codex ...` uses ChatGPT/Codex credentials from the resolved `auth.json`.
- `goa gemini ...` uses the package-local Gemini Core Adapter around upstream `@google/gemini-cli-core`.
- `appserver/` remains a separate Codex app-server stdio JSON-RPC package.

## Build

```bash
go test ./...
go vet ./...
go list ./...
go build ./...

cd gemini-core-adapter
npm install
npm run build
npm audit --omit=dev
cd ..
```

## CLI

```bash
go run ./cmd/goa --help

go run ./cmd/goa codex auth status
go run ./cmd/goa codex models list
go run ./cmd/goa codex send "Reply with exactly OK" --model gpt-5.4
go run ./cmd/goa codex send "Count from 1 to 3" --stream --model gpt-5.4
printf 'Reply with exactly OK\n' | go run ./cmd/goa codex send --stdin --model gpt-5.4

go run ./cmd/goa codex relogin --no-browser --callback-port 1455
go run ./cmd/goa codex --auth-path /tmp/auth.json auth status
go run ./cmd/goa codex --client-version 0.130.0 models list

go run ./cmd/goa gemini models
go run ./cmd/goa gemini generate "Reply with exactly OK" --model flash
```

Add global `--json` for automation:

```bash
go run ./cmd/goa --json codex auth status
go run ./cmd/goa --json codex send "Reply with exactly OK" --model gpt-5.4
go run ./cmd/goa --json gemini generate "Reply with exactly OK" --model flash
```

## Runtime Rules

- Codex auth resolves from explicit `--auth-path`, explicit `--auth-home`, `$CODEX_HOME/auth.json`,
  then `~/.codex/auth.json`.
- The CLI uses ChatGPT/Codex credentials in the resolved `auth.json` for the main LLM call path.
- `goa gemini ...` uses Gemini CLI Core's own Google/Gemini auth path through the adapter.
- Codex and Gemini do not share login state, model discovery, or transport code.
- Gemini commands require `gemini-core-adapter/dist/main.js`, built with `npm run build` inside
  `gemini-core-adapter`.

## Packages

- `github.com/axiom-orient/providers-goa/client`: Go SDK for Codex/Responses and Gemini adapter clients.
- `github.com/axiom-orient/providers-goa/appserver`: Codex app-server stdio JSON-RPC client.
- `github.com/axiom-orient/providers-goa/cmd/goa`: CLI entrypoint.

## Docs

- `docs/README.md`: current package notes, boundaries, change policy, and verification commands.
