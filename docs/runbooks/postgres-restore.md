# Postgres Backup and Restore Runbook

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
