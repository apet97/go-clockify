.PHONY: build test fmt vet check clean bench bench-baseline-check verify-bench lint-test-fragility gen-tool-catalog catalog-drift gen-coverage-dashboard coverage-dashboard-drift gen-openapi openapi-drift gen-raw-allowlist raw-allowlist-drift sync-selfinspect-assets selfinspect-drift mod-tidy-drift api-parity-matrix-drift live-contract-local live-clean-prefix perfect perfect-local perfect-live

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BENCH_COUNT ?= 10
BENCH_PKGS ?= ./internal/clockify ./internal/mcp ./internal/resolve ./internal/timeparse ./internal/tools

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o clockify-mcp ./cmd/clockify-mcp

test:
	go test -race -count=1 -timeout 120s ./...

fmt:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files:"; gofmt -l .; gofmt -d .; exit 1)

vet:
	go vet ./...

check: fmt vet test

clean:
	rm -f clockify-mcp coverage.out

bench:
	go test -run '^$$' -bench=. -benchmem -count=$(BENCH_COUNT) $(BENCH_PKGS)

bench-baseline-check:
	bash scripts/check-bench-baseline.sh

verify-bench: bench-baseline-check
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 go test -run '^$$' -bench=. -benchmem -count=$(BENCH_COUNT) $(BENCH_PKGS) > "$$tmpdir/bench-raw.txt"; \
	 bash scripts/filter-bench-output.sh < "$$tmpdir/bench-raw.txt" > "$$tmpdir/bench.txt"; \
	 go run golang.org/x/perf/cmd/benchstat@latest internal/benchdata/baseline.txt "$$tmpdir/bench.txt"

lint-test-fragility:
	bash scripts/lint-test-fragility.sh

# gen-tool-catalog regenerates docs/tool-catalog.{json,md} and
# docs/default-toolset.{json,md} from the one-user runtime registry.
# Run it after changing any tool descriptor.
gen-tool-catalog:
	go run ./scripts/gen-tool-catalog -out docs

# catalog-drift fails when the committed catalog does not match the runtime
# registry order: workflows first, domain tools second, raw API fallback last.
catalog-drift:
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before"; \
	 cp docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before"; \
	 cp docs/default-toolset.json "$$tmpdir/default-toolset.json.before"; \
	 cp docs/default-toolset.md "$$tmpdir/default-toolset.md.before"; \
	 $(MAKE) --no-print-directory gen-tool-catalog >/dev/null; \
	 diff -q docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before" >/dev/null \
	  && diff -q docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before" >/dev/null \
	  && diff -q docs/default-toolset.json "$$tmpdir/default-toolset.json.before" >/dev/null \
	  && diff -q docs/default-toolset.md "$$tmpdir/default-toolset.md.before" >/dev/null \
	  || { echo "[catalog-drift] docs/tool-catalog/default-toolset files are stale; run make gen-tool-catalog"; \
	       diff -u "$$tmpdir/tool-catalog.md.before" docs/tool-catalog.md | head -80; exit 1; }

gen-coverage-dashboard:
	go run ./scripts/gen-tool-coverage-dashboard --write

coverage-dashboard-drift:
	go run ./scripts/gen-tool-coverage-dashboard

# The raw documented-API fallback is generated from the canonical OpenAPI
# artifact, so keep the artifact easy to refresh and validate.
gen-openapi:
	scripts/gen-clockify-openapi --out docs/openapi/clockify-openapi.yaml

openapi-drift:
	@test -f docs/openapi/clockify-openapi.yaml || { echo "[openapi-drift] docs/openapi/clockify-openapi.yaml missing; run make gen-openapi"; exit 1; }
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp docs/openapi/clockify-openapi.yaml "$$tmpdir/clockify-openapi.yaml.before"; \
	 $(MAKE) --no-print-directory gen-openapi >/dev/null; \
	 scripts/gen-clockify-openapi --validate-only --out docs/openapi/clockify-openapi.yaml >/dev/null; \
	 diff -q docs/openapi/clockify-openapi.yaml "$$tmpdir/clockify-openapi.yaml.before" >/dev/null \
	  || { echo "[openapi-drift] docs/openapi/clockify-openapi.yaml is stale; run make gen-openapi"; \
	       diff -u "$$tmpdir/clockify-openapi.yaml.before" docs/openapi/clockify-openapi.yaml | head -120; exit 1; }

gen-raw-allowlist:
	go run ./scripts/gen-raw-allowlist

raw-allowlist-drift:
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp internal/tools/raw_allowlist_gen.go "$$tmpdir/raw_allowlist_gen.go.before"; \
	 $(MAKE) --no-print-directory gen-raw-allowlist >/dev/null; \
	 diff -q internal/tools/raw_allowlist_gen.go "$$tmpdir/raw_allowlist_gen.go.before" >/dev/null \
	  || { echo "[raw-allowlist-drift] internal/tools/raw_allowlist_gen.go is stale; run make gen-raw-allowlist"; \
	       diff -u "$$tmpdir/raw_allowlist_gen.go.before" internal/tools/raw_allowlist_gen.go | head -120; exit 1; }

sync-selfinspect-assets:
	cp docs/api-parity-matrix.md internal/tools/selfinspect_assets/api-parity-matrix.md
	cp docs/tool-coverage-dashboard.md internal/tools/selfinspect_assets/tool-coverage-dashboard.md
	cp docs/live-tests.md internal/tools/selfinspect_assets/live-tests.md

