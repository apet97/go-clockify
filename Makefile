.PHONY: build test fmt vet check clean bench bench-baseline-check verify-bench gen-tool-catalog catalog-drift gen-openapi openapi-drift gen-raw-allowlist raw-allowlist-drift sync-selfinspect-assets selfinspect-drift api-parity-matrix-drift live-contract-local

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

# gen-tool-catalog regenerates docs/tool-catalog.{json,md} from the
# one-user runtime registry. Run it after changing any tool descriptor.
gen-tool-catalog:
	go run ./scripts/gen-tool-catalog -out docs

# catalog-drift fails when the committed catalog does not match the runtime
# registry order: workflows first, domain tools second, raw API fallback last.
catalog-drift:
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before"; \
	 cp docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before"; \
	 $(MAKE) --no-print-directory gen-tool-catalog >/dev/null; \
	 diff -q docs/tool-catalog.json "$$tmpdir/tool-catalog.json.before" >/dev/null \
	  && diff -q docs/tool-catalog.md "$$tmpdir/tool-catalog.md.before" >/dev/null \
	  || { echo "[catalog-drift] docs/tool-catalog.{json,md} are stale; run make gen-tool-catalog"; \
	       diff -u "$$tmpdir/tool-catalog.md.before" docs/tool-catalog.md | head -80; exit 1; }

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
	cp docs/live-tests.md internal/tools/selfinspect_assets/live-tests.md

selfinspect-drift:
	@diff -q docs/api-parity-matrix.md internal/tools/selfinspect_assets/api-parity-matrix.md >/dev/null \
	  || { echo "[selfinspect-drift] api parity asset is stale; run make sync-selfinspect-assets"; exit 1; }
	@diff -q docs/live-tests.md internal/tools/selfinspect_assets/live-tests.md >/dev/null \
	  || { echo "[selfinspect-drift] live tests asset is stale; run make sync-selfinspect-assets"; exit 1; }

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
	go test -tags=livee2e -count=1 -timeout 5m \
		-run '^(TestLiveOneUserWorkflowMCP|TestLiveRawClockifyReadSideSchemaDiff)$$' \
		./tests/...
	go test -count=1 -timeout 10m ./internal/tools -run '^TestOneUserLive'
	@if [ "$${CLOCKIFY_LIVE_OPTIONAL_DOMAINS:-}" = "1" ]; then \
		echo "== optional livee2e domain campaign =="; \
		go test -tags=livee2e -count=1 -timeout 10m \
			-run '^TestLive' \
			./tests/...; \
	else \
		echo "== optional livee2e domain campaign skipped (CLOCKIFY_LIVE_OPTIONAL_DOMAINS != 1) =="; \
	fi
