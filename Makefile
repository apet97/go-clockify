.PHONY: build test cover fmt vet check clean lint mutation \
        verify verify-core verify-vuln verify-k8s verify-fips \
        cover-check fuzz-short build-tags http-smoke stdio-smoke \
        secret-scan config-parity bench verify-bench \
        build-postgres test-postgres \
        gen-tool-catalog catalog-drift doc-parity config-doc-parity \
        repo-hygiene

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

verify-core: fmt vet lint test cover-check fuzz-short build-tags http-smoke stdio-smoke config-parity catalog-drift doc-parity config-doc-parity repo-hygiene

# doc-parity enforces that every MCP_/CLOCKIFY_ env var referenced
# in docs/ exists in the source, every tool name surfaces in the
# generated catalog, and no TODO/FIXME/TBD markers are left in
# operator-facing docs. See scripts/check-doc-parity.sh for the
# exact heuristics.
doc-parity:
	bash scripts/check-doc-parity.sh

# config-doc-parity re-renders cmd/clockify-mcp/help_generated.go and the
# CONFIG-TABLE block in README.md from internal/config/AllSpecs() and
# fails if either drifted. Pair with: go run ./cmd/gen-config-docs
# -mode=all && git add README.md cmd/clockify-mcp/help_generated.go
config-doc-parity:
	bash scripts/check-config-doc-parity.sh

# repo-hygiene fails on tracked OS / editor / coverage junk. See
# scripts/check-repo-hygiene.sh for the exact pattern list; .gitignore
# keeps future stages clean.
repo-hygiene:
	bash scripts/check-repo-hygiene.sh

# gen-tool-catalog regenerates docs/tool-catalog.{json,md} from the
# live registry. Run after adding, removing, or changing any tool
# descriptor (including InputSchema edits) — the catalog-drift gate
# refuses to merge an unrefreshed catalog.
gen-tool-catalog:
	go run ./scripts/gen-tool-catalog -out docs

# catalog-drift re-runs gen-tool-catalog and fails if the working
# tree diverges from what's committed. Wired into verify-core so a
# PR that forgets to regenerate is caught before merge.
catalog-drift: gen-tool-catalog
	@git diff --exit-code -- docs/tool-catalog.json docs/tool-catalog.md \
	 || { echo "[catalog-drift] docs/tool-catalog.{json,md} are stale — run \`make gen-tool-catalog\` and commit"; exit 1; }

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

# Short fuzz budget for local runs. Count-based (-fuzztime=Nx) instead
# of duration-based (-fuzztime=Ns) to sidestep a Go fuzz engine race:
# with -fuzztime=10s, workers mid-execution when the engine's internal
# context deadline hit would bubble up as "context deadline exceeded"
# and fail the target even though no input had actually crashed. The
# race got worse once Wave D committed ~800 corpus seeds to testdata/
# (baseline gathering eats several seconds before mutation starts).
# Count-based budgets are deterministic: no timing race, ~0.7s per
# target on a laptop at ~250k execs/sec.
#
# CI uses the same count via .github/workflows/ci.yml.
fuzz-short:
	go test -fuzz=FuzzParseDatetime -fuzztime=100000x -run='^$$' -timeout=90s ./internal/timeparse
	go test -fuzz=FuzzValidateID   -fuzztime=100000x -run='^$$' -timeout=90s ./internal/resolve
	go test -fuzz=FuzzJSONRPCParse -fuzztime=100000x -run='^$$' -timeout=90s ./internal/mcp

build-tags:
	SKIP_FIPS=1 bash scripts/check-build-tags.sh

http-smoke:
	bash scripts/smoke-http.sh

stdio-smoke:
	bash scripts/smoke-stdio.sh

secret-scan:
	@if ! command -v gitleaks >/dev/null 2>&1; then \
		echo "gitleaks not installed; install via 'brew install gitleaks' or run scripts/gitleaks-install.sh"; \
		exit 1; \
	fi
	gitleaks detect --no-git --source . --redact --config .gitleaks.toml

config-parity:
	bash scripts/check-config-parity.sh

clean:
	rm -f clockify-mcp coverage.out

# Benchmark capture + regression gate.
#
# `make bench` runs every package benchmark and writes a text profile
# to the path in BENCH_OUT (default .bench/after.txt). `make verify-bench`
# compares that profile to .bench/baseline.txt via benchstat and fails
# the target when a CI-significant regression appears. The workflow:
#
#   make bench BENCH_OUT=.bench/baseline.txt   # capture a known-good
#   # ... make change ...
#   make verify-bench                          # capture .bench/after.txt
#                                              # and compare to baseline
#
# benchstat is installed on demand if missing. The gate uses the
# default p=0.05 threshold; sensitive packages can tighten it locally
# by running benchstat manually with -alpha=0.01 etc.
BENCH_OUT ?= .bench/after.txt
BENCH_BASELINE ?= .bench/baseline.txt
BENCH_PKGS ?= ./internal/...

bench:
	@mkdir -p $(dir $(BENCH_OUT))
	go test -run=^$$ -bench=. -benchmem -count=5 $(BENCH_PKGS) | tee $(BENCH_OUT)

verify-bench: bench
	@if [ ! -f $(BENCH_BASELINE) ]; then \
		echo "[verify-bench] baseline $(BENCH_BASELINE) not present — skipping comparison."; \
		echo "              Record one with: make bench BENCH_OUT=$(BENCH_BASELINE)"; \
		exit 0; \
	fi
	@BENCHSTAT="$$(command -v benchstat 2>/dev/null)"; \
	 if [ -z "$$BENCHSTAT" ]; then \
	   echo "[verify-bench] benchstat not in PATH, installing..."; \
	   go install golang.org/x/perf/cmd/benchstat@latest; \
	   BENCHSTAT="$$(go env GOPATH)/bin/benchstat"; \
	 fi; \
	 echo "== benchstat $(BENCH_BASELINE) vs $(BENCH_OUT) =="; \
	 "$$BENCHSTAT" $(BENCH_BASELINE) $(BENCH_OUT)

# Local mutation testing via gremlins.dev (W2-10). Floors live in
# docs/testing/mutation-floors.md; CI runs the same tool nightly.
# Usage: `make mutation PKG=internal/jsonschema`
PKG ?= internal/jsonschema
mutation:
	@which gremlins > /dev/null 2>&1 || go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
	gremlins unleash ./$(PKG)

# Postgres control-plane backend (ADR 0011). The Postgres impl lives in
# internal/controlplane/postgres with its own go.mod so the default
# binary stays stdlib-only per ADR 0001. `build-postgres` compiles the
# tagged binary; `test-postgres` runs the sub-module's integration
# suite (requires Docker for testcontainers-go).
build-postgres:
	go build -tags=postgres ./...
	cd internal/controlplane/postgres && go build -tags=postgres ./... && go vet -tags=postgres ./...

test-postgres:
	cd internal/controlplane/postgres && go test -tags=postgres,integration -count=1 -timeout 180s ./...
