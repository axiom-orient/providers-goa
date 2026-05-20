# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog, and this project follows Semantic Versioning for public releases.

## [Unreleased]

## [v0.2.0] - 2026-05-20

### Changed
- Public CLI is now explicit provider paths: `goa codex ...` and `goa gemini ...`.
- Codex CLI LLM calls use ChatGPT/Codex credentials from the resolved `auth.json`.
- Removed public API-key CLI guidance and root-level legacy command shortcuts.
- Added package-local Gemini Core Adapter support for `gemini models` and `gemini generate`.

### Verified
- `go test ./...`
- `go vet ./...`
- `go list ./...`
- `go build ./...`
- `cd gemini-core-adapter && npm install && npm run build && npm audit --omit=dev`
- Live Codex and Gemini credential smoke checks.

## [v0.1.0] - 2026-05-16

### Added
- Browser OAuth relogin SDK/CLI path is now part of the documented provider contract.
- Provider notes under `docs/README.md`.
- Nil-context guards for request construction and client request ID propagation.
- CLI coverage for refusal, failed response, streaming error, and JSON request-id propagation.
- App-server stdio environment inheritance regression coverage.

### Changed
- CLI help coverage now locks `goa relogin` into the public command surface.
- Stale brownfield/analysis docs were replaced by compact provider docs.
- Project/module import surface now uses `module github.com/axiom-orient/providers-goa` for the public GitHub package.
- Public release target is documented as `github.com/axiom-orient/providers-goa`.
- Streaming lifecycle handling now treats `failed`, `incomplete`, `cancelled`, and explicit error events as terminal.
- CLI now prints refusal text, preserves streamed `request_id`, and exits non-zero on terminal response failure.
- `internal/cli`, `protocol`, and `appserver/client` were split into smaller files to reduce mixed responsibility.
