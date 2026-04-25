-- 002_audit_phase.sql — persist the two-phase audit phase column.
--
-- The `phase` field on controlplane.AuditEvent is part of the
-- intent/outcome two-phase audit pattern (commit c996b73). The
-- DevFileStore round-trips it transparently because the JSON tag is
-- "phase,omitempty"; the Postgres backend would silently drop it
-- because 001_init.sql does not declare the column and
-- AppendAuditEvent's INSERT did not name it.
--
-- DEFAULT '' keeps legacy single-shot rows (Phase: "") readable by
-- old binaries via the JSON omitempty fallback. The new INSERT path
-- writes the empty string explicitly for those rows.
--
-- The retention reaper still uses idx_audit_events_at; idx_audit_events_phase
-- is added for forensic queries that filter by intent/outcome over a
-- bounded time window (e.g. "show every intent without a matching
-- outcome in the last hour" — the canonical audit-trail integrity
-- check for the fail_closed durability mode).

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_audit_events_phase
    ON audit_events (phase);
