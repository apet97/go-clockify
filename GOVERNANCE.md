# Governance

This document describes how decisions get made in the `go-clockify`
project. Its job is to be honest about a small project's reality, not
to imitate the governance theatre of a large foundation.

## Project status

`go-clockify` is a **single-maintainer project today**. `@apet97` is
the sole maintainer: author of the majority of code, reviewer and
merger of every pull request, release signer, and security-disclosure
first responder. There is no second maintainer, no steering
committee, no technical advisory board, no rotating release captain.

This matches the reality reflected elsewhere in the repo:

- [`.github/CODEOWNERS`](.github/CODEOWNERS) lists `@apet97` as the
  sole owner for every directory.
- [`docs/production-readiness.md`](docs/production-readiness.md#governance)
  labels this a single-maintainer project with self-merge permitted.
- [`docs/branch-protection.md`](docs/branch-protection.md) documents
  why the "Restrict who can push to matching branches" rule is
  disabled (one user; no security benefit from an allow-list of one).

This document is the single source of truth for that decision; the
alternatives cited above reference back here.

Operators evaluating whether to depend on `go-clockify`:
you can read this document, see who is on the hook, and decide
whether the audit trail that does exist (public CI logs, GitHub
web-flow signed squash commits on `main` where available, SLSA
build provenance on every release since the 2026-04-22 public flip
(per ADR-0013, now Superseded — pre-flip releases were
best-effort), the release-smoke workflow) is sufficient for your
risk appetite.

## Who can merge to `main`

`@apet97` is the only maintainer with merge access to `main`. Branch
protection on `main` (snapshot in
[`docs/branch-protection.md`](docs/branch-protection.md)) enforces
the merge gate via required CI checks, linear history, up-to-date
branches, and conversation resolution. Required approvals are
currently set to 0 because GitHub does not let PR authors approve
their own pull requests, and this repository has one maintainer.

`.github/CODEOWNERS` lists `@apet97` as the owner of every path;
this is a stylistic declaration today (one-of-one), kept because it
gives a future co-maintainer a clean diff target: adding a second
handle to the per-path entries is a one-line PR per path rather
than a ground-up rewrite.

## Current state — enforced on `main`

The current branch-protection snapshot enforces:

- Required approvals: 0 enforced.
- Code-owner reviews: disabled.
- Signed commits: disabled.
- Admin enforcement: disabled.
- Required status checks: enabled.
- Branches up to date before merge: enabled.
- Conversation resolution: enabled.
- Linear history: enabled.

The effective merge gate at one-maintainer scale is therefore: CI
green, branch up to date, linear history, and conversation resolution.
The branch-protection snapshot remains the canonical record of the
applied GitHub settings.

## Merge gate

A PR may merge to `main` only if all of the following are true:

1. CI is green. Specifically, every required check listed in
   [`docs/branch-protection.md`](docs/branch-protection.md) reports
   success.
2. The branch is up-to-date with `main` (linear history is required;
   see branch-protection.md).
3. The change does not lower a coverage floor without an explicit
   note in the PR body explaining why
   (see [`docs/coverage-policy.md`](docs/coverage-policy.md)).
4. Any required review comments and conversations are resolved before
   merge.

The merge gate is the same for self-authored PRs and external PRs. The
audit trail (public CI logs, SLSA build provenance on releases where
available, release-smoke verification, and GitHub's signed web-flow
squash commits on `main`) makes the chain reviewable after the fact.

## Target state — not yet enforced

The following controls are target state, not current state:

- Required approvals: 1 non-author approval.
- Code-owner reviews: enabled.
- Signed commits: enabled.
- Admin enforcement: enabled.
- Restrict who can dismiss PR reviews: enabled.

These controls become enforceable when a second maintainer joins or
before a paid / public hosted service launch. Until then,
`docs/branch-protection.md` documents the gap honestly so downstream
consumers can evaluate the trust model.

## Tighter self-review expectations on security-sensitive areas

Until a second maintainer joins, "dual review" on sensitive areas
is an aspiration rather than a mechanism. Today the expectation is
**self-review against the sensitive-area checklist**: the PR body
explicitly calls out which sensitive path is touched and how the
change was validated. The sensitive paths that trigger this
expectation are:

- `internal/authn/` — authentication and JWT verification.
- `internal/enforcement/` — policy enforcement matrix.
- `internal/policy/` — policy definitions.
- `internal/transport/` — transport adapters (gRPC, streamable HTTP).
- `internal/clockify/` — HTTP client and auth headers.
- `.github/workflows/release.yml` — the release pipeline.
- `.github/workflows/docker-image.yml` — the image pipeline.
- `.goreleaser.yaml` — the release orchestrator.
- `deploy/` — operator-facing deploy manifests.

When a second maintainer joins, the sensitive-path list in
`.github/CODEOWNERS` will switch on required CODEOWNERS review; until
then, sensitive-path PRs are self-reviewed against this list and the
rationale is documented in the PR body via the checkbox in
`.github/PULL_REQUEST_TEMPLATE.md`.

## Releases

Releases are cut by `@apet97` via a tag push, which triggers
`release.yml` and `docker-image.yml`. The release pipeline is fully
automated; the maintainer's only manual step is choosing the version
number per `docs/release-policy.md` and writing the changelog entry.

Every release artifact is verified by `release-smoke.yml` on
publication and weekly thereafter (see `docs/verification.md`).

## Security disclosures

Security issues are reported privately via the GitHub Security
Advisory workflow at
<https://github.com/apet97/go-clockify/security/advisories/new>. Full
disclosure policy lives in [`SECURITY.md`](SECURITY.md), including
the response timeline (acknowledgment within 48 hours, fix within
1–2 weeks for high-severity).

There is no separate security team. The maintainer is the security
team. If `@apet97` is unreachable for an extended period, escalate
via a public GitHub issue tagged `unreachable-maintainer`.

## Becoming a maintainer

If you have been substantially contributing for several months and
want to take on review responsibility, open a discussion or issue
and the conversation will start. A second maintainer is an explicit
goal (tracked in Wave L's second-maintainer-onboarding issue); this
document gets a mechanical rewrite to "two-maintainer" on that
event.

## Changes to this document

Changes to this document follow the normal merge gate. Operators
who depend on `go-clockify` and want to be notified of governance
changes should watch the repository for releases and read each
release's CHANGELOG entry. The rationale for each change lives in
`docs/adr/` so future readers can trace why the policy is what it
is — see
[ADR-0016](docs/adr/0016-single-maintainer-governance.md) for the
single-maintainer decision.
