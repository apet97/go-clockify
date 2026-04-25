# Support

`go-clockify` is a single-maintainer project today
(see [GOVERNANCE.md](GOVERNANCE.md)). This document covers how to
ask questions, what to expect when you do, and how to avoid
surprises when taking a dependency on this project.

## Where to ask

| Kind of question | Where |
|------------------|-------|
| Bug report or feature request | [GitHub Issues](https://github.com/apet97/go-clockify/issues) |
| Question about usage, configuration, or operations | [GitHub Issues](https://github.com/apet97/go-clockify/issues) with the `question` label |
| Private security vulnerability | [GitHub Security Advisory](https://github.com/apet97/go-clockify/security/advisories/new) — see [SECURITY.md](SECURITY.md) for full disclosure policy |
| "Is this change safe for my environment?" | Open an issue; pre-upgrade questions are explicitly welcome |

GitHub Discussions is not enabled today. If the signal-to-noise
ratio on issues becomes a problem, enabling it is one of the
Wave L follow-ups.

## Response expectations

There is no SLA. A single maintainer working on a best-effort
basis means questions and bugs land in the same backlog as
feature work.

Rough expectations, based on the last six months of activity:

- Security disclosures acknowledged within 48 hours. Fix targeted
  within 1–2 weeks for high-severity issues. This is the only
  commitment in this document — security response is held to a
  published timeline in [SECURITY.md](SECURITY.md).
- Questions on routine usage: usually a same-day reply, sometimes
  a few days.
- Bugs that affect correctness on the stable `v1.x` wire format:
  prioritised above feature work, targeted in the next point
  release.
- Bugs that affect secondary surfaces (legacy HTTP transport,
  `-tags=fips`, `-tags=otel`): fixed as time allows.
- Feature requests: triaged into the backlog; decisions on
  whether to accept are posted on the issue.

If a maintainer response is time-critical (incident triage,
security disclosure follow-up), escalate by pinging the issue.
GitHub notifications are actually read.

## Commercial support

None today. There is no commercial entity behind `go-clockify` —
it is a personal open-source project released under the MIT
license. Operators who need dedicated support are welcome to
contract the maintainer directly, but no such arrangement exists
as a standing offer.

## Stability guarantees

`go-clockify` follows [Semantic Versioning](https://semver.org/).
`v1.0.0` shipped on 2026-04-12 and declared the wire format
stable.

What we promise for the `v1.x` series:

- **Wire format stability.** MCP method names, tool names, and
  env-var surface stay backward-compatible within `v1.x`. Any
  change that would break a wire-compatible client bumps the
  major.
- **Env-var renames are breaking.** Env-var renames (not just
  additions) only land in a major bump; today we use
  `Deprecated: true` spec entries as a soft migration window
  where we intentionally accept both.
- **Signed releases.** Every tagged release ships with cosign
  signatures, SBOM, and (where available) SLSA build provenance.
  Verification recipe in [`docs/verification.md`](docs/verification.md).
- **No surprise removals.** Deprecations are announced one minor
  version before removal. The `MCP_HTTP_MAX_BODY` alias is the
  canonical example: it will disappear no earlier than a
  `v2` bump, and only if we also make other breaking changes
  worth a major.

What we do NOT promise:

- A specific minor-release cadence. Minors ship when changes
  warrant.
- Long-term support for every minor. The latest minor is the
  supported minor.
- Backports of fixes to older minors. If you are on `v1.0.3` and
  need the `v1.1.2` fix, upgrade to `v1.1.2`.

## Upgrading

Before upgrading, read the `CHANGELOG.md` Unreleased section and
the target release's entry. Default changes (like Wave H's prod
fail-closed defaults or Wave I's profile-based config) are called
out in the Changed / Added sections. `clockify-mcp doctor` will
tell you which of your env vars come from profile defaults vs.
explicit operator intent — run it before and after an upgrade.

## Version support matrix

| Version | Status | Notes |
|---------|--------|-------|
| `v1.1.x` | Active — released 2026-04-22; where new features land | |
| `v1.0.x` | Released 2026-04-12 through 2026-04-20; latest is `v1.0.3` | Patch fixes land here only for correctness regressions that affect the stable wire format |
| `v0.x` | Pre-release; not supported | Pre-`v1` releases from March–April 2026; do not deploy |

## Discussion etiquette

- Include the output of `clockify-mcp doctor` (or your effective
  env) when reporting a bug. It is the single fastest way to
  reproduce what you are seeing.
- If you are asking about operator decisions ("should I use
  `shared-service` or `prod-postgres`?"), skim
  [`docs/deploy/`](docs/deploy/) first — the profile docs answer
  most common questions inline.
- Minimal reproductions are load-bearing. "It broke" is hard to
  action; "I ran `X`, expected `Y`, saw `Z`" is easy to action.

## Thank you

Issues with attached reproduction steps, PRs with tests, and
reviewers who push back on unclear rationale — these are the
things that make an open-source project sustainable for one
maintainer. If you are doing any of those things, thank you.
