package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
	"github.com/apet97/go-clockify/internal/ratelimit"
)

// SupportedProtocolVersions lists MCP protocol versions this server can
// negotiate, newest first. The first entry is returned as the default when
// the client does not send a protocolVersion. When a client requests an
// unsupported version, we echo back the newest supported version — clients
// that cannot downgrade will treat that as an error and disconnect, which is
// the spec-compliant behaviour.
var SupportedProtocolVersions = []string{
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

// ServerInstructions is returned in the initialize response to teach MCP
// clients how to navigate the server. Agentic clients consume this as part
// of their system prompt, so it trades some verbosity for clarity.
const ServerInstructions = `This is the Clockify Go MCP server. It exposes a tiered tool surface and safety enforcement for Clockify operations.

Tool tiers:
  - Tier 1 tools are registered at startup and visible in tools/list.
  - Tier 2 tools are organised into domain groups (invoices, expenses, scheduling, time_off, approvals, shared_reports, user_admin, webhooks, custom_fields, groups_holidays, project_admin) and activated on demand.
  - Use 'clockify_search_tools' to discover tools by keyword or group name.
  - Use 'clockify_activate_group' or 'clockify_activate_tool' to enable Tier 2 tools before calling them.

Safety:
  - The server supports four policy modes: read_only, safe_core, standard (default), full.
  - Destructive tools run through a dry-run interceptor by default; pass dry_run:true to preview, dry_run:false to execute.
  - Use 'clockify_policy_info' to inspect the active policy and dry-run configuration.
  - Tool arguments that reference entities by name (project, client, tag, user) are resolved strictly; ambiguous matches are rejected rather than guessed.

Errors are returned in the MCP tool-error envelope (result.content + isError:true) for tool failures, and as JSON-RPC errors only for protocol violations.`

// Notifier delivers server-initiated notifications (e.g. tools/list_changed)
// to the connected client. Transports implement this: the stdio transport
// writes through the shared JSON encoder, while the legacy HTTP POST-only
// transport logs + counts drops until a real SSE channel is wired by the
// Streamable HTTP transport rewrite.
type Notifier interface {
	Notify(method string, params any) error
}

// droppingNotifier is installed by the legacy HTTP transport. It records
// every drop so operators can measure the gap until Streamable HTTP ships.
type droppingNotifier struct{}

func (droppingNotifier) Notify(method string, _ any) error {
	metrics.ProtocolErrorsTotal.Inc("notification_dropped")
	slog.Warn("notification_dropped",
		"method", method,
		"reason", "legacy_http_transport",
		"hint", "migrate to Streamable HTTP for server-initiated notifications",
	)
	return nil
}

// encoderNotifier adapts the stdio JSON encoder (and its mutex) to the
// Notifier interface. Decoupling notification delivery from the raw encoder
// lets transports plug in their own delivery mechanism without the server
// core holding transport-specific state.
type encoderNotifier struct {
	mu      *sync.Mutex
	encoder **json.Encoder
}

func (e encoderNotifier) Notify(method string, params any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.encoder == nil || *e.encoder == nil {
		return nil
	}
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		msg["params"] = params
	} else {
		// The spec strongly prefers a params object or array even when empty.
		msg["params"] = map[string]any{}
	}
	return (*e.encoder).Encode(msg)
}

type ToolHandler func(context.Context, map[string]any) (any, error)

type ToolDescriptor struct {
	Tool            Tool
	Handler         ToolHandler
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
}

