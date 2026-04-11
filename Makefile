.PHONY: build test cover fmt vet check clean lint mutation

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o clockify-mcp ./cmd/clockify-mcp

test:
	go test -race -count=1 -timeout 120s ./...

cover:
	go test -race -count=1 -timeout 120s -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

fmt:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files:"; gofmt -l .; gofmt -d .; exit 1)

vet:
	go vet ./...

lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

check: fmt vet test

clean:
	rm -f clockify-mcp coverage.out

# Local mutation testing via gremlins.dev (W2-10). Floors live in
# docs/testing/mutation-floors.md; CI runs the same tool nightly.
# Usage: `make mutation PKG=internal/jsonschema`
PKG ?= internal/jsonschema
mutation:
	@which gremlins > /dev/null 2>&1 || go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
	gremlins unleash ./$(PKG)
