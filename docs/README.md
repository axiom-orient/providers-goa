# goa docs

`goa` is the Go provider package. It contains the `client` SDK, the `goa` CLI, and the auxiliary `appserver` JSON-RPC package.

## Package Map

- `go.mod`: local module path is `goa`; public Go releases must rewrite this to a real VCS module path.
- `client/`: SDK core for auth discovery, model listing, Responses create/send, streaming, structured output, diagnostics, refresh, and browser relogin.
- `client/internal/authjson/`: schema-aware `auth.json` rewrite helpers.
- `client/internal/chatgptwire/`: ChatGPT backend wire parsing.
- `cmd/goa/`: CLI adapter over `client`.
- `appserver/`: separate public package for Codex app-server stdio JSON-RPC.

## Boundaries

- CLI code must call the SDK instead of duplicating auth or HTTP transport.
- Only `client/internal/authjson` should rewrite known `auth.json` fields.
- ChatGPT response sends must not auto-retry mutation `POST /responses` after a 401.
- Browser relogin is explicit SDK/CLI behavior, not a hidden send side effect.
- `appserver` is public but auxiliary; provider core behavior lives in `client`.

## Verification

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
go build ./...
go run ./cmd/goa --help
```

## GitHub Release

Target repository and module path:

```text
github.com/axiom-orient/providers-goa
```

Before tagging a public Go release, set the module path in the split GitHub repository to `github.com/axiom-orient/providers-goa`.

## Known Release Constraint

The checked-in module path `goa` is local-only. The GitHub release copy must use `github.com/axiom-orient/providers-goa` before it is tagged.