type Server struct {
	Version      string
	Enforcement  Enforcement                     // nil = no filtering or enforcement
	Activator    Activator                       // nil = activation unrestricted
	ToolTimeout  time.Duration                   // per-call timeout; 0 = default 45s
	ReadyChecker func(ctx context.Context) error // optional upstream health check for /ready

	// ResourceProvider backs resources/* method handlers. nil disables the
	// resources capability (server omits it from initialize.result.capabilities).
	ResourceProvider ResourceProvider
	resourceSubs     resourceSubscriptions

	// prompts registers the built-in prompt templates surfaced via prompts/*.
	prompts *promptRegistry

	// MaxInFlightToolCalls bounds the number of concurrently-running
	// tools/call goroutines spawned by the stdio dispatch loop.
	// Acquired before the goroutine is created so bursty input cannot
	// amplify goroutine count. 0 = unlimited.
	MaxInFlightToolCalls int

	// StrictHostCheck enables DNS rebinding protection on the HTTP
	// transport: the inbound Host header must match either a loopback
	// literal or one of the configured allowed-origin hostnames. Defaults
	// to off so that reverse-proxy deployments that rewrite Host are
	// unaffected; flip on (MCP_STRICT_HOST_CHECK=1) in localhost-bound
	// production deployments to get the strict guarantee.
	StrictHostCheck bool

	mu          sync.RWMutex
	tools       map[string]ToolDescriptor
	initialized atomic.Bool
	// advertiseListChanged controls whether initialize reports
	// capabilities.tools.listChanged=true. Only transports that can actually
	// deliver notifications/tools/list_changed should set this.
	advertiseListChanged atomic.Bool
	encoder              *json.Encoder // stored for push notifications
	encoderMu            sync.Mutex    // protects concurrent encoder writes
	requestSeq           atomic.Int64  // monotonic request ID for log correlation

	// notifier delivers server→client notifications. Set by the transport
	// layer (stdio Run or HTTP ServeHTTP) at startup. nil = drop with a log.
	notifier Notifier

	// Negotiated client info. Populated on successful initialize; read by
	// downstream log calls via NegotiatedProtocolVersion() / ClientInfo().
	negotiatedMu      sync.RWMutex
	negotiatedVersion string
	clientName        string
	clientVersion     string

	toolCallSem chan struct{} // dispatch-layer goroutine cap; nil = unlimited

	Auditor        Auditor
	AuditTenantID  string
	AuditSubject   string
	AuditSessionID string
	AuditTransport string

	// readiness cache
	readyMu     sync.Mutex
	readyCached bool
	readyAt     time.Time

	// inflight tracks cancellable contexts for in-flight tools/call
	// requests, keyed by JSON-RPC request ID. notifications/cancelled
	// looks up the ID and aborts the in-flight tool handler. Nil IDs
	// (notifications) are not tracked.
	inflightMu sync.Mutex
	inflight   map[any]context.CancelFunc
}

// SetNotifier installs a notification sink. Transports call this during
// startup. Safe to call once per server lifetime; later calls replace the
// sink without synchronisation, so transports must call before traffic flows.
func (s *Server) SetNotifier(n Notifier) {
	s.notifier = n
}

// NegotiatedProtocolVersion returns the MCP protocol version agreed with the
// client, or empty string before initialize runs.
func (s *Server) NegotiatedProtocolVersion() string {
	s.negotiatedMu.RLock()
	defer s.negotiatedMu.RUnlock()
	return s.negotiatedVersion
}

// ClientInfo returns the client name and version sent during initialize.
func (s *Server) ClientInfo() (name, version string) {
	s.negotiatedMu.RLock()
	defer s.negotiatedMu.RUnlock()
	return s.clientName, s.clientVersion
}

func NewServer(version string, descriptors []ToolDescriptor, enforcement Enforcement, activator Activator) *Server {
	toolMap := make(map[string]ToolDescriptor, len(descriptors))
	for _, d := range descriptors {
		toolMap[d.Tool.Name] = d
	}
	return &Server{
		Version:     version,
		Enforcement: enforcement,
		Activator:   activator,
		tools:       toolMap,
		inflight:    make(map[any]context.CancelFunc),
		prompts:     newPromptRegistry(),
	}
}

