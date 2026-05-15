# Brand and Legal Review Questions

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


Status: **NEEDS LEGAL REVIEW**. This document frames questions for
CAKE.com / Clockify product, legal, or brand reviewers. It is not legal
advice and does not approve any trademark, product, or partnership
claim.

## Review Trigger

The May 8 launch-readiness review flagged `L-10`: promotion language
around Clockify requires written approval or a rebrand decision before
public launch messaging can claim official-product status.

Until this review is closed:

- Treat `clockify-mcp`, `go-clockify`, `@apet97/clockify-mcp-go`,
  `clockify_*` tool names, `clockify://` resource URIs, and the
  opt-in gRPC service name `clockify.mcp.v1.MCP` as descriptive
  technical identifiers only.
- Do not describe the project as approved, endorsed, partnered,
  product-owned, or official.
- Keep public launch language framed as a launch-candidate evidence or
  review pass.

## Owner / Reviewer Role

This is a CAKE.com / Clockify-side approval gate. The repository
maintainer cannot self-approve it.

- **Owner role: CAKE.com / Clockify legal, brand, or product
  reviewer.** A specific reviewer with written authority to grant or
  refuse trademark and official-product framing must accept the
  request. No reviewer is currently assigned; the maintainer must
  record the assigned reviewer's identity in this section before any
  decision iteration is treated as approval. The repository
  maintainer (`@apet97`) drives the request, packages evidence, and
  archives the response.
- A peer maintainer or contributor cannot serve as the reviewer for
  this gate.
- If the assigned reviewer changes, replace the prior owner record
  with the new reviewer's identity and the reassignment date before
  the next request iteration.

## Approval Evidence Format

Approval (or a written rebrand decision) closes `L-10`, the
`clockify://` URI gate, and the gRPC service-name gate only when the
evidence archived in `docs/launch-readiness-review-may-8.md` carries
all of the following:

- Reviewer identity (full name, role, and CAKE.com / Clockify team).
- ISO 8601 decision date.
- Decision scope, listing the question numbers in this document that
  are answered. The `clockify://` URI gate requires question 5 to be
  answered; the gRPC service-name gate requires question 6 to be
  answered. A trademark approval that does not name questions 5 and 6
  leaves both transport-identifier gates open.
- Decision text (approve / approve-with-changes / rebrand / refuse)
  and the exact wording approved for each affected surface: repository
  name, npm package name, binary name, MCP tool prefix, `clockify://`
  URI scheme, gRPC service name, README, package metadata, container
  labels, GitHub repository description, and release notes.
- Rebrand instructions (when not approved): replacement names,
  schemes, and timelines, plus the maintainer who owns execution.
- Pointer to the originating ticket, email thread, or signed document,
  redacted of any private contact details before archival.

A `docs/release/brand-legal-review.md` edit alone does not close any
of these gates; it must be paired with the ledger entry above.

## Non-Goal of This Document

This file frames reviewer questions and is not approval. The following
local checks do **not** close `L-10`, the `clockify://` URI gate, or
the gRPC service-name gate:

- `make doc-parity`, `make release-check`, the launch-review ledger,
  and any other local verifier passing on this repository.
- `make license-evidence` raw inventory output (it is evidence input
  for counsel, not legal clearance).
- Local wording cleanups that frame the project as
  "launch-candidate evidence" rather than "official-product."
- A maintainer's personal opinion that the brand framing is
  "obviously fine."

## Questions for Reviewers

1. Is `go-clockify` an acceptable repository name for a public project
   under the current owner, or should it move to an approved
   organization/name before launch?
2. Is `@apet97/clockify-mcp-go` an acceptable npm package name, or
   should the package move or be renamed before the next release?
3. Is `clockify-mcp` an acceptable binary/container image name for
   public distribution?
4. Are `clockify_*` MCP tool names acceptable as descriptive API
   identifiers, or do they need a different public namespace?
5. Is `clockify://` acceptable as a resource URI scheme, or should the
   server use a less brand-like scheme before clients depend on it?
6. Is `clockify.mcp.v1.MCP` acceptable as the opt-in gRPC service
   name while MCP has no standardized gRPC service namespace, or
   should it use a less brand-like path before clients depend on it?
7. Does linking to `clockify.me`, `app.clockify.me`, or Clockify API
   docs require any disclaimer that the project is not endorsed until
   approval exists?
8. What exact wording, if any, is approved for public launch messaging
   after Group 1, Group 6, and Group 7 evidence closes?
