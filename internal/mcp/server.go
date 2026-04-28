package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/metrics"
)

// SupportedProtocolVersions lists MCP protocol versions this server can
// negotiate, newest first. The first entry is returned as the default when
// the client does not send a protocolVersion. When a client requests an
// unsupported version, we echo back the newest supported version — clients
// that cannot downgrade will treat that as an error and disconnect, which is
// the spec-compliant behaviour.
var SupportedProtocolVersions = []string{
	"2025-11-25",
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
  - Activate Tier 2 tools with 'clockify_search_tools' using 'activate_group' (preferred) or 'activate_tool' before calling them. Each Tier-2 group is the unit of activation: passing a single tool name via 'activate_tool' brings the entire containing group online, and the response enumerates every newly-available tool name.

Safety:
  - The server supports five policy modes: read_only, time_tracking_safe, safe_core, standard (default), full.
  - time_tracking_safe is the recommended AI-facing default (used by the shared-service and prod-postgres profiles).
  - Destructive tools support dry-run previews when you pass dry_run:true.
  - Omit dry_run or pass dry_run:false to execute the mutation.
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

// sanitizable is implemented by errors that carry both a verbose form
// (Error()) and a sanitised form (Sanitized()) — typically because the
// verbose form embeds an upstream HTTP response body that should not
// cross tenant boundaries on a hosted deployment. clockify.APIError is
// the in-tree implementer; the interface is duck-typed so this package
// stays free of a clockify import.
type sanitizable interface {
	error
	Sanitized() string
}

// sanitizeClientError walks the error chain looking for any wrapped
// error that exposes a Sanitized() form, returning that form as the
// MCP client-facing message. Errors with no sanitised form fall back
// to err.Error() unchanged: those are typically schema/validation /
// transport-level errors with no embedded upstream payload.
func sanitizeClientError(err error) string {
	var s sanitizable
	if errors.As(err, &s) {
		return s.Sanitized()
	}
	return err.Error()
}

type ToolHandler func(context.Context, map[string]any) (any, error)

// RiskClass categorises tools beyond the three MCP boolean hints. It is a
// bitmask so a tool can carry multiple attributes (for example a billing
// action that also triggers an external side effect). Consumers — audit,
// policy, enforcement — pattern-match against bits to decide confirmation
// requirements, log fidelity, and policy gates. The taxonomy mirrors
// docs/policy/production-tool-scope.md.
type RiskClass uint32

const (
	RiskRead               RiskClass = 1 << iota // safe, idempotent reads
	RiskWrite                                    // ordinary mutating writes
	RiskBilling                                  // touches invoices / payments
	RiskAdmin                                    // workspace-admin scope (deactivate, group/user mgmt)
	RiskPermissionChange                         // role / permission changes
	RiskExternalSideEffect                       // triggers outbound delivery (email, webhook test)
	RiskDestructive                              // irreversible delete-style operations
)

// Has reports whether the receiver carries every bit in mask.
func (r RiskClass) Has(mask RiskClass) bool { return r&mask == mask }

type ToolDescriptor struct {
	Tool            Tool
	Handler         ToolHandler
	ReadOnlyHint    bool
	DestructiveHint bool
	IdempotentHint  bool
	// RiskClass is the structured risk taxonomy; defaults are derived from
	// the boolean hints in normalizeDescriptors when this field is zero.
	// Tier-2 tools that need finer granularity than read/write/destructive
	// (billing, admin, permission_change, external_side_effect) override it
	// in internal/tools/risk_overrides.go.
	RiskClass RiskClass
	// AuditKeys lists argument keys (beyond the implicit *_id suffix scan)
	// whose values must be captured in audit events. Used for action-defining
	// fields like role, status, quantity, unit_price.
	AuditKeys []string
}

type Server struct {
	Version      string
	Enforcement  Enforcement                     // nil = no filtering or enforcement
	Activator    Activator                       // nil = activation unrestricted
	ToolTimeout  time.Duration                   // per-call timeout; 0 = default 45s
	ReadyChecker func(ctx context.Context) error // optional upstream health check for /ready
	// ExposeAuthErrors controls whether HTTP transports return detailed
	// authenticator errors to unauthenticated clients. The default is false:
	// transports return a generic OAuth error_description and log details
	// server-side only.
	ExposeAuthErrors bool

	// SanitizeUpstreamErrors controls whether tool-error responses to MCP
	// clients omit upstream Clockify response bodies. The default is false
	// (verbose, useful for local development); hosted profiles
	// (shared-service, prod-postgres) set it to true so a 4xx from
	// Clockify can't leak per-tenant info across tenant boundaries.
	// The full APIError is always logged server-side regardless.
	SanitizeUpstreamErrors bool

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
	MaxMessageSize       int64

	// StrictHostCheck enables DNS rebinding protection on the HTTP
	// transport: the inbound Host header must match either a loopback
	// literal or one of the configured allowed-origin hostnames. Defaults
	// to off so that reverse-proxy deployments that rewrite Host are
	// unaffected; flip on (MCP_STRICT_HOST_CHECK=1) in localhost-bound
	// production deployments to get the strict guarantee.
	StrictHostCheck bool

	// ExtraHTTPHandlers carries optional handlers that the legacy HTTP
	// transport mounts on its mux before ListenAndServe. Used by the
	// -tags=pprof build in cmd/clockify-mcp/ to attach /debug/pprof/*
	// without forcing internal/mcp to depend on net/http/pprof. nil or
	// empty = no extras registered, which is the default production path.
	ExtraHTTPHandlers []ExtraHandler

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

	hub               notifierHub
	setNotifierRemove func() // cleanup from previous SetNotifier call

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

	// AuditDurabilityMode controls what happens when audit persistence fails
	// for a non-read-only successful tool call.
	//
	//   "best_effort" (default): log the error and increment the failure metric;
	//   do not fail the tool call. The mutation already happened; the operator
	//   is alerted but the client sees success.
	//
	//   "fail_closed": return an error to the caller so the client knows the
	//   mutation's audit trail is incomplete. The mutation still happened, but
	//   reporting success when the audit write failed is suppressed.
	//   Read-only operations are never affected regardless of this setting.
	AuditDurabilityMode string

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

// AddNotifier registers a notification sink and returns a function that
// removes it. Multiple notifiers can coexist; Notify fans out to all of
// them. Transports that multiplex clients (gRPC Exchange streams) should
// call AddNotifier per-stream and defer the returned remove function.
func (s *Server) AddNotifier(n Notifier) func() {
	return s.hub.add(n)
}

// SetNotifier installs a notification sink, removing any previously
// installed via SetNotifier. Transports that own a single client (stdio,
// legacy HTTP) use this for backwards compatibility. Internally delegates
// to AddNotifier.
func (s *Server) SetNotifier(n Notifier) {
	if s.setNotifierRemove != nil {
		s.setNotifierRemove()
	}
	s.setNotifierRemove = s.hub.add(n)
}

// Notify forwards a server-initiated notification through all registered
// notifiers. Returns nil when no notifiers are installed.
func (s *Server) Notify(method string, params any) error {
	return s.hub.notify(method, params)
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
	maxMsg := int(s.MaxMessageSize)
	if maxMsg <= 0 {
		maxMsg = 4194304
	}
	// Size the initial buffer to 64 KiB or maxMsg, whichever is smaller.
	// Passing a larger initial buffer than maxMsg silently defeats the
	// limit because bufio.Scanner only consults max when it needs to grow
	// beyond the initial buffer. Before this, a 64 KiB initial capacity
	// plus maxMsg = 4 KiB meant the scanner happily consumed 64 KiB lines.
	initial := min(64*1024, maxMsg)
	buf := make([]byte, 0, initial)
	scanner.Buffer(buf, maxMsg)
	s.encoderMu.Lock()
	s.encoder = json.NewEncoder(w)
	s.encoderMu.Unlock()
	// Install the stdio notifier so activation events (tools/list_changed)
	// flow back through the same thread-safe encoder the responses use.
	if s.hub.len() == 0 {
		s.SetNotifier(encoderNotifier{mu: &s.encoderMu, encoder: &s.encoder})
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
							// Generic message — full panic value and stack are
							// already in the slog event above. Returning the
							// raw recovered value to the client risks leaking
							// internal state, request data, or upstream error
							// strings; the client gets a stable identifier
							// instead.
							panicResp := Response{
								JSONRPC: "2.0",
								ID:      r.ID,
								Result: map[string]any{
									"content": []map[string]any{{
										"type": "text",
										"text": "internal tool error; request logged",
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
					if r.ID != nil {
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
			if req.ID == nil {
				continue
			}
			if err := s.writeResponse(resp); err != nil {
				return err
			}
		}
	}
}

// DispatchMessage parses a single JSON-RPC message from raw bytes, invokes
// the central handler, and returns the serialized response. It is intended
// for non-stdio transports (gRPC sub-module, custom bridges) that own their
// own concurrency model and framing.
//
// Parse and validation errors are converted to JSON-RPC error responses
// mirroring the stdio loop. A notification (no id, no result, no error)
// returns (nil, nil); the caller must skip sending on the wire in that case.
//
// This method does NOT apply the stdio dispatch-layer toolCallSem. Callers
// that need backpressure on tools/call must implement their own bound.
func (s *Server) DispatchMessage(ctx context.Context, msg []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		metrics.ProtocolErrorsTotal.Inc("-32700")
		return json.Marshal(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "invalid JSON"}})
	}
	if rpcErr := validateRequest(req); rpcErr != nil {
		metrics.ProtocolErrorsTotal.Inc(strconv.Itoa(rpcErr.Code))
		return json.Marshal(Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr})
	}
	resp := s.handle(ctx, req)
	if resp.Error != nil {
		metrics.ProtocolErrorsTotal.Inc(strconv.Itoa(resp.Error.Code))
	}
	if req.ID == nil {
		return nil, nil
	}
	return json.Marshal(resp)
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

	if requiresInitialized(req.Method) && !s.initialized.Load() {
		resp.Error = &RPCError{Code: -32002, Message: "server not initialized: send initialize first"}
		return resp
	}

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
		var params ToolCallParams
		// Fast path: when req.Params arrived via top-level JSON decode it
		// is already map[string]any, so we skip the json.Marshal →
		// json.Unmarshal roundtrip in decodeParams and walk the map via
		// type assertions. The roundtrip was ~78 allocs/op worth of
		// garbage on every tools/call (measured via BenchmarkDispatchToolsCall).
		// Falls back to decodeParams for any non-map shape (e.g. an RPC
		// client that wraps params in a json.RawMessage) so malformed
		// payloads still fail with the same -32602 error.
		if m, ok := req.Params.(map[string]any); ok {
			var err error
			params, err = toolCallParamsFromMap(m)
			if err != nil {
				resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params: " + err.Error()}
				return resp
			}
		} else if err := decodeParams(req.Params, &params); err != nil {
			resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params"}
			return resp
		}
		if strings.TrimSpace(params.Name) == "" {
			resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params: name must be a non-empty string"}
			return resp
		}

		// Register a cancellable child context so notifications/cancelled
		// can abort an in-flight tool handler. The cancel func is removed
		// from the inflight map before this case returns regardless of
		// outcome.
		if params.Meta != nil && params.Meta.ProgressToken != nil {
			ctx = WithProgressToken(ctx, params.Meta.ProgressToken)
		}
		callCtx, cancel := context.WithCancel(ctx)
		s.registerInflight(req.ID, cancel)
		defer s.unregisterInflight(req.ID)
		defer cancel()

		result, err := s.callTool(callCtx, params)
		if err != nil {
			// W2-01: schema-validation failures are protocol-level errors
			// (JSON-RPC -32602), not tool-errors. The JSON Pointer to the
			// failing field goes in error.data.pointer so clients can
			// locate the offender without string parsing.
			var ipe *InvalidParamsError
			var ute *UnknownToolError
			if errors.As(err, &ipe) {
				data := map[string]any{}
				if ipe.Pointer != "" {
					data["pointer"] = ipe.Pointer
				}
				resp.Error = &RPCError{
					Code:    -32602,
					Message: ipe.Error(),
					Data:    data,
				}
				return resp
			}
			if errors.As(err, &ute) {
				resp.Error = &RPCError{Code: -32602, Message: ute.Error()}
				return resp
			}
			// MCP spec: tool errors return content with isError: true.
			// Sanitisation only affects the client-facing message; the
			// full err.Error() is preserved in the slog tool_call
			// records emitted from callTool.
			text := err.Error()
			if s.SanitizeUpstreamErrors {
				text = sanitizeClientError(err)
			}
			resp.Result = map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": text,
				}},
				"isError": true,
			}
		} else {
			// Dual-emit per MCP 2025-06-18: text content preserves the wire
			// contract for clients that still read content[0].text, and
			// structuredContent surfaces the typed payload for clients that
			// validate against the advertised outputSchema. structuredContent
			// is only attached when the result marshals to a JSON object (the
			// spec forbids arrays/scalars there); tools whose result is a
			// slice or nil keep text-only output.
			out := map[string]any{"content": []map[string]any{{"type": "text", "text": mustJSON(result)}}}
			if structured, okStruct := structuredContentValue(result); okStruct {
				out["structuredContent"] = structured
			}
			resp.Result = out
		}
	default:
		resp.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return resp
}