// registerInflight stores a cancel func keyed by JSON-RPC request ID so
// notifications/cancelled can abort the in-flight tool handler. Nil IDs
// (notifications) and zero-value uninitialised maps are no-ops.
func (s *Server) registerInflight(id any, cancel context.CancelFunc) {
	if id == nil {
		return
	}
	s.inflightMu.Lock()
	if s.inflight == nil {
		s.inflight = make(map[any]context.CancelFunc)
	}
	s.inflight[id] = cancel
	s.inflightMu.Unlock()
}

// unregisterInflight removes a request from the inflight map. Idempotent.
func (s *Server) unregisterInflight(id any) {
	if id == nil {
		return
	}
	s.inflightMu.Lock()
	delete(s.inflight, id)
	s.inflightMu.Unlock()
}

// cancelInflight cancels and removes an inflight request by ID. Returns
// true when a matching request was found.
func (s *Server) cancelInflight(id any) bool {
	if id == nil {
		return false
	}
	s.inflightMu.Lock()
	cancel, ok := s.inflight[id]
	if ok {
		delete(s.inflight, id)
	}
	s.inflightMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// InflightCount returns the number of tracked in-flight tools/call
// requests. Used by tests to verify the map is cleaned up.
func (s *Server) InflightCount() int {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	return len(s.inflight)
}

// Run processes JSON-RPC requests from r and writes responses to w.
// It respects ctx cancellation for graceful shutdown — when ctx is
// cancelled, the loop exits even if stdin is blocking.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) error {
	if s.MaxInFlightToolCalls > 0 && s.toolCallSem == nil {
		s.toolCallSem = make(chan struct{}, s.MaxInFlightToolCalls)
	}

	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	s.encoderMu.Lock()
	s.encoder = json.NewEncoder(w)
	s.encoderMu.Unlock()
	// Install the stdio notifier so activation events (tools/list_changed)
	// flow back through the same thread-safe encoder the responses use.
	if s.notifier == nil {
		s.notifier = encoderNotifier{mu: &s.encoderMu, encoder: &s.encoder}
	}
	s.advertiseListChanged.Store(true)

	// Channel-based approach: scan lines in a goroutine so we can
	// select on ctx.Done() in the main loop.
	type scanResult struct {
		line []byte
		ok   bool
	}
	lines := make(chan scanResult, 1)

	go func() {
		defer close(lines)
		for scanner.Scan() {
			cpy := make([]byte, len(scanner.Bytes()))
			copy(cpy, scanner.Bytes())
			select {
			case lines <- scanResult{line: cpy, ok: true}:
			case <-ctx.Done():
				return
			}
		}
		// Signal EOF.
		select {
		case lines <- scanResult{ok: false}:
		case <-ctx.Done():
		}
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil
		case result, chanOpen := <-lines:
			if !chanOpen || !result.ok {
				return scanner.Err()
			}
			if len(strings.TrimSpace(string(result.line))) == 0 {
				continue
			}

			var req Request
			if err := json.Unmarshal(result.line, &req); err != nil {
				if err := s.writeResponse(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "invalid JSON"}}); err != nil {
					return err
				}
				continue
			}
			if rpcErr := validateRequest(req); rpcErr != nil {
				if err := s.writeResponse(Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr}); err != nil {
					return err
				}
				continue
			}

			if req.Method == "tools/call" {
				// Acquire a dispatch-layer slot BEFORE spawning the
				// goroutine so bursty input cannot amplify goroutine
				// count. Context cancellation prevents shutdown deadlock.
				if s.toolCallSem != nil {
					select {
					case s.toolCallSem <- struct{}{}:
					case <-ctx.Done():
						return nil
					}
				}
				wg.Add(1)
				go func(r Request) {
					defer wg.Done()
					if s.toolCallSem != nil {
						defer func() { <-s.toolCallSem }()
					}
					// Panic recovery: a crashing tool handler must not
					// take down the whole stdio loop. Emit a structured
					// panic event and surface a tool-error to the client.
					defer func() {
						if rec := recover(); rec != nil {
							metrics.PanicsRecoveredTotal.Inc("stdio_tool_dispatch")
							stack := string(debug.Stack())
							slog.Error("panic_recovered",
								"site", "stdio_tool_dispatch",
								"tool", toolNameFromRequest(r),
								"panic", fmt.Sprintf("%v", rec),
								"stack", stack,
							)
							panicResp := Response{
								JSONRPC: "2.0",
								ID:      r.ID,
								Result: map[string]any{
									"content": []map[string]any{{
										"type": "text",
										"text": "tool panic: " + fmt.Sprintf("%v", rec),
									}},
									"isError": true,
								},
							}
							if err := s.writeResponse(panicResp); err != nil {
								slog.Warn("async_response_failed", "error", err.Error())
							}
						}
					}()
					resp := s.handle(ctx, r)
					if resp.Error != nil {
						metrics.ProtocolErrorsTotal.Inc(strconv.Itoa(resp.Error.Code))
					}
					if r.ID != nil || resp.Error != nil {
						if err := s.writeResponse(resp); err != nil {
							slog.Warn("async_response_failed", "error", err.Error())
						}
					}
				}(req)
				continue
			}

			resp := s.handle(ctx, req)
			if resp.Error != nil {
				metrics.ProtocolErrorsTotal.Inc(strconv.Itoa(resp.Error.Code))
			}
			if req.ID == nil && resp.Error == nil && resp.Result == nil {
				continue
			}
			if err := s.writeResponse(resp); err != nil {
				return err
			}
		}
	}
}

