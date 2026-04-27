# 0016 - Single-maintainer governance reality

## Status

Accepted — implemented on `wave-j/governance-source-of-truth`:

1. `docs(governance): single source of truth — align maintainer docs`

## Context

Before Wave J, four operator-facing docs described the maintainer
population inconsistently:

| Doc | Claim |
|-----|-------|
| `GOVERNANCE.md:9-15` | "two-maintainer project today" + `@backup-maintainer` |
| `.github/CODEOWNERS:15` | `* @apet97 @backup-maintainer` and per-path entries |
| `docs/production-readiness.md:203` | "Single-maintainer project today (`@apet97`)" |
| `docs/branch-protection.md:42` | "this is a single-maintainer repository (see GOVERNANCE.md)" — but GOVERNANCE.md said otherwise |

`@backup-maintainer` was a placeholder handle from an earlier draft,
not a real GitHub user. Merging a PR through CODEOWNERS would
therefore fall back to the `*` rule regardless of what the per-path
entries said — the whole scaffolding resolved to `@apet97`, with the
`@backup-maintainer` text serving as documentation aspiration rather
than enforcement.

The contradiction was noticed during a governance audit on
2026-04-22. Operators reading GOVERNANCE.md were getting a different
story than operators reading production-readiness.md, and the
mismatch was visible from the first line of either doc. An external
reviewer would reasonably conclude one of three things:

1. Docs are stale — which means other operator-facing docs might be
   too.
2. The project actually has a hidden second maintainer — which
   means the audit trail is thinner than advertised.
3. The project is single-maintainer with aspirational docs — which
   is fine, but should be labelled honestly.

Reality is #3. Wave J makes #3 the written truth.

## Decision

Align all governance-relevant docs on "single-maintainer project
today":

- `GOVERNANCE.md` is rewritten to open with "single-maintainer
  project today," soften the dual-review section to "tighter
  self-review expectations," and point at ADR 0016 for the
  rationale. Self-merge is explicitly permitted (already the
  reality, now documented).
- `.github/CODEOWNERS` drops `@backup-maintainer` from every entry.
  The per-path entries stay — a future co-maintainer addition is a
  one-line PR per path, which is a clean diff target.
- `SUPPORT.md` is added at repo root (new file) with the "single
  maintainer, best-effort, no SLA" expectation, the security
  timeline pointer, and the `v1.x` stability guarantee.
- `.github/PULL_REQUEST_TEMPLATE.md` gets a sensitive-area
  checkbox so PRs touching security-sensitive paths document the
  self-review against the checklist in GOVERNANCE.md.
- `docs/production-readiness.md` is unchanged — it was already
  consistent with the new target.
- `docs/branch-protection.md` is unchanged — it already
  documented single-maintainer reality in its footnotes.

Dual-review on sensitive areas becomes an aspiration for when a
second maintainer joins (tracked in Wave L's
"second-maintainer-onboarding" issue). Until then, sensitive-area
PRs get an explicit self-review callout in the PR body, with the
checklist items that apply documented there.

## Consequences

### Positive

- One consistent story across every governance-relevant doc.
  Operators land on a clear trust statement regardless of which
  doc they open first.
- Removing `@backup-maintainer` stops the CODEOWNERS UI from
  silently falling back to `*`. The per-path entries now truthfully
  describe the owner.
- `SUPPORT.md` fills a visible gap — operators evaluating
  dependency risk had to piece together SLA expectations from
  SECURITY.md + README + GOVERNANCE.md. Now one page.
- The sensitive-area checkbox in the PR template turns "tighter
  self-review" from a vibes-based commitment into a visible,
  document-in-the-PR-body commitment.

### Negative

- Dual review is explicitly aspirational rather than a mechanism.
  This is the honest state, but some operators prefer a written
  commitment even when it is one-of-one today.
- A future co-maintainer event requires synchronised edits to
  GOVERNANCE.md, CODEOWNERS, and (optionally) the PR template.
  Mitigated by pointing every doc at this ADR so the checklist is
  in one place.

## Alternatives considered

### A. Keep the two-maintainer fiction and hope nobody notices

Rejected. Operators doing a dependency audit before taking a
production dependency will notice, and the mismatch would suggest
a broader hygiene problem.

### B. Invent a real second maintainer

Rejected. A maintainer is a real commitment; inflating the count
by inventing one is worse than being honest about one.

### C. Remove CODEOWNERS entirely

Rejected. Removing CODEOWNERS loses the per-path review signal
that GitHub surfaces on every PR. Keeping the scaffolding (with
one handle per line) preserves that signal and gives a clean
future-diff target.

### D. Automate governance-doc parity via a CI gate

Considered. A grep-based gate could check that every
governance-relevant doc says "single-maintainer" consistently.
Rejected for Wave J because the surface is small (four docs)
and because doc-parity CI gates tend to be noisy. The doc
structure (pointing everything back to GOVERNANCE.md and ADR
0016) is lighter-weight enforcement. Revisit if we acquire the
second maintainer or drift reappears.

## Follow-ups

Tracked in Wave L issues:

- **Second-maintainer onboarding.** Once a candidate appears,
  GOVERNANCE.md gets a mechanical "single-maintainer" →
  "two-maintainer" edit, CODEOWNERS adds the handle to each
  per-path entry, and the sensitive-area checkbox in the PR
  template switches from self-review to dual-review.
- **Branch-protection hardening once a second maintainer joins.**
  Flip `enforce_admins` on, consider pushing required approvals
  from 1 to 2 for sensitive paths via CODEOWNERS enforcement.
- **Auto-generate branch-protection.md.** Was blocked pre-2026-04-22
  by the GitHub API returning 403 on user-owned private repos; that
  blocker is gone since the public flip (per ADR-0013, now
  Superseded), and `scripts/audit-branch-protection.sh` already
  reads the live protection state via `gh api`. Remaining work is
  the lightweight glue that turns that JSON into the snapshot
  table at `docs/branch-protection.md` (line-by-line render +
  CI gate that fails when the live state diverges from the
  rendered snapshot). Tracked in Wave L follow-ups.

## References

- `GOVERNANCE.md` — the maintainer-count and merge-gate rules.
- `.github/CODEOWNERS` — per-path ownership.
- `SUPPORT.md` — adoption-time expectations.
- `docs/production-readiness.md` — already consistent with the
  new GOVERNANCE.md.
- `docs/branch-protection.md` — already consistent with the new
  GOVERNANCE.md; references this ADR in the footnotes.
