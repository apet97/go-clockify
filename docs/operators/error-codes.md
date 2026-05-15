# MCP HTTP Error Codes

> **Historical artifact. Not current one-user MCP product documentation.**
> Preserved for platform-era audit/history only. Start current one-user work from `README.md`, `docs/agent-cookbook.md`, `docs/tool-catalog.md`, and `docs/goals/oneuser-tool-coverage.md`.


The HTTP transports keep HTTP status codes for client retry/back-off
logic. MCP endpoint admission failures also return a JSON-RPC 2.0
envelope with `id:null` because the request has not reached JSON-RPC
dispatch.

Example:

```json
{
  "jsonrpc": "2.0",
  "id": null,
  "error": {
    "code": -32012,
    "message": "rate limited"
  }
}
```

| Code | HTTP status | Meaning |
|---:|---:|---|
| `-32001` | `400` or `404` | Missing, expired, or unknown streamable HTTP session. |
| `-32002` | `200` JSON-RPC response | Server not initialized; send `initialize` before normal requests. |
| `-32010` | `403` | Request `Origin` is not in the configured CORS allowlist. |
| `-32011` | `403` | Request `Host` failed strict host checking. |
| `-32012` | `429` | Process-local HTTP admission limit rejected the request. |
| `-32013` | `413` | Request body exceeded `MCP_MAX_MESSAGE_SIZE`. |
| `-32014` | `403` | Session ID was replayed with a different authenticated principal. |
| `-32015` | `405` | Unsupported HTTP method for the selected transport route. |
| `-32020` | `401` | Authentication failed; inspect `WWW-Authenticate` for OAuth bearer details. |
| `-32021` | `403` | Reserved for future explicit authorization denials outside session-principal mismatch. |

Standard JSON-RPC codes are still used for JSON parse and request-shape
failures, for example `-32700` for invalid JSON and `-32600` for an
invalid JSON-RPC request.
