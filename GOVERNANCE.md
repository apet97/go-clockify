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
- [`docs/branch-protection-required-checks.md`](docs/branch-protection-required-checks.md)
  lists the required CI checks for `main`.

This document is the single source of truth for that decision; the
required-checks document is operational data for the current CI gate.

Operators evaluating whether to depend on `go-clockify`:
you can read this document, see who is on the hook, and decide
whether the audit trail that does exist (public CI logs and the
commit history on `main`) is sufficient for your risk appetite.

## Who can merge to `main`

`@apet97` is the only maintainer with merge access to `main`. Branch
protection on `main` is expected to enforce the merge gate via required
CI checks. The required check names are tracked in
[`docs/branch-protection-required-checks.md`](docs/branch-protection-required-checks.md).
Required approvals are currently set to 0 because GitHub does not let
PR authors approve their own pull requests, and this repository has one
maintainer.

`.github/CODEOWNERS` lists `@apet97` as the owner of every path;
this is a stylistic declaration today (one-of-one), kept because it
gives a future co-maintainer a clean diff target: adding a second
handle to the per-path entries is a one-line PR per path rather
than a ground-up rewrite.

## Merge gate

A PR may merge to `main` only if all of the following are true:

1. CI is green. Specifically, every required check listed in
   [`docs/branch-protection-required-checks.md`](docs/branch-protection-required-checks.md)
   reports success.
2. The branch is up-to-date with `main` (linear history is required;
   verify the live GitHub branch-protection settings before release).
3. The change does not lower a coverage floor without an explicit
   note in the PR body explaining why
   (see [`docs/coverage-policy.md`](docs/coverage-policy.md)).
4. Any required review comments and conversations are resolved before
   merge.

The merge gate is the same for self-authored PRs and external PRs. The
audit trail (public CI logs and the commit history on `main`) makes the
chain reviewable after the fact.

## Target state — not yet enforced

The following controls are target state, not current state:

- Required approvals: 1 non-author approval.
- Code-owner reviews: enabled.
- Signed commits: enabled.
- Admin enforcement: enabled.
- Restrict who can dismiss PR reviews: enabled.

These controls become enforceable when a second maintainer joins.
Until then, this document names the gap honestly so downstream consumers
can evaluate the trust model.

## Tighter self-review expectations on security-sensitive areas

Until a second maintainer joins, "dual review" on sensitive areas
is an aspiration rather than a mechanism. Today the expectation is
**self-review against the sensitive-area checklist**: the PR body
explicitly calls out which sensitive path is touched and how the
change was validated. The sensitive paths that trigger this
expectation are:

- `cmd/clockify-mcp/` — process entrypoint and doctor command.
- `internal/config/` — required one-user environment loading.
- `internal/mcp/` — MCP protocol core.
- `internal/tools/` — workflow, domain, resource, and raw fallback tools.
- `internal/clockify/` — HTTP client and auth headers.
- `tests/` — live MCP harnesses.

When a second maintainer joins, the sensitive-path list in
`.github/CODEOWNERS` will switch on required CODEOWNERS review; until
then, sensitive-path PRs are self-reviewed against this list and the
rationale is documented in the PR body via the checkbox in
`.github/PULL_REQUEST_TEMPLATE.md`.

## Releases

Releases are cut by `@apet97`: choose the version number, write the
`CHANGELOG.md` entry, push an annotated `vX.Y.Z` tag, and publish the
GitHub release. There is no automated release pipeline — the binary is
built from source with `go build` or `go install`.

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
goal; this document gets a mechanical rewrite to "two-maintainer" on
that event.

## Changes to this document

Changes to this document follow the normal merge gate. Operators
who depend on `go-clockify` and want to be notified of governance
changes should watch the repository for releases and read each
release's CHANGELOG entry. The rationale for each governance change
lives in the reviewing PR, the commit history on `main`, and the
release notes for shipped changes.
