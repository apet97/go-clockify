## Description

Brief description of what this PR does. Focus on the **why** — the
what is in the diff.

## Merge gate

This PR follows [`GOVERNANCE.md`](../GOVERNANCE.md). Required CI
checks are documented in
[`docs/branch-protection.md`](../docs/branch-protection.md).

## Checklist

- [ ] `gofmt -w ./cmd ./internal ./tests` — no formatting issues
- [ ] `go vet ./...` — no warnings
- [ ] `go build ./...` — builds successfully
- [ ] `go test -race ./...` — all tests pass
- [ ] `make check` — green locally (required before push)
- [ ] Tests added for new functionality
- [ ] Documentation updated if needed

## Sensitive areas

This PR touches one or more of the paths listed in
[`GOVERNANCE.md`](../GOVERNANCE.md#tighter-self-review-expectations-on-security-sensitive-areas)
(`cmd/clockify-mcp/`, `internal/config/`, `internal/mcp/`,
`internal/tools/`, `internal/clockify/`, `tests/`):

- [ ] Yes — self-review rationale below.
- [ ] No — this box does not apply.

If **yes**, document the self-review here: which path, what
changed, how it was validated, and any drift-check or manual
exercise performed (example: "flipped the expected value in
`TestFoo_Bar`, confirmed the test fails, restored").