9. If no approval is granted, what rebrand language should replace
   official-product framing in README, package metadata, release notes,
   repository description, and docs?
10. Is the MIT license posture acceptable with the current Go module,
   GitHub Actions, container, and npm wrapper dependency set?
11. What artifact should be archived as evidence: legal ticket,
    product sign-off, partnership note, approved copy deck, or a
    written no-approval/rebrand decision?

## Evidence to Attach

- `docs/launch-readiness-review-may-8.md` `L-10` disposition.
- Current README, npm package metadata, container labels, and GitHub
  repository description.
- `docs/api-coverage.md` and `docs/clients.md` examples that mention
  `clockify_*` tools or `clockify://` resources.
- `internal/transport/grpc/service.go` for the opt-in gRPC service
  name and hand-written service descriptor.
- Dependency and license evidence:
  - Default binary graph: fresh output from
    `go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' ./cmd/clockify-mcp | sort -u | sed '/^$/d'`.
    The May 9, 2026 local check returned only the root module; refresh
    this on the final candidate SHA before legal/product sign-off.
  - Build-tag module graphs require separate review for
    `internal/controlplane/postgres`, `internal/transport/grpc`, and
    `internal/tracing/otel`; attach `go-licenses` output or an
    equivalent counsel-approved license summary for each module.
    Use `make license-evidence-plan` to preview the local evidence
    collection, then `make license-evidence` (backed by
    `scripts/collect-license-evidence.sh`) to print raw
    `go list -deps` module graphs and local `LICENSE*`, `NOTICE*`, or
    `COPYING*` candidates for the default, FIPS, Postgres, gRPC,
    gRPC+Postgres, and OTel build variants. This is raw inventory only,
    not legal advice and not license clearance.
    A temporary May 9, 2026 run of
    `github.com/google/go-licenses@v1.6.0` did not produce usable
    evidence under the local toolchain: it failed on standard-library
    module-info loading, and nested optional modules need scanner-aware
    handling because their module roots do not carry separate license
    files. Treat any scanner output and the local inventory helper as
    evidence for counsel to review, not clearance.
  - npm wrapper metadata: `npm/package.json.tmpl` and
    `npm/clockify-mcp-go/package.json` declare MIT, but the public
    package names remain part of the brand review.
  - GitHub Actions and container supply-chain inputs are separate from
    the Go module graph. `.github/workflows/dependency-review.yml`
    currently has `license-check: false`, so it is vulnerability
    evidence only, not license-clearance evidence.
- Release candidate notes before publishing or public promotion.

## Decision Checklist (for the reviewer)

<!-- BEGIN external-review decision checklist (Lane F packet) -->

This subsection is the reviewer-facing decision-record template. It
is additive to the reviewer questions above and to the Approval
Evidence Format; it does not replace either. The maintainer may
hand the reviewer this section as a structured form to fill out,
mirror it into a ticket / signed document, or extract it into a
separate review report — the only contract is that the recorded
decision must answer every row before the trademark gate, the
`clockify://` URI gate, and the gRPC service-name gate can close.

### Reviewer engagement record

The maintainer fills in the engagement metadata before sending the
packet; the reviewer fills in the rest.

| Field | Value |
|---|---|
| Reviewer (full name, role, CAKE.com / Clockify team) | `<filled by reviewer>` |
| Maintainer routing the request | `@apet97` |
| Engagement reference (ticket / contract / email thread, redacted of private contact details) | `<filled by maintainer or reviewer>` |
| ISO 8601 decision date | `<filled by reviewer>` |
| Candidate tag this decision binds to (annotated tag SHA + peeled commit) | `<filled by maintainer at hand-off>` |

### Per-question decision record

For each row, the reviewer records one of: `approve`,
`approve-with-changes` (specify the exact wording approved),
`rebrand` (specify the replacement names / schemes / timelines),
or `refuse`. A row left blank leaves the corresponding gate open.

