.PHONY: build test cover fmt vet check clean lint mutation \
        verify verify-core verify-vuln verify-k8s verify-fips \
        cover-check fuzz-short build-tags http-smoke stdio-smoke \
        doctor-strict-smoke verify-doctor-strict \
        secret-scan config-parity bench verify-bench bench-baseline-check \
        build-postgres test-postgres build-grpc build-grpc-postgres \
        gen-tool-catalog catalog-drift doc-parity launch-checklist-parity config-doc-parity \
        grpc-release-parity \
        repo-hygiene script-tests release-check

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

verify-core: fmt vet lint test cover-check fuzz-short build-tags http-smoke stdio-smoke verify-doctor-strict grpc-auth-smoke config-parity catalog-drift doc-parity config-doc-parity grpc-release-parity repo-hygiene script-tests

# doc-parity enforces that every MCP_/CLOCKIFY_ env var referenced
# in docs/ exists in the source, every tool name surfaces in the
# generated catalog, and no TODO/FIXME/TBD markers are left in
# operator-facing docs. See scripts/check-doc-parity.sh for the
# exact heuristics.
doc-parity:
	bash scripts/check-doc-parity.sh
	bash scripts/check-launch-checklist-parity.sh

launch-checklist-parity:
	bash scripts/check-launch-checklist-parity.sh

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

# script-tests runs regression tests for repo shell scripts whose
# output contract matters (currently: filter-bench-output.sh, which
# `make bench` pipes raw `go test -bench` output through to produce
# benchstat-compatible profiles). Pure bash, runs in milliseconds.
script-tests:
	bash scripts/test-filter-bench-output.sh

# gen-tool-catalog regenerates docs/tool-catalog.{json,md} from the
# live registry. Run after adding, removing, or changing any tool
# descriptor (including InputSchema edits) — the catalog-drift gate
# refuses to merge an unrefreshed catalog.
gen-tool-catalog:
	go run ./scripts/gen-tool-catalog -out docs

# catalog-drift re-runs gen-tool-catalog and fails if the generated
# docs change relative to the current working tree contents. Wired
# into verify-core so a PR that forgets to regenerate is caught
# before merge, while still allowing local validation on a branch
# with legitimate README / docs edits in flight.
catalog-drift:
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before"; \
	 cp docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before"; \
	 $(MAKE) --no-print-directory gen-tool-catalog >/dev/null; \
	 diff -q docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before" >/dev/null \
	  && diff -q docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before" >/dev/null \
	  || { echo "[catalog-drift] docs/tool-catalog.{json,md} are stale — run \`make gen-tool-catalog\` and commit"; \
	       diff -u "$$tmpdir/tool-catalog.md.before" docs/tool-catalog.md | head -80; exit 1; }

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
	@tmp="$$(mktemp "$${TMPDIR:-/tmp}/clockify-coverage.XXXXXX")"; \
	 trap 'rm -f "$$tmp"' EXIT; \
	 COVERAGE_OUT="$$tmp" bash scripts/check-coverage.sh

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

doctor-strict-smoke verify-doctor-strict:
	bash scripts/smoke-doctor-strict.sh

grpc-auth-smoke:
	bash scripts/smoke-grpc-auth.sh

# grpc-release-parity enforces that the private-network gRPC profile
# documentation, GoReleaser config, asset-count script, and Dockerfile
# stay coherent. Wired into verify-core so a doc that mentions a gRPC
# artifact the release pipeline does not produce fails before tag time.
grpc-release-parity:
	bash scripts/check-grpc-release-parity.sh

# Sanity-build the gRPC-tagged binaries locally. Mirrors the tag matrix
# .goreleaser.yaml ships and keeps `make verify` honest about the
# private-network gRPC profile actually compiling against the working
# tree (the default build path leaves `-tags=grpc` untested).
build-grpc:
	go build -tags=grpc ./...

build-grpc-postgres:
	cd internal/controlplane/postgres && go build -tags=postgres ./...
	go build -tags=grpc,postgres ./cmd/clockify-mcp

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

# release-check composes every pre-ship gate into one laptop-runnable
# target. Humans run this before tagging; CI runs equivalent steps
# across the jobs in ci.yml + release.yml. A single green
# "release-check: OK — shippable" line is the one-word answer to
# "is this repo shippable right now?".
#
# Skips optional tiers (vuln/k8s/fips) when their tools are absent —
# verify-* tiers print a skip notice and return 0 — so the gate
# still runs on a minimal toolchain.
release-check:
	@echo "== release-check: formatting, vet, lint =="
	$(MAKE) fmt vet lint
	@echo "== release-check: tests + coverage floors =="
	$(MAKE) cover-check
	@echo "== release-check: config + doc parity =="
	$(MAKE) config-parity doc-parity config-doc-parity catalog-drift grpc-release-parity
	@echo "== release-check: hygiene + build-tag wiring =="
	$(MAKE) repo-hygiene script-tests build-tags http-smoke stdio-smoke
	@echo "== release-check: strict doctor smoke =="
	$(MAKE) verify-doctor-strict
	@echo "== release-check: full E2E (includes gRPC under -tags=grpc) =="
	go test -tags=grpc -race -count=1 -timeout 180s ./tests/...
	@echo "== release-check: deploy render =="
	@if command -v kubectl >/dev/null 2>&1 && command -v kubeconform >/dev/null 2>&1 && command -v helm >/dev/null 2>&1; then \
		bash scripts/check-k8s-render.sh; \
	else \
		echo "[release-check] kubectl/kubeconform/helm missing — skipping k8s render (CI runs the full check)."; \
	fi
	@echo "release-check: OK — shippable"

