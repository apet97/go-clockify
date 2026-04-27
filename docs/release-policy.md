# Release policy

This document is the contract between `go-clockify` releases and the
operators who deploy them. It exists so a platform-team reviewer can
answer "what does this project promise about versions?" without asking
the maintainer.

## Supported versions

| Line  | Status     | Receives                                                 |
|-------|------------|----------------------------------------------------------|
| 1.2.x | Active     | Security fixes, bug fixes, features                      |
| 1.1.x | Superseded | Nothing — upgrade to `1.2.x`                             |
| 1.0.x | Patch-only | Correctness regressions on the stable v1 wire format only|
| 0.x   | EOL        | Nothing                                                  |

The Active line is the only one receiving non-correctness changes.
When the next minor ships (`1.3.0`), `1.2.x` will move to patch-only
and `1.1.x` will be EOL. See `SUPPORT.md` for the canonical state of
each line at any moment.

This is a single-supported-line policy by default. Platform teams that
need a longer support window should pin a specific patch on the
current minor and self-backport.

## Cadence

- **Patch** (`1.x.y` → `1.x.y+1`): on demand. Shipped when a fix lands
  on `main` and is worth releasing — typically within a week of merge.
- **Minor** (`1.x` → `1.(x+1)`): roughly every 6 weeks. Carries new
  tools, new transports, new auth modes, new env vars.
- **Major** (`1.x` → `2.0`): when a breaking change cannot be deferred.
  Announced one minor in advance.

## What counts as a breaking change

Any of the following requires a major version bump:

- Removing or renaming a tool exposed via `tools/list`.
- Changing a tool's annotation hints (`readOnlyHint`,
  `destructiveHint`, `idempotentHint`, `openWorldHint`, `title`) in a
  way that makes a previously-safe operation appear destructive (or
  vice versa).
- Removing or renaming an environment variable that operators
  configure (`CLOCKIFY_*`, `MCP_*`).
- Changing the semantics of a CLI flag (`--version`, `--help`, future
  flags).
- Bumping the MCP protocol version in a way that drops back-compat for
  a previously-supported version. The current back-compat window is
  documented in `README.md` under "Compatibility".
- Changing the wire format of a stable resource (`clockify://*`) or
  prompt template.

The following are **not** breaking changes:

- Adding a new tool, env var, transport, auth mode, prompt, or
  resource.
- Adding a new field to an existing tool's input or output (additive
  only — operators must tolerate unknown fields per JSON-RPC).
- Tightening a bug-fix in error wording (the error class stays stable;
  only the human-readable string changes).
- Changing internal package layout under `internal/`.

## Deprecation policy

When a breaking change is planned for the next major:

1. The minor release that introduces the deprecation includes the new
   surface alongside the old.
2. The old surface logs `level=WARN msg=deprecated_surface_used` to
   stderr the first time it is touched at process start.
3. The release notes for that minor list the deprecation explicitly
   under a "Deprecations" header.
4. The next major (one minor cycle later, ≥6 weeks) removes the old
   surface.

This gives operators a documented window to migrate without surprise.

## Backport criteria

Once a minor line is on the patch-only track (e.g. `1.0.x` after
`1.1.0` shipped, or `1.1.x` after `1.2.0` shipped), the following
backports are accepted:

- **Security fixes** (CVE-class) — always backported within the
  support window.
- **Data-loss bugs** — backported.
- **Crash bugs** triggered by inputs an operator cannot control —
  backported.
- **Performance regressions ≥2x** vs the prior patch — backported.
- **Everything else** — not backported. Upgrade to the current minor.

Backports do not introduce new env vars, new tools, or new behavior.
If a security fix requires a new configuration knob, the new minor
ships first and the patch backport defers the knob.

## Release artifacts

Every release produces 15 binaries across five tag combinations
(canonical list: `scripts/check-release-assets.sh`):

- **Default (stdlib only) — 5 binaries.** `clockify-mcp-darwin-arm64`,
  `-darwin-x64`, `-linux-arm64`, `-linux-x64`, `-windows-x64.exe`.
- **FIPS-tagged — 4 binaries.** `clockify-mcp-fips-darwin-arm64`,
  `-fips-darwin-x64`, `-fips-linux-arm64`, `-fips-linux-x64`. No
  Windows variant (Go FIPS toolchain support).
- **Postgres-tagged — 2 binaries.** `clockify-mcp-postgres-linux-arm64`,
  `-postgres-linux-x64`. Backs hosted shared-service deployments;
  required by `doctor --strict --check-backends`.
- **gRPC-tagged — 2 binaries.** `clockify-mcp-grpc-linux-arm64`,
  `-grpc-linux-x64`. The `private-network-grpc` profile.
- **gRPC + Postgres — 2 binaries.** `clockify-mcp-grpc-postgres-linux-arm64`,
  `-grpc-postgres-linux-x64`. Hosted gRPC shape.

All binaries are built with `-trimpath`. Every binary ships:

- A SPDX SBOM (`<binary>.spdx.json`).
- A keyless cosign signature (`<binary>.sigstore.json`).
- A SLSA build provenance attestation per binary, stored in the
  GitHub attestation service. SLSA was conditional while the repo
  was user-owned-private (per ADR-0013, now Superseded); mandatory
  on every release since 2026-04-22 when the repo flipped public.

Plus, per release tag:

- A multi-arch container image at
  `ghcr.io/apet97/go-clockify:v<version>` carrying a cosign signature,
  SPDX SBOM attestation, and SLSA build provenance attestation.
- An npm wrapper package (publish gated on `NPM_TOKEN` being set).
- A signed `SHA256SUMS.txt` covering every binary in the release.

Verification steps live in [docs/verification.md](verification.md). A
post-release smoke job (`.github/workflows/release-smoke.yml`)
re-verifies the SLSA attestation, sigstore bundle, and container
image signature on every published release and weekly thereafter, so
delayed drift surfaces as a `release-smoke-failure` GitHub issue
rather than silently rotting.

## How to check what version is supported

```sh
# What's the current supported line?
git -C /path/to/checkout tag --sort=-v:refname | head -1

# What's installed?
clockify-mcp --version
```

If the installed version's minor is older than the latest published
minor, you are unsupported.