func requiresInitialized(method string) bool {
	switch method {
	case "initialize", "notifications/initialized", "notifications/cancelled", "ping":
		return false
	default:
		return true
	}
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
	if requested := strings.TrimSpace(params.ProtocolVersion); requested != "" && slices.Contains(SupportedProtocolVersions, requested) {
		negotiated = requested
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

// IsReadyCached reports whether the last cached readiness probe
// resulted in success. Scrapers should prefer /ready for fresh
// probes; this method only reads the cached value so /metrics
// does not trigger upstream calls on every scrape.
func (s *Server) IsReadyCached() bool {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()
	return s.readyCached
}

// SetReadyCached updates the cached readiness state. Transports that
// lack an HTTP readiness endpoint (gRPC) call this after verifying
// upstream connectivity so IsReadyCached reflects their state.
func (s *Server) SetReadyCached(ready bool) {
	s.readyMu.Lock()
	s.readyCached = ready
	s.readyMu.Unlock()
}

func decodeParams(raw any, out any) error {
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// toolCallParamsFromMap decodes a tools/call parameter map into
// ToolCallParams without going through a json.Marshal → json.Unmarshal
// roundtrip. Only the tools/call hot path uses this helper — cold-path
// decoders (prompts/get, resources/read, …) still call decodeParams.
//
// Behaviour relative to json.Unmarshal:
//   - Wrong-type name / arguments / _meta fields are rejected instead of
//     being silently zeroed so malformed tools/call requests surface as
//     JSON-RPC -32602 invalid params.
//   - Extra keys in m are ignored, matching json.Unmarshal's default.
//   - Progress token is taken verbatim so clients supplying a string
//     or number both round-trip untouched.
//
// See FuzzToolCallParamsFromMap for the equivalence guard against
// json.Unmarshal on random maps.
func toolCallParamsFromMap(m map[string]any) (ToolCallParams, error) {
	var p ToolCallParams
	rawName, ok := m["name"]
	if !ok {
		return p, fmt.Errorf("name must be a non-empty string")
	}
	name, ok := rawName.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return p, fmt.Errorf("name must be a non-empty string")
	}
	p.Name = name
	if rawArgs, ok := m["arguments"]; ok {
		args, ok := rawArgs.(map[string]any)
		if !ok {
			return p, fmt.Errorf("arguments must be an object")
		}
		p.Arguments = args
	}
	if rawMeta, ok := m["_meta"]; ok {
		meta, ok := rawMeta.(map[string]any)
		if !ok {
			return p, fmt.Errorf("_meta must be an object")
		}
		p.Meta = &RequestMeta{}
		if tok, ok := meta["progressToken"]; ok {
			p.Meta.ProgressToken = tok
		}
	}
	return p, nil
}

// mustJSON serialises a tool's return value into the string payload of
// the MCP content envelope. Previously used json.MarshalIndent with a
// two-space indent; the pretty-printing cost every successful
// tools/call about 20% of its wall-clock time and doubled the allocated
// bytes for no observable benefit — the output is transported inside a
// JSON string field, so clients decode it uniformly regardless of
// whitespace. Switched to json.Marshal; the MCP wire format is unchanged.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// structuredContentValue reports whether v is safe to place in the MCP
// structuredContent field. The spec restricts structuredContent to a JSON
// object, so slices, scalars, nil, and non-string-keyed maps are rejected.
// ResultEnvelope and map[string]any — the two shapes tool handlers actually
// return today — both pass.
func structuredContentValue(v any) (any, bool) {
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	switch rv.Kind() {
	case reflect.Struct:
		return v, true
	case reflect.Map:
		if rv.Type().Key().Kind() == reflect.String {
			return v, true
		}
	}
	return nil, false
}

// validateRequest checks JSON-RPC 2.0 version and id type per spec.
// Returns an RPCError if validation fails, or nil if valid.
func validateRequest(req Request) *RPCError {
	if req.JSONRPC != "2.0" {
		return &RPCError{Code: -32600, Message: "invalid request: jsonrpc must be \"2.0\""}
	}
	if strings.TrimSpace(req.Method) == "" {
		return &RPCError{Code: -32600, Message: "invalid request: method must be a non-empty string"}
	}
	if req.Method == "initialize" && req.ID == nil {
		return &RPCError{Code: -32600, Message: "invalid request: initialize must include an id"}
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
