# Brand and Legal Review Questions

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

## Closure Rule

This gate closes only when a maintainer links written approval or a
rebrand decision from `docs/launch-readiness-review-may-8.md` and the
launch checklist. Local tests, doc parity, or phrasing cleanup do not
close it.