| Question (numbered against "Questions for Reviewers" above) | Decision | Approved wording / rebrand instruction |
|---|---|---|
| Q1 — Repository name `go-clockify` | | |
| Q2 — npm package name `@apet97/clockify-mcp-go` | | |
| Q3 — Binary / container image name `clockify-mcp` | | |
| Q4 — `clockify_*` MCP tool names as descriptive identifiers | | |
| Q5 — `clockify://` resource URI scheme | | |
| Q6 — `clockify.mcp.v1.MCP` opt-in gRPC service name | | |
| Q7 — Disclaimer requirement when linking to `clockify.me`, `app.clockify.me`, or Clockify API docs | | |
| Q8 — Approved public launch wording after Group 1, Group 6, and Group 7 close | | |
| Q9 — Replacement language if no approval is granted | | |
| Q10 — MIT license posture across the Go module, GitHub Actions, container, and npm wrapper dependency set | | |
| Q11 — Artifact archived as the closure evidence (legal ticket, product sign-off, partnership note, approved copy deck, or written no-approval / rebrand decision) | | |

### Branded-surface coverage record

Per the Approval Evidence Format above, the recorded decision must
also list the exact wording approved (or the rebrand instruction)
for **every** branded surface below. A row left blank means the
gate stays open for that surface even when Q1–Q11 are answered.

- [ ] Repository name (`go-clockify`).
- [ ] npm package name (`@apet97/clockify-mcp-go`).
- [ ] Binary name (`clockify-mcp`) and container image name
      (`ghcr.io/apet97/go-clockify`).
- [ ] MCP tool prefix (`clockify_*`).
- [ ] `clockify://` resource URI scheme used by `resources/list`
      and `resources/templates/list`.
- [ ] Opt-in gRPC service name (`clockify.mcp.v1.MCP`) at
      `internal/transport/grpc/service.go`; reviewer notes whether
      the decision applies before clients depend on the descriptor.
- [ ] README marketing copy and disclaimers (the project frames
      itself as a community MCP server; reviewer confirms whether
      that framing must change either way the decision lands).
- [ ] npm package metadata (`description`, `keywords`,
      `repository`, `homepage`).
- [ ] Container labels (OCI `org.opencontainers.image.*` fields
      in `.github/workflows/docker-image.yml` and
      `deploy/Dockerfile`).
- [ ] GitHub repository description.
- [ ] GitHub release notes templating.
- [ ] Public launch / promotion wording for blog posts, social
      announcements, or partner copy.

### Question-to-gate mapping (do not skip)

The May 8 ledger keeps three transport-identifier gates open under
one reviewer engagement:

- **Trademark / "official Clockify" language gate (`L-10`).** Closes
  only when Q1–Q4, Q7–Q9, and Q11 are answered AND the branded-
  surface coverage record is complete for repository name, npm
  package, binary, container, README, package metadata, container
  labels, repository description, and release notes.
- **`clockify://` URI scheme gate.** Closes only when Q5 is
  answered AND the URI-scheme row in the branded-surface coverage
  record is complete.
- **gRPC service-name gate.** Closes only when Q6 is answered AND
  the gRPC-service-name row in the branded-surface coverage record
  is complete.

If the reviewer's decision answers Q1–Q4 only (the "is the project
allowed to use Clockify naming at all" surface) and does not name
Q5 or Q6, the URI-scheme and gRPC-service-name gates stay open
until a follow-up decision lands per the Approval Evidence Format.

### Reviewer non-goals

The reviewer is **not** asked to:

- Sign off on the DPA / terms / privacy posture (separate gate;
  see [`dpa-privacy-evidence-checklist.md`](dpa-privacy-evidence-checklist.md)).
- Sign off on the paid-hosted external security review (separate
  gate; see [`external-security-review-request.md`](external-security-review-request.md)).
- Sign off on the paid-commercial RLS decision (separate ADR
  template under `docs/adr/`).
- Sign off on cross-replica hosted HTTP quota evidence (separate
  checklist in [`../runbooks/release-candidate-evidence.md`](../runbooks/release-candidate-evidence.md)).
- Approve any release. The reviewer's decision is one input among
  several to a launch-readiness call; it does not by itself
  authorize a `vX.Y.Z` tag, a public visibility flip, or any other
  release action.

### What this checklist does not do

This checklist is a request packet. It does not:

- Perform the review.
- Record the decision (the launch-readiness ledger does, per the
  Approval Evidence Format above).
- Replace the Approval Evidence Format. The Format is the
  contract; this checklist is one way to organize the data the
  Format requires.
- Grant any official-product status, endorsement, or partnership
  claim.

<!-- END external-review decision checklist (Lane F packet) -->

## Closure Rule

This gate closes only when a maintainer links written approval or a
rebrand decision from `docs/launch-readiness-review-may-8.md` and the
launch checklist. Local tests, doc parity, or phrasing cleanup do not
close it.
