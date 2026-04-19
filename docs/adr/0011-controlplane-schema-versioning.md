# 0011 - Control-plane schema versioning

## Status

Accepted — the guard is enforced on every boot against the Postgres
backend; see `internal/controlplane/postgres/migrations.go:applyMigrations`.

## Context

`internal/controlplane` is the durable surface for tenants, credential
references, sessions, and the audit log. Wave B1 introduced a
pgx-backed implementation behind `-tags=postgres` alongside the
existing file/memory `DevFileStore`. Two new failure modes appear as
soon as a schema lives in a database instead of the `Store` interface:

1. **Forward drift** — a newer binary ships with a new migration; an
   older binary rolling back must not silently treat the new column
   as absent. Silent divergence here is the classic "mysterious
   missing field" incident.
2. **Manual drift** — an operator or a run-once script mutates the
   schema (adds a column, renames a table) outside the migration
   path. The binary cannot tell whether it is safe to read or write.

The file-backed `DevFileStore` was spared both modes because its
schema is whatever the binary's Go types say it is. That guarantee
stops being free the moment we persist outside the process.

Two paths were evaluated:

- **Promote `internal/controlplane` to `pkg/controlplane`** — makes
  the store a consumable public surface. Cost: every importer
  (`cmd/clockify-mcp`, `internal/mcp`, tests) gets a path rewrite;
  public API evolution becomes a semver commitment; there is no
  external consumer today. Rejected.
- **Keep in `internal/`, version the persisted schema, document the
  contract** — localises the blast radius to the Postgres backend
  without widening the package's API surface. Accepted.

## Decision

1. **Forward-only embedded migrations.** `internal/controlplane/postgres/migrations/`
   contains numbered SQL files (`001_init.sql`, …). `embed.FS` pulls
   them into the binary; `applyMigrations` runs them in order under a
   `pg_advisory_lock` at startup so concurrent boots against one
   database are safe. Each applied migration is recorded in a
   `schema_migrations (version INT PK, applied_at TIMESTAMPTZ)` table.

2. **Schema-compat guard.** On every boot, after applying missing
   migrations, the applier reads `MAX(version)` from
   `schema_migrations` and compares it to the highest version the
   binary knows about (derived from the `embed.FS` at compile time).
   If the database has a strictly newer version, `Open` returns:

   > controlplane/postgres: database schema is at version N but this
   > binary only knows up to M; upgrade the binary or roll the
   > database back

   and the process refuses to start.

3. **No down migrations.** Migration files are strictly additive. A
   "remove column" change is implemented as a new migration that
   drops the column in the next binary release; rolling back the
   binary still sees a valid read surface because the columns are
   only removed *after* the new binary has been in production long
   enough to roll back through (documented in `COMPAT.md`). This is
   the same policy the Go standard library uses for exported API
   deprecations.

4. **Schema contract document.** `internal/controlplane/COMPAT.md`
   tracks every version with a one-line description and a pointer to
   the migration file plus the interface-method changes (if any)
   shipped in that version.

5. **Package stays `internal/`.** The `Store` interface remains part
   of the main module (no `pkg/controlplane` move). External consumers
   can take a dependency by vendoring or by upstreaming a request;
   neither case is realistic in the next release window.

## Consequences

### Positive

- Operators cannot silently boot a stale binary against a database
  that has moved past it. The error message names the versions and
  the remediation.
- Migrations are legible — each file is a unit of change reviewable
  in isolation, and the `schema_migrations` table gives operators a
  one-row answer to "what version is this DB on?".
- `pg_advisory_lock` makes horizontal scale-outs and CI parallelism
  trivially safe without relying on naming conventions or ordering
  tricks.
- The file/memory `DevFileStore` remains untouched — this policy
  applies only to the Postgres backend.

### Negative

- No automatic down-migrations. Rolling back a shipped migration
  requires writing a new "undo" migration and cutting a new release,
  not running `goose down`. We think this is the right default — a
  down migration that deletes rows is generally more dangerous than
  living with the forward change — but it is a sharp edge.
- The applier is ~160 LOC of hand-written Go (`migrations.go`) plus
  SQL files. A third-party library would be smaller in Go but
  forbidden under ADR 0001.
- Contributors adding a new column need to remember to bump the
  `NNN_` file number and update `COMPAT.md`. The applier fails fast
  on duplicate version numbers; `COMPAT.md` is on the honour system.

### Neutral

- The `schema_version` column was explicitly considered as an
  alternative (one column per table, written on every row). Rejected
  as over-engineering: a single `schema_migrations` row plus the
  compat check gives the same safety guarantee without bloating
  every write path.

## Alternatives considered

- **`pressly/goose` or `golang-migrate`** — off the table under
  ADR 0001. Neither lives in the top-level binary's dependency
  graph, neither's build-tag gating would be worth the complexity,
  and a hand-rolled applier at this scale is ~150 LOC.
- **Promote to `pkg/`** — described above. Deferred; revisit if a
  second external consumer materialises.
- **Per-row `schema_version` column** — described above. Rejected.
- **No guard, rely on operator discipline** — the whole point of
  writing an ADR is that we are not willing to rely on that.

## References

- Code: `internal/controlplane/postgres/migrations.go`
  (`applyMigrations`), `internal/controlplane/postgres/migrations/*.sql`.
- Tests: `internal/controlplane/postgres/migrations_test.go`
  (`TestSchemaCompatGuard_RefusesFutureVersion`).
- Schema contract: `internal/controlplane/COMPAT.md`.
- Related ADRs: ADR 0001 (stdlib-only default build — explains why
  the migration library is hand-rolled), ADR 0006 / ADR 0007 /
  ADR 0008 (the other build-tag sub-module precedents).
