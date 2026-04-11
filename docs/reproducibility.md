# Reproducible builds

Every binary published with a `v*.*.*` release tag can be rebuilt
byte-for-byte from the git source on a clean Linux runner. This document
explains the guarantee, the exact environment required, and how to
verify a release yourself without waiting for CI.

## Guarantee

For every release tag `vX.Y.Z` on `main`, and for every binary asset
attached to that release (9 assets per release: 5 default builds + 4
FIPS builds), the following statement is true:

> On a Linux/amd64 runner with Go `go.mod`'s pinned version, a clean
> source checkout at `vX.Y.Z`, and the exact `-trimpath` / `-ldflags`
> combination the release used, the rebuilt binary is **byte-identical**
> to the corresponding asset published to the GitHub release.

CI enforces this automatically: every `release: published` event fires
`.github/workflows/reproducibility.yml`, which rebuilds all 9 assets in
parallel and compares sha256. Any drift fails the workflow and produces
a diff of `go version -m` output to help pin the cause.

You can also dispatch the workflow manually against any existing tag
via "Actions → Reproducibility → Run workflow → tag: vX.Y.Z".

## Why it works

Four things combine to make the builds deterministic:

1. **Pinned Go toolchain**. `go.mod` records an exact Go version (e.g.
   `go 1.25.9`); `.github/workflows/reproducibility.yml` installs the
   same version via `setup-go` with `go-version-file: go.mod`.
2. **`-trimpath`**. Strips GOROOT, GOPATH, and module-cache paths from
   the resulting binary, eliminating hostname, temp-dir, and
   home-directory variance.
3. **`CGO_ENABLED=0`**. Every release build uses pure Go (no cgo), so
   there is no linker, no libc, and no platform-SDK variance to
   reproduce.
4. **Commit-anchored ldflags**. `main.version`, `main.commit`, and
   `main.buildDate` are injected at build time from `{Tag, FullCommit,
   CommitDate}` of the tagged commit. All three are fixed for a given
   tag, so every rebuild embeds the same values.

## The goreleaser dirty-tree quirk

There is one non-obvious trick the workflow uses. goreleaser creates
`dist/` and writes its own intermediate files before invoking
`go build`, so Go's VCS stamping sees the working tree as dirty and
embeds `mod=vX.Y.Z+dirty` / `vcs.modified=true` in the binary. A clean
`git clone` + `go build` would embed `mod=vX.Y.Z` / `vcs.modified=false`
instead, producing a different binary even with everything else
identical.

The reproducibility workflow induces the same dirty state by writing
`dist/.reproducibility-placeholder` before running `go build`. This is
the only non-intuitive step; everything else is just matching
`-trimpath`, `-ldflags`, `CGO_ENABLED`, and the platform env.

## Local verification

To reproduce a release asset yourself without waiting for CI, run:

```bash
# Clone the tag (full history, for git show -s --format=%ct)
git clone --branch v0.7.1 https://github.com/apet97/go-clockify /tmp/repro
cd /tmp/repro

# Install the exact Go version recorded in go.mod
go install golang.org/dl/go1.25.9@latest
~/go/bin/go1.25.9 download

# Extract ldflags inputs
SHA=$(git rev-parse HEAD)
TS=$(git show -s --format=%ct HEAD)
# Linux:
BUILD_DATE=$(date -u -d "@${TS}" +%Y-%m-%dT%H:%M:%SZ)
# macOS: BUILD_DATE=$(date -u -r "${TS}" +%Y-%m-%dT%H:%M:%SZ)

# Induce the goreleaser dirty-tree state
mkdir -p dist
echo "repro" > dist/.reproducibility-placeholder

# Rebuild linux-x64 (swap GOOS/GOARCH for other platforms)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  ~/go/bin/go1.25.9 build -trimpath \
  -ldflags "-s -w -X main.version=v0.7.1 -X main.commit=${SHA} -X main.buildDate=${BUILD_DATE}" \
  -o /tmp/clockify-mcp-linux-x64.local ./cmd/clockify-mcp

# Download the release asset and compare
gh release download v0.7.1 --repo apet97/go-clockify -p clockify-mcp-linux-x64 -D /tmp/released
sha256sum /tmp/clockify-mcp-linux-x64.local /tmp/released/clockify-mcp-linux-x64
# Both lines should print the same sha256 value.
```

For FIPS binaries, add `GOFIPS140=latest` and `-tags=fips` to the
`go build` invocation.

## Matrix of supported platforms

| Asset | GOOS | GOARCH | Build tags | Reproducible |
|---|---|---|---|---|
| `clockify-mcp-linux-x64` | linux | amd64 | — | yes |
| `clockify-mcp-linux-arm64` | linux | arm64 | — | yes |
| `clockify-mcp-darwin-x64` | darwin | amd64 | — | yes |
| `clockify-mcp-darwin-arm64` | darwin | arm64 | — | yes |
| `clockify-mcp-windows-x64.exe` | windows | amd64 | — | yes |
| `clockify-mcp-fips-linux-x64` | linux | amd64 | `fips` + `GOFIPS140=latest` | yes |
| `clockify-mcp-fips-linux-arm64` | linux | arm64 | `fips` + `GOFIPS140=latest` | yes |
| `clockify-mcp-fips-darwin-x64` | darwin | amd64 | `fips` + `GOFIPS140=latest` | yes |
| `clockify-mcp-fips-darwin-arm64` | darwin | arm64 | `fips` + `GOFIPS140=latest` | yes |

The reproducibility workflow cross-compiles every target from a single
`ubuntu-22.04` runner, so you only need a Linux environment (or any
cross-compile-capable Go toolchain) to verify every asset locally.

## Troubleshooting

**"sha256 mismatch for `<asset>`"**: the CI workflow prints a diff-ready
investigation snippet:

```bash
diff <(go version -m /tmp/<asset>.local) <(go version -m /tmp/released/<asset>)
cmp /tmp/<asset>.local /tmp/released/<asset>
```

Look for differences in `vcs.modified`, `vcs.revision`, `mod`, or `go
1.x.y`. The most common causes are:

1. **Go version drift** (`go 1.25.9` vs `go 1.25.10`) — the release
   toolchain was bumped after the tag was cut. Fix: pin the tag to a
   specific Go minor in the reproducibility workflow, or re-release
   with an updated `go.mod`.
2. **New goreleaser feature** that dirties additional files in `dist/`
   or writes new build metadata. Fix: extend the "Induce goreleaser
   dirty-tree state" step with the missing files.
3. **`SOURCE_DATE_EPOCH` drift** — we do not currently rely on this
   variable (Go embeds commit time directly), so it should never be an
   issue, but worth checking if a future goreleaser release adds
   support.

**"No such file or directory"** on `dist/.reproducibility-placeholder`:
some other step cleaned `dist/` between the induce-dirty step and the
build. Move the placeholder write immediately before `go build`.

## Relationship to other verification steps

| Verifies | Tool |
|---|---|
| **Byte-identical rebuild** (this doc) | reproducibility workflow |
| **Signer identity + blob integrity** | cosign + `docs/verification.md` |
| **Build provenance (who/where/when)** | SLSA attestation + `docs/verification.md` |
| **Package content** | SPDX SBOM + `docs/verification.md` |

Use reproducibility verification alongside the cosign signature check
for a complete supply-chain story: cosign proves who signed the binary
and that the bytes haven't been tampered with, and the reproducibility
workflow proves the bytes match what the source code + build contract
actually produce.
