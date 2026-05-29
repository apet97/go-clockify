# Fuzz corpus

Six fuzz targets back this repo's fuzz-short and weekly fuzz workflows.
Three carry a committed regression corpus that Go replays on every
`go test` run; three rely on inline `f.Add(...)` seeds.

| Target | Source | Persistent corpus |
|---|---|---|
| `FuzzParseDatetime` | `internal/timeparse/timeparse_test.go` | `internal/timeparse/testdata/fuzz/FuzzParseDatetime/` |
| `FuzzValidateID` | `internal/resolve/resolve_test.go` | `internal/resolve/testdata/fuzz/FuzzValidateID/` |
| `FuzzJSONRPCParse` | `internal/mcp/server_test.go` | `internal/mcp/testdata/fuzz/FuzzJSONRPCParse/` |
| `FuzzSafeRawPath` | `internal/tools/raw_path_fuzz_test.go` | inline seeds only |
| `FuzzTightenInputSchema` | `internal/tools/schema_tighten_fuzz_test.go` | inline seeds only |
| `FuzzTranslateAPIError` | `internal/clockify/errors_fuzz_test.go` | inline seeds only |

The persistent corpus lives under `<package>/testdata/fuzz/<target>/`.
Every entry is a file whose contents match Go's standard fuzz format:

```
go test fuzz v1
string("...")
```

## How the corpus is used

**On every `go test` run** (no `-fuzz` flag), Go replays the entire
committed corpus as regression inputs against the matching fuzz target.
A test is registered per corpus file, named
`FuzzX/<hex-digest-filename>`. The corpus doubles as a regression suite —
if any committed input starts failing the fuzz invariant, CI breaks at
that single entry.

**On `go test -fuzz=<target>`**, Go uses the committed corpus as seeds
for new mutation and writes any newly-interesting inputs to
`$GOCACHE/fuzz/.../` (NOT to `testdata/fuzz/`). Committed inputs stay in
the repo until someone explicitly commits more, deletes an entry, or
prunes.

The three targets without a `testdata/fuzz/` directory still benefit
from their inline `f.Add(...)` seeds — but only under `-fuzz`. Bootstrap
a persistent corpus for them when an interesting failure shows up.

## How to grow a corpus

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
package's `testdata/fuzz/X/` directory automatically and fails the test
with the path to the file. Commit the file immediately — it's now a
crash regression and the fix can be validated by re-running the replay
(not `-fuzz`):

```bash
go test -count=1 -run 'FuzzX/<filename>' ./internal/<pkg>
```

Do NOT add the input to `f.Add(...)` instead of committing the testdata
file — the testdata path runs on every CI invocation; an `f.Add(...)`
call only runs under `-fuzz`, defeating the regression gate.

## When to prune

Rarely. The corpus is small. Prune only when:

- A fuzz target is renamed and the old `testdata/fuzz/<old-name>/`
  directory is unreferenced (delete with `git rm -r`).
- A fuzz target is deleted (same).
- A committed entry is subsumed by code that no longer distinguishes
  it from a simpler seed — and even then, leave it unless it's
  actively misleading the fuzzer.

## Related

- CI fuzz workflow: `.github/workflows/fuzz.yml` runs each of the six
  targets weekly with `-fuzztime=30s` against the committed corpus and
  inline seeds.
- Go fuzzing docs: <https://go.dev/security/fuzz/>
