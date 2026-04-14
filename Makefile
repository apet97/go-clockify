.PHONY: build test cover fmt vet check clean lint mutation \
        verify verify-core verify-vuln verify-k8s verify-fips \
        cover-check fuzz-short build-tags http-smoke stdio-smoke config-parity

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

# Fast inner-loop check — seconds, not minutes. Use `make verify` before a PR.
check: fmt vet test

# Full local verification pipeline. Mirrors the PR-blocking CI jobs where
# possible. Tool-gated tiers (vuln/k8s/fips) auto-skip with a warning when
# their dependencies are missing; see CONTRIBUTING.md for the exact list of
# checks `make verify` runs locally versus the full CI set.
verify: verify-core verify-vuln verify-k8s verify-fips

verify-core: fmt vet lint test cover-check fuzz-short build-tags http-smoke stdio-smoke config-parity

verify-vuln:
	@if command -v govulncheck >/dev/null 2>&1; then \
		echo "== govulncheck =="; \
		govulncheck ./...; \
	else \
		echo "[verify-vuln] govulncheck not installed, skipping."; \
		echo "              Install: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

verify-k8s:
	@if command -v kubectl >/dev/null 2>&1 && command -v kubeconform >/dev/null 2>&1 && command -v helm >/dev/null 2>&1; then \
		bash scripts/check-k8s-render.sh; \
	else \
		echo "[verify-k8s] kubectl/kubeconform/helm missing, skipping."; \
		echo "             Install: brew install kubernetes-cli kubeconform helm"; \
	fi

verify-fips:
	@FIPS_ONLY=1 bash scripts/check-build-tags.sh || { \
		echo "[verify-fips] FIPS build failed — local Go toolchain may lack GOFIPS140 support."; \
		echo "              This step runs in CI; local failure is non-fatal."; \
	}

cover-check:
	bash scripts/check-coverage.sh

# Short fuzz budget for local runs. CI uses 30s per target (see ci.yml).
fuzz-short:
	go test -fuzz=FuzzParseDatetime -fuzztime=10s ./internal/timeparse
	go test -fuzz=FuzzValidateID   -fuzztime=10s ./internal/resolve
	go test -fuzz=FuzzJSONRPCParse -fuzztime=10s ./internal/mcp

build-tags:
	SKIP_FIPS=1 bash scripts/check-build-tags.sh

http-smoke:
	bash scripts/smoke-http.sh

stdio-smoke:
	bash scripts/smoke-stdio.sh

config-parity:
	bash scripts/check-config-parity.sh

clean:
	rm -f clockify-mcp coverage.out

# Local mutation testing via gremlins.dev (W2-10). Floors live in
# docs/testing/mutation-floors.md; CI runs the same tool nightly.
# Usage: `make mutation PKG=internal/jsonschema`
PKG ?= internal/jsonschema
mutation:
	@which gremlins > /dev/null 2>&1 || go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
	gremlins unleash ./$(PKG)
