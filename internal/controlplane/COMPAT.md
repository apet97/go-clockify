# Control-plane schema compatibility

This file is the source of truth for `internal/controlplane`'s
persisted contract. Every change to the Postgres schema or to the
`Store` interface that crosses the persist boundary lands here along
with the commit that ships it.

See ADR 0011 (`docs/adr/0011-controlplane-schema-versioning.md`) for
the policy: forward-only migrations, startup compat guard, one source
of truth per backend.

## Postgres backend

| Version | Migration file | Ships | Summary |
|---------|----------------|-------|---------|
| 1 | `internal/controlplane/postgres/migrations/001_init.sql` | v1.1.0 (Wave B1) | Initial four-table schema: `tenants`, `credential_refs`, `sessions`, `audit_events`, plus `schema_migrations` and hot-path indexes. |

A version number appears here **once** — migrations are append-only.
Removing a column is a new row ("version 2 — drop obsolete foo").

## Store interface contract

The interface in `internal/controlplane/store.go` is the portable
surface; every backend (DevFileStore, Postgres, and anything landed
behind a future build tag) must satisfy it. Method additions are
backwards-compatible for *downstream callers* (they can ignore the
new method) but require an implementation in every existing backend.

| Method | Added | Semantics |
|--------|-------|-----------|
| `Tenant`, `PutTenant`, `CredentialRef`, `PutCredentialRef`, `Session`, `PutSession`, `DeleteSession`, `AppendAuditEvent` | v0.6.0 | Original eight methods. |
| `Close() error` | v1.1.0 (Wave B1.0) | Release backend-owned resources. `DevFileStore` returns nil. Postgres closes the pool. |
| `RetainAudit(ctx, maxAge) (int, error)` | v1.1.0 (Wave B2.1) | Drop audit events older than `maxAge`, return count removed. `maxAge <= 0` is a no-op. Called by the retention reaper in `cmd/clockify-mcp/retain.go`. |

## File / memory backend

The file-backed `DevFileStore` serialises its state as a single JSON
document (see `State` in `internal/controlplane/store.go`). There is
no `schema_version` field on the JSON — the file is implicitly v1 and
has been since the store shipped. If the on-disk shape changes, bump
the JSON to include a top-level `"version"` int and teach `load()` to
refuse unknown values. A future bump will add a corresponding row to
a "File backend" section below.

## When to update this file

- **Adding a Postgres migration**: add a row to the Postgres version
  table with the new file path, the release that ships it, and a
  one-line summary. `applyMigrations` will fail fast on a missing
  file, but a missing `COMPAT.md` row is on the honour system —
  reviewers should block the PR if the table is not updated.
- **Adding a `Store` interface method**: add a row to the contract
  table with the semantics and which release first shipped it.
  Existing backends must implement it before the PR lands.
- **Changing the file-backed JSON shape**: add a "File backend"
  section with the version bump and the migration logic (`load()`
  changes go in the same PR).

## Release cadence

Schema changes ship on the regular release train — there is no
separate "schema release". The only hard rule is that a rollback
across a schema bump requires a new binary with the inverse
migration, not `goose down`. See ADR 0011 for the rationale.
