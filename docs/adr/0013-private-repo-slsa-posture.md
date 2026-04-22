# 0013 - Private-repo SLSA posture

## Status

**Superseded 2026-04-22 — the repository flipped to public.** The
skip path and `continue-on-error` treatments this ADR codified
are removed; SLSA attestation is now a mandatory gate on every
release and every main-branch image push. The ADR stays in the
tree as the historical record of why the private-repo workaround
existed from 2026-04-22 (SLSA workaround introduction via Wave G)
through 2026-04-22 (this flip) — roughly the v1.0.0–v1.0.3 era.
Future readers finding a reference to this ADR in a commit
message or PR body should understand: private-repo posture, no
longer live.

Prior status (pre-supersedure): Accepted — codifies the
release-smoke skip path added alongside this ADR and the
`continue-on-error` treatment already in `release.yml` and
`docker-image.yml`.

## Context

`go-clockify` ships three independent supply-chain verifications
on every release:

1. **SLSA build provenance** via
   `actions/attest-build-provenance`, verified by
   `gh attestation verify` at release-smoke time.
2. **cosign keyless signature on the binary** via goreleaser's
   `signs.cosign-keyless`, verified by `cosign verify-blob`.
3. **cosign keyless signature on the multi-arch container
   image** via `docker-image.yml`, verified by `cosign verify`.

The GitHub build-provenance attestation service is a feature
whose availability depends on the repository's **account tier**:

- Public repositories — available.
- Repositories owned by a GitHub organization — available.
- Repositories owned by a **user account and marked private** —
  **not available**. `gh attestation verify` returns
  `Feature not available for user-owned private repositories.`
  and the server responds with HTTP 404. No amount of retry,
  propagation wait, or `--owner` trust-root tweaking unblocks
  this — the attestation is never produced.

`go-clockify` is currently a user-owned private repository.
Commit [`6f4f748`](https://github.com/apet97/go-clockify/commit/6f4f748)
already flipped `docker-image.yml`'s attest-build-provenance step
to `continue-on-error: true` so the per-main-push build pipeline
does not fail. The release workflow (`release.yml`) carries the
same `continue-on-error` on its attestation step for the same
reason.

The gap this ADR closes: **release-smoke** still ran
`gh attestation verify` as fatal, so every scheduled or
release-dispatched smoke run failed, opened a
`release-smoke-failure` issue (#7), and never auto-closed. The
two mandatory cosign layers — which *do* work on user-owned
private repos, because they use keyless OIDC, not the
attestation service — were never reported on because smoke
exited before it got to them.

## Decision

SLSA build provenance is **optional supply-chain telemetry** for
this repository at its current account tier. The mandatory
gate is the two-layer cosign chain (binary + image).

Release-smoke now:

1. Runs `gh attestation verify` in a wrapper that captures its
   output.
2. On success — as today — the step passes.
3. On failure — the wrapper inspects the error:
   - If the error matches `Feature not available for
     user-owned private repositor` or `HTTP 404`, the step logs a
     `::notice::` titled "SLSA attestation skipped" and exits 0.
   - Every other failure mode (signature mismatch, missing
     bundle, wrong owner, expired cosign root, tampered
     binary) stays fatal.
4. The two cosign checks below remain mandatory — a bad cosign
   signature on either binary or image still fails the smoke
   and opens an issue.

The skip is **narrow and explicit**: it fires only on the two
specific error signatures the attestation service emits when
the feature gate is off. If GitHub later enables build
provenance for user-owned private repos, the step will succeed
and the skip branch becomes dead code — at which point the
cosign checks continue to gate unchanged.

## Consequences

**Positive.**
- Release-smoke can now reach a green state without requiring a
  repository-visibility or ownership change.
- Issue #7 closes automatically on the next green run instead of
  lingering as a permanent red flag that nobody actually needs
  to act on.
- The two cosign layers, which are the actually-binding
  cryptographic evidence at this account tier, are no longer
  masked by the attestation 404.

**Negative / residual.**
- `go-clockify` releases at this account tier carry two of the
  three canonical supply-chain attestations, not three. That is
  a true reduction in evidence compared to a
  public-or-org-owned repo. The README and
  `docs/production-readiness.md` should state this plainly to
  operators who require SLSA provenance specifically.
- If GitHub changes the exact text of the "feature not
  available" message, the grep will stop matching and the step
  will start failing again. The fix is a one-line grep update.
  Tracked implicitly: any regression will be caught on the next
  smoke run.

## Upgrade path (third option, not taken today)

Moving `go-clockify` to a public repository or to an
organization account activates the attestation service. At
that point:

1. `release.yml`'s attest-build-provenance step stops needing
   `continue-on-error: true`.
2. `docker-image.yml`'s attest-build-provenance step stops
   needing `continue-on-error: true`.
3. The skip branch added to `release-smoke.yml` becomes dead
   code but does not need removing — it simply never fires.

That flip is a **product/governance decision**, not a code
change, and is tracked as a separate backlog issue.

## References

- Commit [`6f4f748`](https://github.com/apet97/go-clockify/commit/6f4f748) —
  docker-image.yml continue-on-error.
- `.github/workflows/release-smoke.yml` — skip wrapper.
- `docs/verification.md` — operator-facing note on manual
  verification and the skip semantics.
