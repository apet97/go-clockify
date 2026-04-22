# 0015 - Profile-centric configuration model

## Status

Accepted — implemented across four commits on `wave-i/profiles-and-doctor`:

1. `feat(config): canonical profiles with pinned defaults`
2. `feat(cmd): clockify-mcp --profile flag and doctor subcommand`
3. `docs: example env files and profile stubs for Wave I`
4. `docs(adr): 0015 profile-centric configuration model` (this ADR)

## Context

After Wave H the config surface stood at 53 env vars, grouped into
Core / Safety / Performance / Bootstrap / Transport / Auth /
Metrics / ControlPlane / Audit / Logging / Deploy. Wave H pinned
the production-strict defaults (streamable_http fail-closed on dev
DSN, legacy HTTP deny in prod, audit fail_closed in prod), but the
operator still had to hand-mix five or six env vars per deployment
shape to get to a working baseline.

Three documented "deployment shapes" existed as prose in
`docs/deploy/`:

- `profile-local-stdio.md` — single-user stdio subprocess.
- `profile-single-tenant-http.md` — one-process HTTP with a
  static bearer and a file-backed control plane.
- `production-profile-shared-service.md` — multi-tenant
  streamable HTTP with Postgres and OIDC.

Two more shapes were implicit — a private-network gRPC deployment
(required by our gRPC transport story) and a prod-Postgres variant
that turned on `ENVIRONMENT=prod` for the Wave H guards — with no
canonical doc or env file.

The gap: an operator reading a profile doc had to translate prose
into env-var decisions. A later audit of whether "that production
deployment we ran last week" actually matched the prose was by
hand. `clockify-mcp doctor` did not exist. Misconfigurations
surfaced at first request, not at `clockify-mcp --help`.

## Decision

Introduce a **profile-centric configuration model** with five
named profiles as a code-enforced source of truth. The operator
picks one with `clockify-mcp --profile=<name>` (or
`MCP_PROFILE=<name>` for container/systemd invocation) and every
other env var becomes optional-override rather than
required-input.

Profiles:

| Name | Shape | Key pins |
|------|-------|----------|
| `local-stdio` | single-user stdio subprocess | stdio transport, safe_core policy, no control plane, no auth |
| `single-tenant-http` | one-process HTTP + file store | streamable_http, static_bearer, file:// DSN, MCP_ALLOW_DEV_BACKEND=1, legacy HTTP deny |
| `shared-service` | multi-tenant streamable HTTP + Postgres | streamable_http, oidc, audit fail_closed, legacy HTTP deny — operator supplies the postgres DSN |
| `private-network-grpc` | gRPC + mTLS behind a private perimeter | grpc, mtls, audit fail_closed — requires `-tags=grpc` build |
| `prod-postgres` | shared-service with `ENVIRONMENT=prod` | as shared-service, plus ENVIRONMENT=prod triggers the Wave H prod guards |

### Apply semantics

`applyProfile(name)` runs once at the top of `config.Load()`. For
each key in the profile's Env map, it calls `os.Setenv(k, v)`
**only when the key is currently unset** (strict empty-string
check). This preserves the existing "explicit operator value
wins" invariant that every test in `prod_defaults_test.go` and
`transport_auth_matrix_test.go` already relied on.

Profiles do not duplicate or mutate any existing Load() logic —
they just pre-populate env values. Every downstream validation
(auth-mode matrix, legacy HTTP deny in prod, DSN fail-closed
guard, etc.) runs unchanged.

### Attribution

`clockify-mcp doctor` snapshots `os.Getenv` for every spec'd key
**before** Load() runs, then renders a grouped report that
attributes each row as:

- `explicit` — operator set it in the shell / container env.
- `profile` — applyProfile filled it in from the active profile.
- `default` — no env set, but the EnvSpec declares a default.
- `empty` — no env set, no documented default, behaviour is "off".

Exit code 0 = Load() succeeded, exit 2 = Load() error. The error
is printed verbatim as the second line of output.

## Consequences

### Positive

- Operator mental model collapses from "which of 53 env vars do I
  need?" to "which of 5 shapes am I running?". The 53 vars still
  exist for fine-tuning; profiles are opt-in sugar.
- `clockify-mcp doctor` turns "did we configure this right?" from
  a prose audit into a one-command answer — CI-friendly, exit
  code 2 on misconfiguration.
- The profile registry in `internal/config/profile.go` becomes the
  single source of truth cited by the five profile docs and five
  example env files. `TestProfile_KeysAreSpecced` enforces parity
  with `AllSpecs()`; `TestProfile_HelperCoversAllProfileKeys`
  enforces parity with the test helper.
- `prod-postgres` is a one-line alias on top of `shared-service`;
  adding a future `prod-mtls` is an equally small decision.
- `--profile` works for non-CLI invocation paths too: setting
  `MCP_PROFILE` in a Dockerfile or systemd unit has the identical
  effect.

### Negative

- One more thing for operators to learn. Mitigated by the README
  "Start Here" rewrite, a five-row table in the readme, and every
  profile doc pointing at the canonical flag and example file.
- `applyProfile` uses `os.Setenv` on unset keys, which is a
  process-level mutation. Tests that depend on env state must
  snapshot profile-controlled keys via `t.Setenv("")` before
  exercising `Load()`. The `setProfileEnv(t, name, overrides)`
  helper in `profile_test.go` encapsulates this and is enforced
  by `TestProfile_HelperCoversAllProfileKeys`.
- The "self-hosted" legacy shape does not get its own profile
  name — operators on that path choose between `local-stdio` and
  `single-tenant-http`. Rationale in
  `docs/deploy/profile-self-hosted.md`.

## Alternatives considered

### A. Refactor every `os.Getenv` in `Load()` to a `getenv` closure

Pass a `profileDefaults map[string]string` through the config
package and have every read fall back to it. Cleaner but touches
55 call sites plus the `optionalBoolEnv` / `optionalDurationEnv`
helpers. Rejected for blast radius; the os.Setenv-on-unset-keys
approach is a surgical 10-line addition and the test helper is a
localised fix.

### B. Use Viper / koanf for layered config

Adopt a real config-composition library with explicit layers
(profile < env < flag). Rejected for a stdlib-only-default-build
repo: adding a dependency for config when the entire
profile-apply logic is 20 lines violates ADR-0001. No other
justification for taking on a new runtime dependency.

### C. Profile-as-prose, no code enforcement

Keep the existing prose-only profile docs. Rejected because it
doesn't solve the audit problem: a doctor subcommand that doesn't
know the profile boundaries cannot attribute "this value came
from the shape you picked" vs. "this value is a raw default".

### D. One profile per binary

Ship `clockify-mcp-stdio`, `clockify-mcp-http`, etc. Rejected:
operators run the same binary under systemd and via
`go install`; a multi-binary distribution complicates every step
of the release pipeline (goreleaser matrix, sigstore bundles,
Docker tags) for no clear benefit.

## Follow-ups

Track in Wave L issues:

- Profile auto-detection from environment (e.g. presence of a
  postgres DSN + OIDC issuer → suggest `shared-service`).
- Doctor remote-audit mode: point doctor at a running server and
  audit the live Config instead of the local env.
- If a future deployment shape emerges that needs six-plus env
  vars, consider whether a profile definition grows to support
  "required keys with no default" as a schema instead of relying
  on the Wave H fail-closed guards to catch them.
