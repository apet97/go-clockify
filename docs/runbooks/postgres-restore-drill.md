# Runbook: Postgres Restore Drill

This runbook describes the procedure for a periodic database restore drill to ensure data integrity and recovery readiness.

## Objective
Restore a production database backup to a staging environment and verify that `clockify-mcp` can connect and function correctly.

## Prerequisites
- Access to production Postgres backups (S3, GCS, or local).
- A clean staging Postgres instance.
- `psql` or `pg_restore` installed on the operator machine.

## Drill Steps

### 1. Retrieve the Latest Backup
Download the latest production backup from the storage bucket.
```bash
aws s3 cp s3://my-backups/clockify-mcp/latest.dump .
```

### 2. Prepare the Staging Database
Create a new database in the staging environment.
```bash
psql -h staging-db -U admin -c "CREATE DATABASE clockify_mcp_drill;"
```

### 3. Restore the Data
Use `pg_restore` to populate the staging database.
```bash
pg_restore -h staging-db -U admin -d clockify_mcp_drill latest.dump
```

### 4. Configure `clockify-mcp` to use the Drill Database
Update the environment variables for a staging instance:
```env
MCP_CONTROL_PLANE_DSN=postgres://admin:pass@staging-db:5432/clockify_mcp_drill?sslmode=require
```

### 5. Verify Application Functionality
- [ ] Start the application and check for migration errors in logs.
- [ ] Run a simple `clockify_whoami` tool call.
- [ ] List all active tenants to ensure data parity.
- [ ] Verify audit log entries from the past 24 hours are present.

## Post-Drill Cleanup
Once verified, decommission the staging database.
```bash
psql -h staging-db -U admin -c "DROP DATABASE clockify_mcp_drill;"
```

## Sign-off
Record the date and outcome of the drill in the maintenance log.
