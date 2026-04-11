# GOCLMCP Architecture

This document explains the runtime structure of `clockify-mcp` — what each
layer owns, how a tool call flows end-to-end, and where the safety gates
sit. For the rationale behind individual design decisions (stdlib-only,
pluggable enforcement, dispatch vs. business-layer semaphores, …) see the
ADRs under [`docs/adr/`](adr/).

## Layer diagram

```
+--------------------------------------------------------------+
|                        MCP client                            |
|              (Claude Desktop, Cursor, curl, …)               |
+--------------------------------------------------------------+
                           | stdio | HTTP | streamable HTTP
                           v
+-----------------------------+  +-----------------------------+
|      Protocol core (mcp)    |  |  Transports                 |
|  - stdio loop               |  |  - stdio (Run)              |
|  - JSON-RPC dispatch        |<-|  - legacy HTTP (transport_  |
|  - tools/list, tools/call,  |  |    http.go)                 |
|    resources/*, prompts/*,  |  |  - streamable HTTP          |
|    initialize, ping,        |  |    (transport_streamable_   |
|    notifications/cancelled  |  |    http.go)                 |
|  - Server.callTool          |  +-----------------------------+
+--------------+--------------+
               |
     Enforcement interface (pluggable)
               |
               v
+--------------------------------------------------------------+
|                  Safety layer (enforcement)                  |
|   Pipeline.FilterTool / BeforeCall / AfterCall               |
|   - policy gate    (internal/policy)                         |
|   - rate limit     (internal/ratelimit, global + per-token)  |
|   - dry-run        (internal/dryrun)                         |
|   - truncation     (internal/truncate)                       |
|   - bootstrap      (internal/bootstrap)                      |
+--------------------------------------------------------------+
               |
     ToolHandler (registered in internal/tools)
               |
               v
+--------------------------------------------------------------+
|              Tool surface (internal/tools)                   |
|   Service struct with lazy user/workspace cache              |
|   - 33 Tier 1 handlers: users, workspaces, entries, timer,   |
|     projects, clients, tags, tasks, reports, workflows,      |
|     context/discovery, resources, prompts                    |
|   - 91 Tier 2 handlers across 11 lazy-loaded groups          |
+--------------------------------------------------------------+
               |
        Clockify HTTP client (internal/clockify)
               |
               v
+--------------------------------------------------------------+
|                    Clockify API                              |
+--------------------------------------------------------------+
```

## 1. Tool-call enforcement flow

Every `tools/call` traverses the same pipeline, regardless of transport.

```mermaid
sequenceDiagram
    autonumber
    participant C as MCP client
    participant T as Transport (stdio / HTTP / streamable)
    participant S as Server.handle
    participant E as Pipeline.BeforeCall
    participant P as policy gate
    participant R as rate limit
    participant H as ToolHandler
    participant A as Pipeline.AfterCall
    participant U as Clockify API

    C->>T: JSON-RPC tools/call
    T->>S: req
    S->>S: initialized guard (-32002 if not)
    S->>S: decode params, extract _meta.progressToken
    S->>E: BeforeCall(ctx, name, args, hints)
    E->>P: IsAllowed(name, hints.readOnly)
    P-->>E: allow / deny
    E->>R: AcquireForSubject(ctx, principal.Subject)
    R-->>E: release fn / rate-limited
    E->>E: dry-run intercept (if enabled)
    E-->>S: (short-circuit result | release fn)
    alt BeforeCall short-circuited (dry-run)
        S-->>T: envelope wrapping dry-run preview
    else normal path
        S->>H: handler(callCtx, args)
        H->>U: HTTP GET/POST...
        U-->>H: JSON response
        H-->>S: typed data / err
        S->>A: AfterCall(result)
        A-->>S: truncated result
    end
    S->>S: release(), metrics, audit, span.End
    S-->>T: JSON-RPC Response
    T-->>C: JSON-RPC Response
```

## 2. Dry-run interception

