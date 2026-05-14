# Coverage Policy

## Rule

No regressions, only ratchets. Coverage floors live in
`scripts/check-coverage.sh`; that script is the source of truth.

Raising a floor is a normal maintenance change. Lowering a floor requires a
clear reason in the PR description, such as deleted dead code or a package move
that changes the denominator.

## Running Locally

```sh
make cover-check
```

For a faster read while iterating on one package:

```sh
go test -cover ./internal/tools
go test -cover ./internal/mcp
```

## One-User Quality Gates

The one-user product has extra tests beyond raw line coverage:

- Golden startup shape for `initialize`, `tools/list`, `prompts/list`, and
  resources.
- Runtime-order checks for workflow/domain/raw fallback tools.
- Output-schema checks for workflow tools.
- Fake-server smoke for workflow and domain envelopes.
- Startup/listing tests that assert no Clockify API calls happen before a
  tool or network-backed resource is read.

When changing workflow tools, add or update behavior tests before changing the
implementation. When changing descriptors, regenerate the tool catalog.

## Generated Artifacts

Do not count generated catalog diffs as coverage. They are parity artifacts.
After descriptor changes, run:

```sh
make gen-tool-catalog
git diff -- docs/tool-catalog.md docs/tool-catalog.json
```