// handleCancelled processes a notifications/cancelled message by looking
// up the request ID in the inflight map and aborting the corresponding
// tool handler. Malformed payloads and unknown IDs are silently ignored
// per the MCP spec — cancellation is best-effort.
func (s *Server) handleCancelled(raw any) {
	var p struct {
		RequestID any    `json:"requestId"`
		Reason    string `json:"reason,omitempty"`
	}
	if err := decodeParams(raw, &p); err != nil || p.RequestID == nil {
		return
	}
	if !s.cancelInflight(p.RequestID) {
		return
	}
	metrics.Cancellations.Inc("client_requested")
	slog.Info("cancellation",
		"request_id", p.RequestID,
		"reason", p.Reason,
	)
}

// toolNameFromRequest extracts the tool name from a tools/call Request for
// log correlation. Falls back to "unknown" when params are missing or
// malformed — this helper runs in the panic-recovery path so it must not
// allocate on the error path beyond a short string.
func toolNameFromRequest(req Request) string {
	if req.Method != "tools/call" || req.Params == nil {
		return req.Method
	}
	if m, ok := req.Params.(map[string]any); ok {
		if name, ok := m["name"].(string); ok && name != "" {
			return name
		}
	}
	return "unknown"
}

// writeResponse thread-safely encodes a response to the output encoder.
func (s *Server) writeResponse(resp Response) error {
	s.encoderMu.Lock()
	defer s.encoderMu.Unlock()
	if s.encoder == nil {
		return nil
	}
	return s.encoder.Encode(resp)
}

