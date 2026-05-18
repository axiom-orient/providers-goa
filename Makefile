.PHONY: fmt test vet list run-help version check

fmt:
	find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w

test:
	go test ./...

vet:
	go vet ./...

run-help:
	go run ./cmd/goa --help

check: fmt test vet list run-help

list:
	go list ./...

version:
	go run ./cmd/goa version