selfinspect-drift:
	@diff -q docs/api-parity-matrix.md internal/tools/selfinspect_assets/api-parity-matrix.md >/dev/null \
	  || { echo "[selfinspect-drift] api parity asset is stale; run make sync-selfinspect-assets"; exit 1; }
	@diff -q docs/tool-coverage-dashboard.md internal/tools/selfinspect_assets/tool-coverage-dashboard.md >/dev/null \
	  || { echo "[selfinspect-drift] coverage dashboard asset is stale; run make sync-selfinspect-assets"; exit 1; }
	@diff -q docs/live-tests.md internal/tools/selfinspect_assets/live-tests.md >/dev/null \
	  || { echo "[selfinspect-drift] live tests asset is stale; run make sync-selfinspect-assets"; exit 1; }

# mod-tidy-drift fails when `go mod tidy` would change go.mod or go.sum, i.e.
# the committed root-module graph is not tidy. A trap restores go.mod/go.sum on
# every exit path, so the gate is side-effect-free even on failure. GOWORK=off
# keeps the tidy deterministic regardless of the repo-root go.work file. The
# separate tools/govulncheck module is intentionally out of scope.
mod-tidy-drift:
	@tmpdir="$$(mktemp -d)"; \
	 cp go.mod "$$tmpdir/go.mod.before"; \
	 cp go.sum "$$tmpdir/go.sum.before"; \
	 trap 'cp "$$tmpdir/go.mod.before" go.mod; cp "$$tmpdir/go.sum.before" go.sum; rm -rf "$$tmpdir"' EXIT; \
	 if ! GOWORK=off go mod tidy; then \
	   echo "[mod-tidy-drift] 'go mod tidy' failed"; \
	   exit 1; \
	 fi; \
	 drift=0; \
	 diff -u "$$tmpdir/go.mod.before" go.mod || drift=1; \
	 diff -u "$$tmpdir/go.sum.before" go.sum || drift=1; \
	 if [ "$$drift" -ne 0 ]; then \
	   echo "[mod-tidy-drift] go.mod/go.sum are stale; run 'go mod tidy' and commit the result"; \
	   exit 1; \
	 fi

# api-parity-matrix-drift fails when docs/api-parity-matrix.md no longer
# matches the regenerated output of check-api-parity-matrix.sh. The script
# itself exits non-zero when the file would change, so a clean run proves
# the committed matrix is in sync with docs/tool-catalog.json and the
# coverage ledger.
api-parity-matrix-drift:
	bash scripts/check-api-parity-matrix.sh

# live-contract-local intentionally performs real calls against the sacrificial
# Clockify workspace. Do not point it at a production or personal workspace.
live-contract-local:
	@if [ "$${CLOCKIFY_RUN_LIVE_E2E:-}" != "1" ] || [ -z "$${CLOCKIFY_API_KEY:-}" ] || [ -z "$${CLOCKIFY_WORKSPACE_ID:-}" ] || [ -z "$${CLOCKIFY_LIVE_PREFIX:-}" ]; then \
		echo "live-contract-local: set CLOCKIFY_RUN_LIVE_E2E=1, CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX." >&2; \
		echo "  Read docs/live-tests.md before running this target." >&2; \
		exit 1; \
	fi
	go test -v -tags=livee2e -count=1 -timeout 5m \
		-run '^(TestLiveOneUserWorkflowMCP|TestLiveRawClockifyReadSideSchemaDiff)$$' \
		./tests/...
	go test -v -count=1 -timeout 10m ./internal/tools -run '^TestOneUserLive'
	@if [ "$${CLOCKIFY_LIVE_OPTIONAL_DOMAINS:-}" = "1" ]; then \
		echo "== optional livee2e domain campaign =="; \
		go test -v -tags=livee2e -count=1 -timeout 10m \
			-run '^TestLive' \
			./tests/...; \
	else \
		echo "== optional livee2e domain campaign skipped (CLOCKIFY_LIVE_OPTIONAL_DOMAINS != 1) =="; \
	fi

# live-clean-prefix deletes objects in the sacrificial workspace whose name
# starts with CLOCKIFY_LIVE_PREFIX. Read docs/live-tests.md first.
live-clean-prefix:
	@if [ -z "$${CLOCKIFY_API_KEY:-}" ] || [ -z "$${CLOCKIFY_WORKSPACE_ID:-}" ] || [ -z "$${CLOCKIFY_LIVE_PREFIX:-}" ] || [ -z "$${CLOCKIFY_LIVE_WORKSPACE_CONFIRM:-}" ]; then \
		echo "live-clean-prefix: set CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, CLOCKIFY_LIVE_PREFIX, and CLOCKIFY_LIVE_WORKSPACE_CONFIRM." >&2; \
		exit 1; \
	fi
	go run ./scripts/live-clean-prefix

# perfect runs every deterministic gate. It must be green before a release.
perfect:
	go test -race -count=1 -timeout 120s ./...
	$(MAKE) catalog-drift
	$(MAKE) api-parity-matrix-drift
	$(MAKE) coverage-dashboard-drift
	$(MAKE) openapi-drift
	$(MAKE) raw-allowlist-drift
	$(MAKE) selfinspect-drift
	$(MAKE) mod-tidy-drift
	git diff --check

# perfect-local adds the lint and benchmark gates that need extra local tools.
perfect-local: perfect
	golangci-lint run
	$(MAKE) bench-baseline-check

# perfect-live runs the sacrificial-workspace contract suite, then sweeps it.
# Requires the live env vars from docs/live-tests.md.
perfect-live:
	$(MAKE) live-contract-local
	$(MAKE) live-clean-prefix
