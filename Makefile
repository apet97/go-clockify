.PHONY: build test fmt vet check clean gen-tool-catalog catalog-drift gen-openapi openapi-drift gen-coverage-matrix coverage-matrix-drift live-contract-local

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

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

gen-coverage-matrix:
	python3 scripts/gen-coverage-matrix

coverage-matrix-drift:
	@tmpdir="$$(mktemp -d)"; \
	 trap 'rm -rf "$$tmpdir"' EXIT; \
	 cp docs/openapi/coverage-matrix.json "$$tmpdir/coverage-matrix.json.before"; \
	 cp docs/openapi/coverage-matrix.md "$$tmpdir/coverage-matrix.md.before"; \
	 $(MAKE) --no-print-directory gen-coverage-matrix >/dev/null; \
	 diff -q docs/openapi/coverage-matrix.json "$$tmpdir/coverage-matrix.json.before" >/dev/null \
	  && diff -q docs/openapi/coverage-matrix.md "$$tmpdir/coverage-matrix.md.before" >/dev/null \
	  || { echo "[coverage-matrix-drift] docs/openapi/coverage-matrix.{json,md} are stale; run make gen-coverage-matrix"; \
	       diff -u "$$tmpdir/coverage-matrix.md.before" docs/openapi/coverage-matrix.md | head -120; exit 1; }

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