# Benchmark capture + regression gate.
#
# `make bench` runs every package benchmark and writes a text profile
# to the path in BENCH_OUT (default .bench/after.txt). `make verify-bench`
# compares that profile to the committed CI baseline at
# internal/benchdata/baseline.txt via benchstat. The workflow:
#
#   # ... make change ...
#   make verify-bench                          # capture .bench/after.txt
#                                              # and compare to CI baseline
#
# For ad hoc same-machine before/after checks, explicitly override
# BENCH_BASELINE=.bench/baseline.txt after recording that local baseline.
# Do not commit or treat workstation baselines as release evidence.
#
# benchstat is installed on demand if missing. The comparison uses the
# default p=0.05 threshold; sensitive packages can tighten it manually
# with benchstat -alpha=0.01 etc.
BENCH_OUT ?= .bench/after.txt
BENCH_BASELINE ?= internal/benchdata/baseline.txt
BENCH_PKGS ?= ./internal/...

bench:
	@mkdir -p $(dir $(BENCH_OUT))
	@raw="$$(mktemp "$${TMPDIR:-/tmp}/clockify-bench.XXXXXX")"; \
	 trap 'rm -f "$$raw"' EXIT; \
	 if ! go test -run=^$$ -bench=. -benchmem -count=5 $(BENCH_PKGS) > "$$raw" 2>&1; then \
	   cat "$$raw"; \
	   exit 1; \
	 fi; \
	 bash scripts/filter-bench-output.sh < "$$raw" | tee "$(BENCH_OUT)"; \
	 echo "benchmarks collected:"; \
	 grep -c '^Benchmark' "$(BENCH_OUT)" || true

verify-bench: bench
	@if [ ! -f $(BENCH_BASELINE) ]; then \
		echo "[verify-bench] baseline $(BENCH_BASELINE) not present."; \
		echo "              The default baseline is committed at internal/benchdata/baseline.txt."; \
		echo "              For local-only experiments, pass BENCH_BASELINE=.bench/baseline.txt explicitly."; \
		exit 1; \
	fi
	@if [ "$(BENCH_BASELINE)" = "internal/benchdata/baseline.txt" ]; then \
		bash scripts/check-bench-baseline.sh "$(BENCH_BASELINE)"; \
		base_platform="$$(awk '/^goos: / { goos=$$2 } /^goarch: / { goarch=$$2 } /^pkg: / { print goos "/" goarch; exit }' "$(BENCH_BASELINE)")"; \
		out_platform="$$(awk '/^goos: / { goos=$$2 } /^goarch: / { goarch=$$2 } /^pkg: / { print goos "/" goarch; exit }' "$(BENCH_OUT)")"; \
		if [ -z "$$base_platform" ] || [ -z "$$out_platform" ]; then \
			echo "[verify-bench] unable to read benchmark platform metadata."; \
			exit 1; \
		fi; \
		if [ "$$base_platform" != "$$out_platform" ]; then \
			echo "[verify-bench] benchmark output platform $$out_platform does not match committed baseline $$base_platform."; \
			echo "              Run the CI bench workflow for release evidence, or pass BENCH_BASELINE=.bench/baseline.txt for explicit same-machine experiments."; \
			exit 1; \
		fi; \
	fi
	@BENCHSTAT="$$(command -v benchstat 2>/dev/null)"; \
	 if [ -z "$$BENCHSTAT" ]; then \
	   echo "[verify-bench] benchstat not in PATH, installing..."; \
	   go install golang.org/x/perf/cmd/benchstat@latest; \
	   BENCHSTAT="$$(go env GOPATH)/bin/benchstat"; \
	 fi; \
	 echo "== benchstat $(BENCH_BASELINE) vs $(BENCH_OUT) =="; \
	 "$$BENCHSTAT" $(BENCH_BASELINE) $(BENCH_OUT)

bench-baseline-check:
	bash scripts/check-bench-baseline.sh

# Local mutation testing via gremlins.dev (W2-10). Per-package
# efficacy floors live in .github/workflows/mutation.yml (top-of-file
# comment table + matrix entries); CI runs the same tool nightly.
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
	# INTEGRATION_REQUIRED=1 turns a Testcontainers failure into t.Fatal
	# instead of t.Skip. Without it, the suite would report green when
	# Docker is unreachable, masking regressions in the postgres
	# control-plane backend. See store_test.go::dsn for the gate.
	cd internal/controlplane/postgres && INTEGRATION_REQUIRED=1 go test -tags=postgres,integration -count=1 -timeout 180s ./...
