# Fuzz corpus

This repository commits a persistent fuzz corpus for three targets:

| Target | Package | Entries (at Wave D bootstrap) |
|---|---|---|
| `FuzzParseDatetime` | `internal/timeparse` | 327 |
| `FuzzValidateID` | `internal/resolve` | 85 |
| `FuzzJSONRPCParse` | `internal/mcp` | 384 |

Corpus files live under `<package>/testdata/fuzz/<target>/`. Every
entry is a file whose contents match Go's standard fuzz format:

```
go test fuzz v1
string("...")
```

## How the corpus is used

**On every `go test` run** (no `-fuzz` flag), Go replays the entire
committed corpus as regression inputs against the fuzz target. A
test is registered per corpus file, named
`FuzzX/<hex-digest-filename>`. This means the corpus doubles as a
regression suite — if any committed input starts failing the fuzz
invariant, CI breaks at that single entry.

**On `go test -fuzz=<target>`**, Go uses the committed corpus as
seeds for new mutation, and writes any newly-interesting inputs to
`$GOCACHE/fuzz/.../` (NOT to `testdata/fuzz/`). Committed inputs
stay in the repo until someone explicitly commits more, deletes an
entry, or prunes.

## How the corpus was bootstrapped (Wave D)

Before Wave D, all three fuzz targets had only the hand-written
`f.Add(...)` seeds at the top of each fuzz function. `testdata/fuzz/`
did not exist anywhere in the repo, so a bare `go test ./...`
exercised only what the test author wrote — a narrow surface.

The Wave D D5 commit ran each target with
`go test -fuzz=<name> -fuzztime=60s` to populate
`$GOCACHE/fuzz/github.com/apet97/go-clockify/<package>/<target>/`
with auto-discovered interesting inputs, then copied every entry
from the cache to `<package>/testdata/fuzz/<target>/` so Go's test
runner will replay them on every CI invocation.

## How to grow the corpus

When you fix a fuzz crash, or when you want to give the fuzzer more
seed variety before a long fuzz run:

```bash
# Pick one target and run for a meaningful time.
go test -fuzz=FuzzParseDatetime -fuzztime=5m -run='^$' ./internal/timeparse

# Copy any new interesting inputs into testdata/fuzz/.
FUZZROOT="$(go env GOCACHE)/fuzz/github.com/apet97/go-clockify"
cp "$FUZZROOT/internal/timeparse/FuzzParseDatetime/"* \
   internal/timeparse/testdata/fuzz/FuzzParseDatetime/

# Verify the regression replay still passes.
go test -count=1 -run FuzzParseDatetime ./internal/timeparse

# Commit.
git add internal/timeparse/testdata/fuzz/
git commit -m "test(timeparse): grow fuzz corpus with <N> new inputs"
```

`cp -n` (no-clobber) is optional — duplicate filenames mean the cache
and the committed corpus already agreed on an entry, so `cp` would
write the same bytes on top of themselves.

## When a fuzz crash fires

If `go test -fuzz=X` finds a crash, Go writes a reproducer into the
package's `testdata/fuzz/X/` directory automatically and fails the
test with the path to the file. Commit the file immediately — it's
now a crash regression and the fix can be validated by re-running
the replay (not `-fuzz`):

```bash
go test -count=1 -run 'FuzzX/<filename>' ./internal/<pkg>
```

Do NOT add the input to `f.Add(...)` instead of committing the
testdata file — the testdata path runs on every CI invocation; an
`f.Add(...)` call only runs under `-fuzz`, defeating the regression
gate.

## When to prune

Rarely. The corpus is small (~3 MB total). Prune only when:

- A fuzz target is renamed and the old `testdata/fuzz/<old-name>/`
  directory is unreferenced (delete with `git rm -r`).
- A fuzz target is deleted (same).
- A committed entry is subsumed by code that no longer distinguishes
  it from a simpler seed — and even then, leave it unless it's
  actively misleading the fuzzer.

## Related

- Fuzz functions: `internal/timeparse/timeparse_test.go:FuzzParseDatetime`,
  `internal/resolve/resolve_test.go:FuzzValidateID`,
  `internal/mcp/server_test.go:FuzzJSONRPCParse`
- CI fuzz-short gate: `Makefile:fuzz-short` target runs each target
  for 10 s on every PR (not a corpus bootstrap — that's what this doc
  is for; the PR gate is a smoke check).
- Go fuzzing docs: https://go.dev/security/fuzz/
