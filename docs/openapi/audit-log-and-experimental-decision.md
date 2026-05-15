# Audit Log And Experimental OpenAPI Decision

Date: 2026-05-15

The current one-user MCP product keeps the startup registry fixed at 152 tools:
workflow tools first, domain tools second, and the raw API fallback last.

## Audit Log

`POST /workspaces/{workspaceId}/audit-log` lives on
`https://auditlog-api.api.clockify.me/v1`, not the default Clockify API host.
It stays documented as an OpenAPI coverage gap rather than a typed MCP tool
until the product contract explicitly allows either a new tool or host-aware raw
fallback behavior.

Live success for this route is API evidence only. It is not catalog coverage,
and it must not increase the startup tool count.

## Entity Changes

The `/workspaces/{workspaceId}/entities/{created,updated,deleted}` routes are
tagged `Entity changes (Experimental)` in the referenced OpenAPI document. They
stay unsupported by typed tools until the maintainer explicitly opts into
experimental API coverage.

## Raw Fallback Boundary

`clockify_api_get` and `clockify_api_request` remain pinned-workspace escape
hatches for the configured base API host. They are intentionally not a blanket
multi-host router for audit-log or experimental endpoints.
