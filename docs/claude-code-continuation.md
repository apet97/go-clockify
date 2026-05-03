# Claude Code Continuation Packet

Prepared 2026-05-02 after PR #51 merged into `main`.

## Current State

- **Branch to start from:** `main`
- **PR #51 merge tip:** `adce316d60644fe51365086aba186227c9ae3977`
  (`docs(launch): record bench comparison evidence`)
- **Merged PR:** https://github.com/apet97/go-clockify/pull/51
- **Tracked tree expectation:** clean after `git pull --ff-only origin main`.
  This workstation may still show an untracked nested `go-clockify/`
  directory; it is local noise and not part of the tracked project state.

## Completed Local Gates

PR #51 completed the remaining local feature/readiness work that was
available without external launch evidence:

- Shared-service Postgres E2E is wired into CI and required on `main`.
- ADR 0017 Path A cross-instance streamable-HTTP rehydration is accepted
  and gated by `TestStreamableHTTPCrossInstanceRehydration`.
- Auth-model docs and `forward_auth` duplicate/oversized header guards
  are closed.
- Product launch docs, client matrix, support matrix, and deploy-profile
  verification sections are closed.
- Live-contract false-green prevention is in place through
  `make live-contract-local`, `TestLiveContractSkipSentinel`, and the
  launch evidence gate.
- Benchmark baseline was refreshed from Actions artifact
  `bench-current-25255062599`; normal comparison run `25255216987`
  passed against the refreshed linux/amd64 baseline.
- PR #51 checks were green before merge.

The last local verification pass before this continuation packet:

```sh
make check
make doc-parity
make config-doc-parity
make catalog-drift
make bench-baseline-check
git diff --check
```

## Remaining External Blockers Only

Do not claim official launch readiness until all three external evidence
items exist and are linked from the launch docs.

1. **Scheduled live-contract cron greens.**
   Two consecutive scheduled runs of `.github/workflows/live-contract.yml`
   on the candidate SHA must be green, including
   `TestLiveReadSideSchemaDiff`, mutating tests, and the audit-phase tier.
   Manual dispatches are useful diagnostics but do not close Group 1.

2. **Candidate-tag security walk-through.**
   Re-run `make verify-vuln`, `make verify-fips`, `make secret-scan`, and
   Semgrep on the final candidate tag. Record any findings in
   `SECURITY.md` and update `docs/launch-candidate-checklist.md` only with
   candidate-tag evidence.

3. **Release/sigstore/SLSA evidence.**
   Cut `vX.Y.Z-rc.N`, watch `release-smoke.yml`, verify signed binaries,
   SBOMs, Docker image signatures, sigstore bundles, and SLSA attestations,
   then archive the reference `doctor --strict` output.

## Start Commands

```sh
cd /Users/15x/Downloads/WORKING/addons-me/GOCLMCP
git switch main
git pull --ff-only origin main
git status --short --branch --untracked-files=no
git log -1 --oneline
```

Expected baseline:

```text
adce316 docs(launch): record bench comparison evidence
```

If this packet is read from a later local continuation commit, `git log
-1` may show that newer handoff commit. In that case, confirm that
`adce316...` is present immediately below it in history.

## Read First

1. `AGENTS.md`
2. `docs/agent-handoff.md`
3. `docs/launch-candidate-checklist.md`
4. `docs/official-clockify-mcp-gap-analysis.md`
5. `docs/live-tests.md`
6. `docs/release-policy.md`
7. This continuation packet

## Exact Next Prompts

Use one of these prompts in Claude Code depending on which external
evidence is now available.

### Live-Contract Evidence

```text
Audit the latest scheduled live-contract runs for go-clockify on main.
Start from docs/claude-code-continuation.md and AGENTS.md. Use GitHub
Actions evidence only: list recent scheduled live-contract.yml runs,
verify whether two consecutive cron runs on candidate commit
adce316d60644fe51365086aba186227c9ae3977 are green, confirm
TestLiveReadSideSchemaDiff, mutating, and audit-phase tests ran, and
check the live-test-failure issue state. If the evidence is complete,
update docs/launch-candidate-checklist.md, docs/agent-handoff.md, and
docs/official-clockify-mcp-gap-analysis.md with the exact run URLs. Do
not claim official launch readiness.
```

### Candidate-Tag Security

```text
Perform the candidate-tag security walk-through for go-clockify. Start
from main at adce316d60644fe51365086aba186227c9ae3977, create a small
branch, inspect AGENTS.md and docs/claude-code-continuation.md, then run
make verify-vuln, make verify-fips, make secret-scan, and Semgrep on the
final candidate tag. Record findings or "no findings" in SECURITY.md and
update docs/launch-candidate-checklist.md with exact evidence. Do not
weaken security defaults and do not claim official launch readiness.
```

### Release/Sigstore/SLSA Evidence

```text
Complete release-readiness evidence for the go-clockify candidate tag.
Start from docs/claude-code-continuation.md, docs/release-policy.md, and
docs/verification.md. Cut vX.Y.Z-rc.N only after live-contract and
candidate-tag security evidence exist, watch release-smoke.yml, verify
sigstore bundles, SLSA attestations, SBOMs, Docker image signature, and
reference doctor --strict outputs. Update the launch checklist and
handoff docs with URLs and commands. Do not claim official launch
readiness until every external evidence item is linked.
```

## Branch Rules

- Start from `main` at or after `adce316d60644fe51365086aba186227c9ae3977`.
- Use a short-lived branch for any change that will be pushed.
- One logical change per commit; commit bodies end with `Why:` and
  `Verified:` lines.
- Do not skip hooks, do not force-push shared branches, and do not push
  secrets or local workstation files.
- Do not edit generator-owned files by hand. If config or tool
  descriptors change, run `go run ./cmd/gen-config-docs -mode=all` and
  `make gen-tool-catalog` in the same commit.

## Verification Sequence

Run this before pushing any continuation-doc or launch-evidence update:

```sh
make check
make doc-parity
make config-doc-parity
make catalog-drift
make bench-baseline-check
git diff --check
```

For candidate promotion, add the external gates named above. Local green
is necessary but not sufficient.