func (s *Server) handle(ctx context.Context, req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize(req.Params)
	case "notifications/initialized":
		return Response{}
	case "notifications/cancelled":
		s.handleCancelled(req.Params)
		return Response{}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.listTools()}
	case "resources/list":
		if result, rpcErr := s.handleResourcesList(ctx); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "resources/read":
		if result, rpcErr := s.handleResourcesRead(ctx, req.Params); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "resources/templates/list":
		if result, rpcErr := s.handleResourcesTemplatesList(ctx); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "resources/subscribe":
		if result, rpcErr := s.handleResourcesSubscribe(req.Params); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "resources/unsubscribe":
		if result, rpcErr := s.handleResourcesUnsubscribe(req.Params); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "prompts/list":
		if result, rpcErr := s.handlePromptsList(); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "prompts/get":
		if result, rpcErr := s.handlePromptsGet(req.Params); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "tools/call":
		// Guard: reject tools/call before initialization (spec compliance)
		if !s.initialized.Load() {
			resp.Error = &RPCError{Code: -32002, Message: "server not initialized: send initialize first"}
			return resp
		}

		var params ToolCallParams
		if err := decodeParams(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}

		// Register a cancellable child context so notifications/cancelled
		// can abort an in-flight tool handler. The cancel func is removed
		// from the inflight map before this case returns regardless of
		// outcome.
		callCtx, cancel := context.WithCancel(ctx)
		s.registerInflight(req.ID, cancel)
		defer s.unregisterInflight(req.ID)
		defer cancel()

		result, err := s.callTool(callCtx, params)
		if err != nil {
			// MCP spec: tool errors return content with isError: true
			resp.Result = map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": err.Error(),
				}},
				"isError": true,
			}
		} else {
			resp.Result = map[string]any{"content": []map[string]any{{"type": "text", "text": mustJSON(result)}}}
		}
	default:
		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return resp
}

// handleInitialize parses the InitializeParams, negotiates the protocol
// version, records client info for log correlation, and returns the server
// capabilities and instructions.
//
// Version negotiation policy: if the client requests a version we support,
// echo it back. Otherwise return our newest supported version — the client
// is expected to either accept the downgrade or disconnect. A previously-
// initialized server accepts a repeat initialize and re-negotiates; the
// current spec does not forbid this and a strict rejection has historically
// broken clients that aggressively reconnect.
func (s *Server) handleInitialize(raw any) map[string]any {
	var params InitializeParams
	_ = decodeParams(raw, &params) // tolerate missing / malformed params

	negotiated := SupportedProtocolVersions[0]
	if requested := strings.TrimSpace(params.ProtocolVersion); requested != "" {
		for _, v := range SupportedProtocolVersions {
			if v == requested {
				negotiated = requested
				break
			}
		}
	}

	// Extract clientInfo name/version for log correlation.
	var clientName, clientVersion string
	if params.ClientInfo != nil {
		if v, ok := params.ClientInfo["name"].(string); ok {
			clientName = v
		}
		if v, ok := params.ClientInfo["version"].(string); ok {
			clientVersion = v
		}
	}

	s.negotiatedMu.Lock()
	s.negotiatedVersion = negotiated
	s.clientName = clientName
	s.clientVersion = clientVersion
	s.negotiatedMu.Unlock()

	s.initialized.Store(true)

	slog.Info("initialize",
		"protocol_version", negotiated,
		"requested_version", params.ProtocolVersion,
		"client_name", clientName,
		"client_version", clientVersion,
	)

	caps := map[string]any{"tools": s.toolCapabilities()}
	if s.ResourceProvider != nil {
		caps["resources"] = map[string]any{"subscribe": true, "listChanged": true}
	}
	caps["prompts"] = map[string]any{"listChanged": true}

	return map[string]any{
		"protocolVersion": negotiated,
		"serverInfo": map[string]any{
			"name":    "clockify-go-mcp",
			"title":   "Clockify Go MCP Server",
			"version": s.Version,
		},
		"capabilities": caps,
		"instructions": ServerInstructions,
	}
}

func (s *Server) toolCapabilities() map[string]any {
	tools := map[string]any{}
	if s.advertiseListChanged.Load() {
		tools["listChanged"] = true
	}
	return tools
}

func (s *Server) listTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.tools))
	for k := range s.tools {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tools := make([]Tool, 0, len(keys))
	for _, key := range keys {
		d := s.tools[key]
		if s.Enforcement != nil && !s.Enforcement.FilterTool(key, ToolHints{
			ReadOnly:    d.ReadOnlyHint,
			Destructive: d.DestructiveHint,
			Idempotent:  d.IdempotentHint,
		}) {
			continue
		}
		tools = append(tools, d.Tool)
	}
	return tools
}

