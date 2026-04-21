# Clockify MCP Review

## Overall Assessment

This is a mature, production-oriented MCP server, not a thin demo wrapper around the Clockify API. The repo shows strong architectural discipline, unusually deep safety controls, and clear operator thinking throughout the transport, auth, metrics, audit, and deployment layers.

The repository shape is a real strength. The protocol core, Clockify client, tool surface, and enforcement/runtime layers are separated cleanly enough that the system still reads coherently despite the overall size. That matters here because the project is already operating at a fairly large scope: 124 tools, 11 activatable Tier 2 groups, built-in resources, prompts, multiple transports, and multiple deployment profiles.

My main takeaway is that this repo already looks serious enough to ship and maintain, but it has a few high-value contract and documentation drifts that will confuse MCP clients and operators faster than the underlying code will fail.

## What's Strong

- The layered architecture is solid and easy to reason about in the key integration points: `internal/mcp/server.go`, `internal/runtime/runtime.go`, `internal/tools/registry.go`, and `internal/enforcement/enforcement.go`.
- The safety and ops posture is well above the bar for a typical MCP server. Policy modes, dry-run support, rate limiting, audit durability behavior, metrics exposure, transport/auth matrix handling, and operator deployment guides all show deliberate engineering rather than afterthought hardening.
- The project has strong quality discipline. I verified locally that `gofmt -l .`, `go vet ./...`, and `go test ./...` all passed.
- The repo also has meaningful test breadth for this category of project. At the time of review it contains 241 Go files and 129 `_test.go` files, which matches the impression of a heavily exercised codebase rather than a lightly validated integration.
- The deployment docs are notably stronger than average. In particular, `docs/deploy/profile-local-stdio.md` gives a clean fast-path for the common single-user case instead of forcing every reader through shared-service complexity first.

## Concrete Issues To Fix First

### 1. Activation naming drift is the highest-value UX/docs bug

The actual public activation surface is `clockify_search_tools` with `activate_group` / `activate_tool` parameters in `internal/tools/registry.go:176`.

But several agent-facing and generated surfaces still tell consumers to use nonexistent tools:

- `internal/mcp/server.go:43` references `clockify_activate_group` / `clockify_activate_tool`
- `scripts/gen-tool-catalog/main.go:135` generates the same incorrect names
- `docs/tool-catalog.md:7` publishes the same incorrect names

This is the most important issue to fix first because `ServerInstructions` directly shapes agent behavior during MCP initialization. If the initialization guidance is wrong, an otherwise capable client will systematically try the wrong commands.

### 2. npm package naming drift is an onboarding-breaker

The README currently tells users to install and configure the npm wrapper as `@anycli/clockify-mcp-go`:

- `README.md:37`
- `README.md:117`

But the actual package metadata points to `@apet97/clockify-mcp-go`:

- `npm/clockify-mcp-go/package.json:2`

That is not a cosmetic typo. It is a broken first-run path for users following the documented npm install flow, and MCP users usually experience this as "server will not start" rather than a clear packaging problem.

### 3. The dry-run contract described in docs/instructions does not match runtime behavior

Some user-facing text implies destructive operations default to preview when `dry_run` is omitted:

- `docs/clients.md:24`
- `internal/mcp/server.go:47`

But the implementation only enables the dry-run interceptor when `dry_run` is present and explicitly `true`:

- `internal/dryrun/dryrun.go:112`
- `internal/tools/entries.go:345`

That means the current runtime contract is narrower than the documented contract. The fastest fix is to align the docs and server instructions to the current implementation unless the intended product behavior is actually to change the runtime semantics. Right now the mismatch is more dangerous than either choice by itself.

## Strategic Improvements

- Split the top-level onboarding path more aggressively between "single-user stdio quick start" and "shared-service/operator deployment". The repo already has a good fast-path in `docs/deploy/profile-local-stdio.md:1`, but the main README still exposes a lot of operator surface area very early.
- Add more task-oriented workflows and examples. With 124 tools and Tier 2 activation, the biggest usability challenge is not capability but discovery. More examples around activation, safe-write flows, and common reporting/admin tasks would lower that cost substantially.
- Add a semantic docs-surface consistency check in CI for public names and contracts. Install package names, activation names, and safety-contract text should not be allowed to drift independently across README content, generated docs, and `ServerInstructions`.

## Validation Notes

- This review is based on static repository inspection plus local non-mutating verification.
- Checks run locally:
  - `gofmt -l .`
  - `go vet ./...`
  - `go test ./...`
- All three checks passed during review.
- I did not perform a live Clockify workspace/API exercise, so this feedback is grounded in repository truth and local verification rather than live-product behavior against a real account.

## Assumptions

- The deliverable requested here is a feedback document, not code fixes.
- The goal is to evaluate the MCP implementation as it exists in this repository today, using local evidence and local verification only.
