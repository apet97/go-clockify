-- 003_drop_session_affinity_id.sql - remove unused session affinity column.
--
-- `session_affinity_id` was introduced before ADR 0017 settled on
-- strict principal revalidation plus control-plane rehydration. No
-- production path ever populated it, so every persisted row carried
-- the empty default and the column became misleading future surface.
--
-- Session stickiness remains a deployment/load-balancer optimization,
-- not part of the durable control-plane contract.

ALTER TABLE sessions
    DROP COLUMN IF EXISTS session_affinity_id;