func (s *Server) callTool(ctx context.Context, params ToolCallParams) (any, error) {
	reqID := s.requestSeq.Add(1)
	callStart := time.Now()
	outcome := "success"
	defer func() {
		metrics.ToolCallsTotal.Inc(params.Name, outcome)
		metrics.ToolCallDuration.Observe(time.Since(callStart).Seconds(), params.Name)
	}()

	s.mu.RLock()
	d, ok := s.tools[params.Name]
	s.mu.RUnlock()
	if !ok {
		outcome = "tool_error"
		s.recordAudit(params.Name, "tools/call", outcome, "unknown_tool", params.Arguments, ToolHints{})
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}

	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	hints := ToolHints{
		ReadOnly:    d.ReadOnlyHint,
		Destructive: d.DestructiveHint,
		Idempotent:  d.IdempotentHint,
	}

	// Enforcement: policy gate, rate limit, dry-run intercept
	var release func()
	if s.Enforcement != nil {
		lookup := func(name string) (ToolHandler, bool) {
			s.mu.RLock()
			td, found := s.tools[name]
			s.mu.RUnlock()
			if !found {
				return nil, false
			}
			return td.Handler, true
		}
		result, rel, err := s.Enforcement.BeforeCall(ctx, params.Name, params.Arguments, hints, lookup)
		if rel != nil {
			release = rel
			defer release()
		}
		if err != nil {
			switch {
			case errors.Is(err, ratelimit.ErrRateLimitExceeded), errors.Is(err, ratelimit.ErrConcurrencyLimitExceeded):
				outcome = "rate_limited"
			case strings.Contains(err.Error(), "blocked by policy"):
				outcome = "policy_denied"
			default:
				outcome = "tool_error"
			}
			s.recordAudit(params.Name, "tools/call", outcome, err.Error(), params.Arguments, hints)
			slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "req_id", reqID)
			return nil, err
		}
		if result != nil {
			outcome = "dry_run"
			s.recordAudit(params.Name, "tools/call", outcome, "dry_run_intercepted", params.Arguments, hints)
			slog.Info("tool_call", "tool", params.Name, "intercepted", true, "req_id", reqID)
			return result, nil
		}
	}

	// Dispatch
	start := time.Now()
	timeout := s.ToolTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := d.Handler(callCtx, params.Arguments)
	duration := time.Since(start)

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded):
			outcome = "timeout"
			metrics.Cancellations.Inc("timeout")
		case errors.Is(err, context.Canceled) || errors.Is(callCtx.Err(), context.Canceled):
			outcome = "cancelled"
			metrics.Cancellations.Inc("context_cancelled")
		default:
			outcome = "tool_error"
		}
		s.recordAudit(params.Name, "tools/call", outcome, err.Error(), params.Arguments, hints)
		slog.Warn("tool_call", "tool", params.Name, "error", err.Error(), "duration_ms", duration.Milliseconds(), "req_id", reqID)
		return nil, err
	}
	slog.Info("tool_call", "tool", params.Name, "duration_ms", duration.Milliseconds(), "req_id", reqID)
	if !d.ReadOnlyHint {
		s.recordAudit(params.Name, "tools/call", outcome, "", params.Arguments, hints)
		slog.Info("audit", "tool", params.Name, "destructive", d.DestructiveHint, "req_id", reqID)
	}

	// Post-processing (truncation)
	if s.Enforcement != nil {
		result, _ = s.Enforcement.AfterCall(result)
	}

	return result, nil
}

// InFlightToolCalls reports the current depth of the stdio dispatch
// semaphore. Returns 0 when the semaphore is disabled.
func (s *Server) InFlightToolCalls() int {
	if s.toolCallSem == nil {
		return 0
	}
	return len(s.toolCallSem)
}

