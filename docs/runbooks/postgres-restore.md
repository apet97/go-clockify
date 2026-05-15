# Postgres Backup and Restore Runbook

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


This runbook outlines the exact commands to take a Postgres backup from a production environment and restore it into a fresh staging namespace to confirm tenants, sessions, and audit history can be read.

## Prerequisites

* `kubectl` installed and configured for the target clusters (Production & Staging).
* `pg_dump` and `pg_restore` (or `psql`) installed locally or accessible via a debug pod.
* Proper RBAC permissions to access the database pods in both namespaces.

## 1. Take a Backup (Production)

Assuming your Postgres instance is running in a pod named `postgres-prod-0` in the `clockify-prod` namespace:

```bash
# Set environment variables
PROD_NAMESPACE="clockify-prod"
DB_POD="postgres-prod-0"
DB_NAME="clockify_mcp"
DB_USER="clockify_admin"

# Execute pg_dump to create a backup file locally
kubectl exec -n $PROD_NAMESPACE $DB_POD -- \
  pg_dump -U $DB_USER -F c $DB_NAME > clockify_mcp_backup.dump
```

## 2. Restore into a Fresh Namespace (Staging)

Assuming you have a fresh Postgres instance running in `postgres-staging-0` in the `clockify-staging` namespace:

```bash
# Set staging environment variables
STAGING_NAMESPACE="clockify-staging"
STAGING_DB_POD="postgres-staging-0"

# 1. Copy the dump to the staging pod
kubectl cp clockify_mcp_backup.dump $STAGING_NAMESPACE/$STAGING_DB_POD:/tmp/clockify_mcp_backup.dump

# 2. Restore the database
kubectl exec -n $STAGING_NAMESPACE $STAGING_DB_POD -- \
  pg_restore -U $DB_USER -d $DB_NAME -1 /tmp/clockify_mcp_backup.dump

# 3. Clean up the dump file from the pod
kubectl exec -n $STAGING_NAMESPACE $STAGING_DB_POD -- rm /tmp/clockify_mcp_backup.dump
```

## 3. Verification Steps

After the restore is complete, perform the following queries to verify data integrity:

```bash
# Open a psql session on the staging database
kubectl exec -it -n $STAGING_NAMESPACE $STAGING_DB_POD -- psql -U $DB_USER -d $DB_NAME
```

Run these SQL commands:

```sql
-- 1. Confirm Tenants exist
SELECT count(*) FROM tenants;

-- 2. Confirm active Sessions
SELECT count(*) FROM sessions WHERE expires_at > NOW();

-- 3. Confirm Audit History is present
SELECT count(*) FROM audit_events;
```

Ensure the row counts match what is expected from the production database at the time of the backup.

## 4. Tenant Isolation / RLS Check

The current v1.x control-plane schema relies on application-layer
tenant scoping; it does not yet ship database-enforced Postgres RLS
policies. Do not use this restore runbook as evidence that the
paid-hosted RLS launch gate is closed until an RLS migration and test
have landed.

When an RLS migration exists, every production restore drill must add
these checks before the staging database is used for tenant traffic:

```sql
-- Confirm tenant-scoped tables have row security enabled.
SELECT relname, relrowsecurity
FROM pg_class
WHERE relname IN ('tenants', 'credential_refs', 'sessions', 'audit_events');

-- Confirm the expected tenant policies were restored.
SELECT schemaname, tablename, policyname
FROM pg_policies
WHERE tablename IN ('tenants', 'credential_refs', 'sessions', 'audit_events')
ORDER BY tablename, policyname;
```

Then run a two-tenant smoke against the restored staging deployment:

1. Set the application tenant context for tenant A and confirm tenant
   B rows are not visible.
2. Set the application tenant context for tenant B and confirm tenant
   A rows are not visible.
3. Run `clockify-mcp-postgres doctor --profile=prod-postgres --strict --check-backends`
   against the restored staging deployment.

If any table reports `relrowsecurity = false`, if an expected policy
is missing, or if the two-tenant smoke can read across tenants, treat
the restore as failed and keep the environment closed to traffic.