```mermaid
sequenceDiagram
    autonumber
    participant S as Server.handle
    participant E as Pipeline.BeforeCall
    participant D as dryrun.Pipeline
    participant PT as Preview tool lookup
    participant M as MinimalResult

    S->>E: BeforeCall(destructive tool)
    E->>D: CheckDryRun(name, args, destructive)
    D-->>E: action (ConfirmPattern | PreviewTool | MinimalFallback | NotDestructive)
    alt ConfirmPattern
        E->>M: MinimalResult(name, args)
        M-->>E: minimal envelope
        E-->>S: (result, release, nil)  [handler NOT called]
    else PreviewTool
        E->>PT: PreviewToolFor(name)
        PT-->>E: preview handler
        E->>E: BuildPreviewArgs(args)
        E->>PT: handler(ctx, previewArgs)
        PT-->>E: preview result
        E-->>S: WrapResult(preview, name)
    else MinimalFallback
        E->>M: MinimalResult(name, args)
        M-->>E: minimal envelope
        E-->>S: (result, release, nil)
    else NotDestructive
        E-->>S: error (tool is not destructive)
    end
```

## 3. Tier 2 activation

```mermaid
sequenceDiagram
    autonumber
    participant C as MCP client
    participant S as Server.handle
    participant H as clockify_search_tools handler
    participant A as Service.ActivateGroup
    participant SV as Server.ActivateGroup
    participant N as Notifier

    C->>S: tools/call clockify_search_tools {activate_group:"invoices"}
    S->>H: handler(ctx, args)
    H->>A: ActivateGroup(ctx, "invoices")
    A->>A: Tier2Handlers("invoices") — build + tighten + decorate
    A->>SV: ActivateGroup("invoices", descriptors)
    SV->>SV: register 12 new tool descriptors
    SV->>N: Notify("notifications/tools/list_changed", {})
    N-->>C: async SSE event (streamable) or drop (legacy HTTP)
    SV-->>A: ok
    A-->>H: ActivationResult
    H-->>S: envelope {activated:"invoices", toolCount:12}
    S-->>C: JSON-RPC Response
    C->>S: tools/list (re-fetch)
    S-->>C: 45 tools (33 Tier 1 + 12 newly activated Tier 2)
```

## 4. Graceful shutdown (stdio transport)

```mermaid
sequenceDiagram
    autonumber
    participant Sig as SIGTERM
    participant R as Server.Run loop
    participant I as Inflight map
    participant H as ToolHandler (in-flight)
    participant C as Clockify client

    Sig->>R: signal
    R->>R: ctx.Cancel()
    R->>R: scanner loop observes ctx.Done, stops reading
    par drain in-flight calls
        R->>I: iterate registered cancels
        R->>H: per-call ctx.Done propagates
        H->>C: in-flight HTTP request aborts via ctx
        H-->>R: returns error / cancelled envelope
    end
    R->>R: wait for all callTool goroutines
    R->>R: encoder.Flush, audit final events
    R-->>Sig: process exits
```

## 5. Streamable HTTP session lifecycle

```mermaid
sequenceDiagram
    autonumber
    participant C as MCP client
    participant H as HTTP transport (streamable)
    participant M as sessionManager
    participant F as session factory
    participant S as per-session Server
    participant EH as sessionEventHub

    C->>H: POST /mcp {initialize}
    H->>H: authenticate, attach Principal to ctx
    H->>M: create(id, principal)
    M->>F: Factory(ctx, principal, id)
    F-->>M: Server (isolated tool state)
    M->>EH: newSessionEventHub
    M-->>H: streamSession
    H->>S: handle(initialize)
    S-->>H: result {capabilities, protocolVersion}
    H-->>C: 200 + X-MCP-Session-ID

    C->>H: GET /mcp (Accept: text/event-stream, Last-Event-ID: 12)
    H->>M: get(session id)
    H->>EH: subscribeFrom(12)
    EH-->>H: channel (backlog replay events > 12, then live)
    loop
        EH-->>H: sessionEvent (id stamped)
        H-->>C: "id: N\nevent: <method>\ndata: <json>\n\n"
    end

    C->>H: POST /mcp {tools/call} + X-MCP-Session-ID + Mcp-Protocol-Version
    H->>H: validateProtocolVersion vs session.negotiated
    H->>S: handle(tools/call)
    S-->>H: result
    H-->>C: Response
```

## Related

- [Wave 1 backlog](wave1-backlog.md) — curated remaining work and landed items
- [Observability](observability.md) — metrics, SLOs, alerts, tracing
- [HTTP transport guide](http-transport.md)
- [Runbooks](runbooks/)
- [Architecture Decision Records](adr/)
