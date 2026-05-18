# Error Recovery

Recoverable tool failures return `ok:false` in `structuredContent` with an
`error.code`, a cleaned `error.message`, and a `recovery` object. The cleaned
message removes internal method/path prefixes while server logs keep the full
diagnostic.

| Code / shape | Meaning | Recovery |
| --- | --- | --- |
| `feature_unavailable` | Clockify denied or does not expose the feature for this workspace, plan, or role. | Run `clockify_status` or `doctor --live`, check `optional_features`, then continue with a supported workflow or report the missing plan/permission. |
| `timeout` | The tool exceeded `CLOCKIFY_TOOL_TIMEOUT`. | Retry with narrower filters, a lower page size, or a more specific get/list tool. Increase `CLOCKIFY_TOOL_TIMEOUT` only for legitimately long operations. |
| `cancelled` | The client cancelled the request before the handler completed. | Retry when the client is ready to keep the request open. Cancelled `tools/call` responses may be suppressed by protocol design. |
| `result_too_large` | The result exceeded `CLOCKIFY_MAX_TOOL_RESULT_BYTES` and could not be safely truncated. | Use narrower filters, lower `page_size` or `max_rows`, or switch to a paged/detail tool. Export tools may return a temp-file path instead of inline data. |
| `unsupported` | The named action has no Clockify API endpoint in this surface. | Follow the tool's recovery hint. Examples: send invoice email from the UI, trigger a real webhook event, or delete and re-add an invoice line item. |
| JSON-RPC `-32602` | Input arguments failed MCP/runtime schema validation before the handler ran. | Fix the argument named in `error.data.pointer`; numbers may arrive as JSON numbers, but wrong types, invalid enum values, and missing required fields are rejected locally. |
| Clockify permission / HTTP error | Upstream rejected a valid request. | Check role and plan in `docs/permissions.md`, use `dry_run:true` for risky tools, and keep the exact tool/error context for support. |

Do not weaken schemas or route fences to make a recovery envelope disappear.
The honest result is often better than a permissive call that hides a Clockify
permission, plan, or endpoint boundary.
