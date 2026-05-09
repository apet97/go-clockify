-- 004_audit_events_tenant_at_index.sql - speed tenant-scoped audit review.
--
-- Operators and support runbooks commonly inspect a single tenant's
-- audit trail over a bounded time window. 001_init.sql already keeps a
-- global `at` index for retention scans; this composite index keeps
-- forensic per-tenant queries from depending on a full-table filter
-- once hosted/shared-service audit volume grows.

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_id_at
    ON audit_events (tenant_id, at);