// IsReadyCached reports whether the last cached readiness probe
// resulted in success. Scrapers should prefer /ready for fresh
// probes; this method only reads the cached value so /metrics
// does not trigger upstream calls on every scrape.
func (s *Server) IsReadyCached() bool {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.readyCached
}

// ActivateGroup registers a group of tool descriptors dynamically and
// sends a tools/list_changed notification to the client.
func (s *Server) ActivateGroup(groupName string, descriptors []ToolDescriptor) error {
	if s.Activator != nil && !s.Activator.IsGroupAllowed(groupName) {
		return fmt.Errorf("group '%s' is blocked by policy", groupName)
	}
	s.mu.Lock()
	activatedNames := make([]string, 0, len(descriptors))
	for _, d := range descriptors {
		s.tools[d.Tool.Name] = d
		activatedNames = append(activatedNames, d.Tool.Name)
	}
	s.mu.Unlock()
	if s.Activator != nil {
		s.Activator.OnActivate(activatedNames)
	}
	s.notifyToolsChanged()
	slog.Info("group_activated", "group", groupName, "tools_added", len(descriptors))
	return nil
}

// ActivateTier1Tool marks a single registered tool as visible.
func (s *Server) ActivateTier1Tool(name string) error {
	s.mu.Lock()
	if _, exists := s.tools[name]; !exists {
		s.mu.Unlock()
		return fmt.Errorf("unknown tool: %s", name)
	}
	s.mu.Unlock()
	if s.Activator != nil {
		s.Activator.OnActivate([]string{name})
	}
	s.notifyToolsChanged()
	slog.Info("tier1_tool_activated", "tool", name)
	return nil
}

// notifyToolsChanged delivers notifications/tools/list_changed through the
// configured Notifier. If no notifier is installed (e.g. a test harness that
// calls ActivateGroup directly without running a transport), the notification
// is dropped and counted so the gap is visible in /metrics.
func (s *Server) notifyToolsChanged() {
	notifier := s.notifier
	if notifier == nil {
		metrics.ProtocolErrorsTotal.Inc("notification_dropped_no_notifier")
		slog.Warn("notification_dropped",
			"method", "notifications/tools/list_changed",
			"reason", "no_notifier_installed",
		)
		return
	}
	if err := notifier.Notify("notifications/tools/list_changed", map[string]any{}); err != nil {
		slog.Warn("notification_failed",
			"method", "notifications/tools/list_changed",
			"error", err.Error(),
		)
	}
}

func decodeParams(raw any, out any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (s *Server) recordAudit(tool, action, outcome, reason string, args map[string]any, hints ToolHints) {
	if s.Auditor == nil || hints.ReadOnly {
		return
	}
	s.Auditor.RecordAudit(AuditEvent{
		Tool:        tool,
		Action:      action,
		Outcome:     outcome,
		Reason:      reason,
		ResourceIDs: resourceIDs(args),
		Metadata: map[string]string{
			"tenant_id":  s.AuditTenantID,
			"subject":    s.AuditSubject,
			"session_id": s.AuditSessionID,
			"transport":  s.AuditTransport,
		},
	})
}

func resourceIDs(args map[string]any) map[string]string {
	if len(args) == 0 {
		return nil
	}
	ids := map[string]string{}
	for k, v := range args {
		if !strings.HasSuffix(strings.ToLower(k), "_id") {
			continue
		}
		value, ok := v.(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		ids[k] = value
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// validateRequest checks JSON-RPC 2.0 version and id type per spec.
// Returns an RPCError if validation fails, or nil if valid.
func validateRequest(req Request) *RPCError {
	if req.JSONRPC != "2.0" {
		return &RPCError{Code: -32600, Message: "invalid request: jsonrpc must be \"2.0\""}
	}
	if req.ID != nil {
		switch req.ID.(type) {
		case string, float64:
			// valid per JSON-RPC 2.0 spec
		default:
			return &RPCError{Code: -32600, Message: "invalid request: id must be a string or number"}
		}
	}
	return nil
}
