# Governance

This document describes how decisions get made in the `go-clockify`
project. Its job is to be honest about a small project's reality, not
to imitate the governance theatre of a large foundation.

## Project status

`go-clockify` is a **single-maintainer project** today. `@apet97`
authors the majority of code, reviews and merges all pull requests,
ships releases, and triages security disclosures. There is no
steering committee, no technical advisory board, no rotating release
captain. There is no fiction here either — operators can read this
document, see who is on the hook, and decide whether their risk
appetite allows depending on it.

## Who can merge to `main`

`@apet97` is the sole maintainer with merge access to `main`. Branch
protection on `main` (snapshot in
[`docs/branch-protection.md`](docs/branch-protection.md)) enforces
the merge gate via required CI checks; it does **not** enforce a
two-reviewer rule, because there is no second reviewer to enforce.

If a second maintainer joins the project, this section will be
updated to require their approval on PRs that touch the security-
sensitive surfaces listed below, and `.github/CODEOWNERS` will gain
their handle on the relevant lines.

## Merge gate

A PR may merge to `main` only if all of the following are true:

1. CI is green. Specifically, every required check listed in
   `docs/branch-protection.md` reports success.
2. The PR has been reviewed. For a self-authored PR by the sole
   maintainer, this means the maintainer has re-read the diff after
   CI lands. For PRs from external contributors, this means the
   maintainer has reviewed and approved.
3. The branch is up-to-date with `main` (linear history is preferred;
   merge commits are accepted only if rebasing would lose context
   from a long-lived feature branch).
4. Commits are signed (or, where signing is not yet enforced by
   branch protection, the maintainer has visually confirmed the
   author).
5. The change does not lower a coverage floor without an explicit
   note in the PR body explaining why
   (see `docs/coverage-policy.md`).

The merge gate is the same for self-authored PRs and external PRs.
Self-merge is permitted because there is no alternative reviewer
under single-maintainer governance, but the audit trail (signed
commits, SLSA build provenance on every release, public CI logs)
makes the chain reviewable after the fact.

## Tighter expectations on security-sensitive areas

For changes that touch any of the following directories, the
maintainer commits to **dual review when a second reviewer is
available**:

- `internal/authn/`
- `internal/enforcement/`
- `internal/policy/`
- `internal/transport/`
- `internal/clockify/`
- `.github/workflows/release.yml`
- `.github/workflows/docker-image.yml`
- `.goreleaser.yaml`
- `deploy/`

When no second reviewer exists, the maintainer documents the change
in the PR description with: (a) the threat being mitigated or the
behaviour being changed, (b) the test that exercises it, (c) the
rollback plan if it goes wrong. This is a self-imposed audit step,
not a hard merge gate; the goal is to make the reasoning visible
without blocking the project on the absence of a coreviewer.

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

There is no formal process today because there is no second
maintainer. If you have been substantially contributing for several
months and want to take on review responsibility, open a discussion
or issue and the conversation will start.

## Changes to this document

Changes to this document follow the normal merge gate. Operators who
depend on `go-clockify` and want to be notified of governance
changes should watch the repository for releases and read each
release's CHANGELOG entry.
