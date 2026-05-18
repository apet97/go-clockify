package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SupportedProtocolVersions lists MCP protocol versions this server can
// negotiate, newest first. The first entry is returned as the default when
// the client does not send a protocolVersion unless an operator configures a
// per-server default. When a client requests an unsupported version, we echo
// back the newest supported version — clients that cannot downgrade will treat
// that as an error and disconnect, which is the spec-compliant behaviour.
var SupportedProtocolVersions = []string{
	"2025-11-25",
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

func IsSupportedProtocolVersion(version string) bool {
	return slices.Contains(SupportedProtocolVersions, strings.TrimSpace(version))
}

func DefaultProtocolVersion(configured string) string {
	configured = strings.TrimSpace(configured)
	if IsSupportedProtocolVersion(configured) {
		return configured
	}
	return SupportedProtocolVersions[0]
}

func negotiateProtocolVersion(requested, configuredDefault string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return DefaultProtocolVersion(configuredDefault)
	}
	if IsSupportedProtocolVersion(requested) {
		return requested
	}
	return SupportedProtocolVersions[0]
}

// ServerInstructions is returned in the initialize response to teach MCP
// clients how to use the one-user/full-access product path.
const ServerInstructions = `This is a single-user full-access Clockify MCP for one pinned workspace.
All tools are loaded at startup.
Use workflow tools first.
Use IDs returned by previous calls.
If a feature is unavailable, report it and continue.`

// Notifier delivers server-initiated notifications (e.g. tools/list_changed)
// to the connected client. The stdio transport implements this by writing
// through the shared JSON encoder.
type Notifier interface {
	Notify(method string, params any) error
}

// notifierCtxKey carries the calling peer's Notifier through the request
// context so resources/subscribe and resources/unsubscribe can record
// per-peer subscription state.
type notifierCtxKey struct{}

// WithNotifier returns a child context carrying n as the calling peer's
// Notifier. The stdio loop calls this once per session after AddNotifier
// so the resources/subscribe handler attributes the subscription
// correctly. Passing a nil Notifier yields the parent context unchanged.
func WithNotifier(ctx context.Context, n Notifier) context.Context {
	if n == nil {
		return ctx
	}
	return context.WithValue(ctx, notifierCtxKey{}, n)
}

// notifierFromContext returns the Notifier wrapped into ctx by WithNotifier,
// or (nil, false) when no notifier is present. Direct-handler tests that
// skip the transport path land in the false branch and the subscribe
// handlers fall back to the broadcast sentinel so legacy test behaviour
// stays intact.
func notifierFromContext(ctx context.Context) (Notifier, bool) {
	if ctx == nil {
		return nil, false
	}
	n, ok := ctx.Value(notifierCtxKey{}).(Notifier)
	return n, ok && n != nil
}

// encoderNotifier adapts the stdio JSON encoder (and its mutex) to the
// Notifier interface so notification delivery does not require the server
// core to hold raw I/O state. Notifications intentionally bypass outChan: the
// shared encoder mutex prevents byte interleaving, but response/notification
// ordering is not guaranteed and does not need to be.
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

type errorTranslator interface {
	error
	ErrorTranslation() any
}

type ToolHandler func(context.Context, map[string]any) (any, error)

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
	// AuditKeys is kept as neutral legacy metadata for typed tool descriptors.
	// The one-user stdio runtime does not wire an audit durability pipeline.
	AuditKeys []string
}

type Server struct {
	Version     string
	ToolTimeout time.Duration // per-call timeout; 0 = default 45s
	// DefaultProtocolVersion is used only when initialize omits
	// params.protocolVersion. Empty or unsupported values fall back to
	// SupportedProtocolVersions[0]; explicit supported client requests still win.
	DefaultProtocolVersion string

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
	// MaxToolResultBytes caps a successful tools/call result before it is
	// placed in the MCP content envelope. 0 disables the cap.
	MaxToolResultBytes int

	mu    sync.RWMutex
	tools map[string]ToolDescriptor
	// toolListCache stores the sorted, filtered tools/list snapshot.
	// Protected by mu and invalidated when descriptors or visibility changes.
	toolListCache      []Tool
	toolListCacheValid bool
	// toolListResultJSON stores the serialized tools/list result only.
	// JSON-RPC ids stay outside this cache and are marshaled per request.
	toolListResultJSON      []byte
	toolListResultJSONValid bool
	initialized             atomic.Bool
	// advertiseListChanged controls whether initialize reports
	// capabilities.tools.listChanged=true. Only transports that can actually
	// deliver notifications/tools/list_changed should set this.
	advertiseListChanged atomic.Bool
	// StaticToolList keeps tools.listChanged out of initialize for products
	// that load their complete registry at startup and never mutate tool
	// availability during a session.
	StaticToolList bool
	encoder        *json.Encoder // stored for push notifications
	encoderMu      sync.Mutex    // protects concurrent encoder writes
	writer         io.Writer     // raw stdio writer for cached JSON-RPC responses
	outChan        chan []byte   // serialized stdout responses, drained by one writer goroutine
	outCtx         context.Context
	outDone        chan struct{}
	outErrMu       sync.Mutex
	outErr         error
	requestSeq     atomic.Int64 // monotonic request ID for log correlation

	hub               notifierHub
	setNotifierRemove func() // cleanup from previous SetNotifier call

	// Negotiated client info. Populated on successful initialize; read by
	// downstream log calls via NegotiatedProtocolVersion() / ClientInfo().
	negotiatedMu      sync.RWMutex
	negotiatedVersion string
	clientName        string
	clientVersion     string

	toolCallSem chan struct{} // dispatch-layer goroutine cap; nil = unlimited

	// inflight tracks cancellable contexts for in-flight tools/call
	// requests, keyed by JSON-RPC request ID. notifications/cancelled
	// looks up the ID and aborts the in-flight tool handler. Nil IDs
	// (notifications) are not tracked.
	inflightMu sync.Mutex
	inflight   map[any]context.CancelCauseFunc
	// activeProgressTokens tracks progress tokens of in-flight tools/call
	// requests so a duplicate token across concurrent requests is rejected.
	// Keyed by the token's canonical string form. Guarded by inflightMu.
	activeProgressTokens map[string]*progressTokenState
}

// AddNotifier registers a notification sink and returns a function that
// removes it. Multiple notifiers can coexist; Notify fans out to all of
// them.
func (s *Server) AddNotifier(n Notifier) func() {
	return s.hub.add(n)
}

// SetNotifier installs a single notification sink, removing any
// previously installed via SetNotifier. The stdio loop uses this for the
// per-session encoder notifier.
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

// ClientInfo returns the client name and version sent during initialize.
func (s *Server) ClientInfo() (name, version string) {
	s.negotiatedMu.RLock()
	defer s.negotiatedMu.RUnlock()
	return s.clientName, s.clientVersion
}

// MarkInitialized seeds a freshly-built Server with the negotiated state
// it would normally obtain from a live `initialize` exchange. Tests and
// embedders use this to skip the handshake; idempotent — calling it
// twice with the same values is a no-op.
//
// protocolVersion may be empty (pre-2025-03-26 client); ClientInfo
// fields are best-effort. The initialized flag is set unconditionally
// because the caller's contract is that the server is ready to dispatch
// tool calls; lacking protocolVersion just means the negotiated-version
// check degrades to "accept any supported version".
func (s *Server) MarkInitialized(protocolVersion, clientName, clientVersion string) {
	s.negotiatedMu.Lock()
	if protocolVersion != "" {
		s.negotiatedVersion = protocolVersion
	}
	if clientName != "" {
		s.clientName = clientName
	}
	if clientVersion != "" {
		s.clientVersion = clientVersion
	}
	s.negotiatedMu.Unlock()
	s.initialized.Store(true)
}

func NewServer(version string, descriptors []ToolDescriptor) *Server {
	toolMap := make(map[string]ToolDescriptor, len(descriptors))
	for _, d := range descriptors {
		toolMap[d.Tool.Name] = d
	}
	s := &Server{
		Version:  version,
		tools:    toolMap,
		inflight: make(map[any]context.CancelCauseFunc),
		prompts:  newPromptRegistry(),
	}
	s.hub.onRemove = s.resourceSubs.dropNotifier
	return s
}

// registerInflight stores a cancel func keyed by JSON-RPC request ID so
// notifications/cancelled can abort the in-flight tool handler. Nil IDs
// (notifications) and zero-value uninitialised maps are no-ops.
func (s *Server) registerInflight(id any, cancel context.CancelCauseFunc) {
	if id == nil {
		return
	}
	key := rpcIDKey(id)
	s.inflightMu.Lock()
	if s.inflight == nil {
		s.inflight = make(map[any]context.CancelCauseFunc)
	}
	s.inflight[key] = cancel
	s.inflightMu.Unlock()
}

// unregisterInflight removes a request from the inflight map. Idempotent.
func (s *Server) unregisterInflight(id any) {
	if id == nil {
		return
	}
	key := rpcIDKey(id)
	s.inflightMu.Lock()
	delete(s.inflight, key)
	s.inflightMu.Unlock()
}

// cancelInflight cancels and removes an inflight request by ID. Returns
// true when a matching request was found.
func (s *Server) cancelInflight(id any) bool {
	if id == nil {
		return false
	}
	key := rpcIDKey(id)
	s.inflightMu.Lock()
	cancel, ok := s.inflight[key]
	if ok {
		delete(s.inflight, key)
	}
	s.inflightMu.Unlock()
	if ok {
		cancel(errExplicitCancellation)
	}
	return ok
}

// errExplicitCancellation is the context cancellation cause set by
// cancelInflight when a client sends notifications/cancelled. handle compares
// context.Cause against this sentinel to decide whether to suppress the
// response per the MCP cancellation spec. Because the cause is bound to the
// individual request's context, a later request that reuses the same JSON-RPC
// id is never affected by an earlier cancellation. The repeat-initialize reset
// path (cancelAllInflight) uses context.Canceled instead, so its late
// responses are still delivered.
var errExplicitCancellation = errors.New("request cancelled via notifications/cancelled")

func rpcIDKey(id any) any {
	switch v := id.(type) {
	case json.Number:
		return v.String()
	case float64:
		if math.Trunc(v) == v {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return id
	}
}

// validateProgressToken enforces the MCP rule that a progress token is a
// string or an integer. Wire values arrive json-decoded with UseNumber, so
// numbers are json.Number; a non-integral number, a boolean, an object, an
// array, or null are all rejected.
func validateProgressToken(token any) error {
	switch v := token.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("progressToken string must not be empty")
		}
		return nil
	case json.Number:
		if _, err := v.Int64(); err != nil {
			return fmt.Errorf("progressToken number must be an integer")
		}
		return nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float64, float32:
		return fmt.Errorf("progressToken number must be an integer, not a float")
	default:
		return fmt.Errorf("progressToken must be a string or integer")
	}
}

func progressTokenKey(token any) string {
	switch v := token.(type) {
	case string:
		return "s:" + v
	case json.Number:
		return "n:" + v.String()
	default:
		return fmt.Sprintf("v:%v", v)
	}
}

// registerProgressToken records token as active. It returns an error when
// the token is already in use by another in-flight request.
func (s *Server) registerProgressToken(token any) error {
	key := progressTokenKey(token)
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	if s.activeProgressTokens == nil {
		s.activeProgressTokens = make(map[string]*progressTokenState)
	}
	if _, dup := s.activeProgressTokens[key]; dup {
		return fmt.Errorf("progressToken is already in use by another in-flight request")
	}
	s.activeProgressTokens[key] = &progressTokenState{}
	return nil
}

func (s *Server) releaseProgressToken(token any) {
	key := progressTokenKey(token)
	s.inflightMu.Lock()
	delete(s.activeProgressTokens, key)
	s.inflightMu.Unlock()
}

// cancelAllInflight cancels and untracks every in-flight request. It is used
// when a client repeats initialize: the session negotiation is being reset, so
// cancelAllInflight is a best-effort initialize/reset boundary: request IDs
// from the previous negotiated session must not remain cancellable. Handlers are
// cancelled but not joined here, so a handler that wins the race to return may
// still emit a late response for its original request ID; clients must match
// responses by ID.
func (s *Server) cancelAllInflight() int {
	s.inflightMu.Lock()
	cancels := make([]context.CancelCauseFunc, 0, len(s.inflight))
	for id, cancel := range s.inflight {
		delete(s.inflight, id)
		cancels = append(cancels, cancel)
	}
	s.inflightMu.Unlock()

	// Use context.Canceled, not errExplicitCancellation: an initialize/reset
	// is not a client cancellation, so these handlers' late responses are
	// still delivered rather than suppressed.
	for _, cancel := range cancels {
		cancel(context.Canceled)
	}
	return len(cancels)
}

// InflightCount returns the number of tracked in-flight tools/call
// requests. Used by tests to verify the map is cleaned up.
func (s *Server) InflightCount() int {
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	return len(s.inflight)
}

// Run processes newline-delimited JSON-RPC requests from r and writes responses
// to w. The reader must reach EOF when the stdio session ends; a reader that
// signals logical end-of-input without EOF can leave the scan goroutine blocked
// in Read. Scanner errors are returned on the EOF path.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer) (retErr error) {
	if s.MaxInFlightToolCalls > 0 && s.toolCallSem == nil {
		s.toolCallSem = make(chan struct{}, s.MaxInFlightToolCalls)
	}
	stdioCtx, cancelStdio := context.WithCancel(ctx)
	defer cancelStdio()

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
	// WARNING: If a client streams a JSON object without newlines exceeding maxMsg,
	// scanner.ErrTooLong will trigger and crash the process cleanly to prevent OOMs.
	scanner.Buffer(buf, maxMsg)
	s.encoderMu.Lock()
	s.encoder = json.NewEncoder(w)
	s.writer = w
	s.encoderMu.Unlock()
	s.outChan = make(chan []byte, 100)
	s.outCtx = stdioCtx
	s.outDone = make(chan struct{})
	s.setOutputErr(nil)
	defer func() {
		if retErr == nil {
			retErr = s.outputErr()
		}
	}()
	var writerWG sync.WaitGroup
	writerWG.Go(func() {
		defer close(s.outDone)
		for b := range s.outChan {
			s.encoderMu.Lock()
			if _, err := s.writer.Write(b); err != nil {
				s.encoderMu.Unlock()
				s.setOutputErr(err)
				cancelStdio()
				slog.Warn("async_response_write_failed", "error", err.Error())
				return
			}
			s.encoderMu.Unlock()
		}
	})
	defer writerWG.Wait()
	defer close(s.outChan)
	// Install the stdio notifier so list/resource change notifications flow
	// back through the same thread-safe encoder the responses use.
	stdioNotifier := &encoderNotifier{mu: &s.encoderMu, encoder: &s.encoder}
	if s.hub.len() == 0 {
		s.SetNotifier(stdioNotifier)
	}
	s.advertiseListChanged.Store(!s.StaticToolList)
	// Thread the stdio peer's Notifier into every dispatched request so
	// resources/subscribe records subscriptions against this peer.
	ctx = WithNotifier(stdioCtx, stdioNotifier)

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
			case <-stdioCtx.Done():
				return
			}
		}
		// Signal EOF.
		select {
		case lines <- scanResult{ok: false}:
		case <-stdioCtx.Done():
		}
	}()

	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-stdioCtx.Done():
			if err := s.outputErr(); err != nil {
				return err
			}
			return nil
		case result, chanOpen := <-lines:
			if !chanOpen || !result.ok {
				return scanner.Err()
			}
			if len(strings.TrimSpace(string(result.line))) == 0 {
				continue
			}

			var req Request
			decoder := json.NewDecoder(bytes.NewReader(result.line))
			decoder.UseNumber()
			if err := decoder.Decode(&req); err != nil {
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
					// take down the whole stdio loop. RecoverDispatch
					// emits a structured panic event and hands the
					// stable JSON-RPC tool-error envelope to the sink
					// for delivery.
					defer RecoverDispatch(r.ID, "stdio_tool_dispatch", toolNameFromRequest(r), func(resp Response) {
						if err := s.writeResponse(resp); err != nil {
							slog.Warn("async_response_failed", "error", err.Error())
						}
					})
					resp := s.handle(ctx, r)
					// A zero-value Response (empty JSONRPC) means handle
					// suppressed the reply for an explicitly cancelled
					// tools/call per the MCP cancellation spec; write nothing.
					if r.ID != nil && resp.JSONRPC != "" {
						if err := s.writeResponse(resp); err != nil {
							slog.Warn("async_response_failed", "error", err.Error())
						}
					}
				}(req)
				continue
			}

			if out, ok, err := s.tryMarshalCachedToolsListResponse(req); ok || err != nil {
				if err != nil {
					return err
				}
				if out != nil {
					if err := s.writeRawResponse(out); err != nil {
						return err
					}
				}
				continue
			}

			if dispatchAsyncRequest(req.Method) && req.ID != nil {
				wg.Add(1)
				go func(r Request) {
					defer wg.Done()
					resp := s.handle(ctx, r)
					if err := s.writeResponse(resp); err != nil {
						slog.Warn("async_response_failed", "error", err.Error())
					}
				}(req)
				continue
			}
			resp := s.handle(ctx, req)
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
// for embedders that own their own concurrency model and framing.
//
// Parse and validation errors are converted to JSON-RPC error responses
// mirroring the stdio loop. A notification (no id, no result, no error)
// returns (nil, nil); the caller must skip sending on the wire in that case.
//
// This method does NOT apply the stdio dispatch-layer toolCallSem. Callers
// that need backpressure on tools/call must implement their own bound.
func (s *Server) DispatchMessage(ctx context.Context, msg []byte) ([]byte, error) {
	var req Request
	// Decode with UseNumber (like Run) so large numeric ids keep precision
	// and numeric tool arguments arrive as json.Number, not float64.
	decoder := json.NewDecoder(bytes.NewReader(msg))
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return json.Marshal(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "invalid JSON"}})
	}
	if rpcErr := validateRequest(req); rpcErr != nil {
		return json.Marshal(Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr})
	}
	if out, ok, err := s.tryMarshalCachedToolsListResponse(req); ok || err != nil {
		return out, err
	}
	resp := s.handle(ctx, req)
	// req.ID == nil is a notification; an empty JSONRPC means handle
	// suppressed the reply for an explicitly cancelled tools/call. Either
	// way there is nothing to send on the wire.
	if req.ID == nil || resp.JSONRPC == "" {
		return nil, nil
	}
	return json.Marshal(resp)
}

// handleCancelled processes a notifications/cancelled message by looking
// up the request ID in the inflight map and aborting the corresponding
// tool handler. Malformed payloads and unknown IDs are silently ignored
// per the MCP spec — cancellation is best-effort.
func (s *Server) handleCancelled(raw any) {
	if m, ok := raw.(map[string]any); ok {
		requestID := m["requestId"]
		if requestID == nil {
			return
		}
		if !s.cancelInflight(requestID) {
			return
		}
		reason, _ := m["reason"].(string)
		slog.Info("cancellation",
			"request_id", requestID,
			"reason", reason,
		)
		return
	}
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

func dispatchAsyncRequest(method string) bool {
	switch method {
	case "resources/list", "resources/read", "resources/templates/list", "prompts/get", "prompts/list":
		return true
	default:
		return false
	}
}

// writeResponse serializes a response and enqueues it for the dedicated stdout writer.
func (s *Server) writeResponse(resp Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return s.enqueueResponseBytes(b)
}

func (s *Server) writeRawResponse(raw []byte) error {
	b := append([]byte{}, raw...)
	b = append(b, '\n')
	return s.enqueueResponseBytes(b)
}

func (s *Server) enqueueResponseBytes(b []byte) error {
	if s.outChan == nil {
		s.encoderMu.Lock()
		defer s.encoderMu.Unlock()
		if s.writer != nil {
			_, err := s.writer.Write(b)
			return err
		}
		return nil
	}
	if err := s.outputErr(); err != nil {
		return err
	}
	select {
	case s.outChan <- b:
		return nil
	case <-s.outDone:
		if err := s.outputErr(); err != nil {
			return err
		}
		return io.ErrClosedPipe
	case <-s.outCtx.Done():
		if err := s.outputErr(); err != nil {
			return err
		}
		return s.outCtx.Err()
	}
}

func (s *Server) setOutputErr(err error) {
	s.outErrMu.Lock()
	defer s.outErrMu.Unlock()
	s.outErr = err
}

func (s *Server) outputErr() error {
	s.outErrMu.Lock()
	defer s.outErrMu.Unlock()
	return s.outErr
}

// HandleWithRecover invokes handle with structured panic recovery.
// Used by embedders whose dispatch goroutines do not own a higher-
// level recovery wrapper — stdio's loop has its own RecoverDispatch
// deferred at the goroutine boundary in Run.
//
// site is the metric/log label that lets operators distinguish where
// a panic originated. Returns the stable JSON-RPC tool-error response
// when a panic was recovered; otherwise the handler's normal Response.
//
// Why a named-return wrapper: recover() only sees the panic when
// called by the deferred function directly; the sink callback then
// writes the panic envelope into the named return variable so the
// caller observes the structured response rather than the zero
// value. RecoverDispatch handles metric increment + slog event.
func (s *Server) HandleWithRecover(ctx context.Context, req Request, site string) (resp Response) {
	defer RecoverDispatch(req.ID, site, toolNameFromRequest(req), func(r Response) {
		resp = r
	})
	resp = s.handle(ctx, req)
	return
}

func (s *Server) handle(ctx context.Context, req Request) Response {
	resp := Response{JSONRPC: "2.0", ID: req.ID}

	if requiresInitialized(req.Method) && !s.initialized.Load() {
		resp.Error = &RPCError{Code: RPCCodeServerNotInitialized, Message: "server not initialized: send initialize first"}
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
		if result, rpcErr := s.handleResourcesSubscribe(ctx, req.Params); rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
	case "resources/unsubscribe":
		if result, rpcErr := s.handleResourcesUnsubscribe(ctx, req.Params); rpcErr != nil {
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
			if err := validateProgressToken(params.Meta.ProgressToken); err != nil {
				resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params: " + err.Error()}
				return resp
			}
			if err := s.registerProgressToken(params.Meta.ProgressToken); err != nil {
				resp.Error = &RPCError{Code: -32602, Message: "invalid tools/call params: " + err.Error()}
				return resp
			}
			defer s.releaseProgressToken(params.Meta.ProgressToken)
			ctx = WithProgressToken(ctx, params.Meta.ProgressToken)
		}
		callCtx, cancel := context.WithCancelCause(ctx)
		s.registerInflight(req.ID, cancel)
		defer s.unregisterInflight(req.ID)
		defer cancel(nil)

		result, err := s.callTool(callCtx, params)
		if err != nil {
			// MCP cancellation spec: when the client explicitly cancelled this
			// request via notifications/cancelled, send no response at all. A
			// zero-value Response signals Run and DispatchMessage to write
			// nothing. context.Cause is read from this request's own context,
			// so a later request that reuses the same JSON-RPC id is
			// unaffected by this cancellation.
			if errors.Is(context.Cause(callCtx), errExplicitCancellation) {
				return Response{}
			}
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
			result := map[string]any{
				"content": []map[string]any{{
					"type": "text",
					"text": err.Error(),
				}},
				"isError": true,
			}
			var translator errorTranslator
			if structured := runtimeCancellationEnvelope(params.Name, err); structured != nil {
				result["structuredContent"] = structured
			} else if errors.As(err, &translator) {
				result["structuredContent"] = map[string]any{"error_translation": translator.ErrorTranslation()}
			}
			resp.Result = result
		} else if guarded, tooLarge := s.applyToolResultSizeGuard(params.Name, result); tooLarge != nil {
			resp.Result = toolResultEnvelope(tooLarge)
		} else {
			resp.Result = toolResultEnvelope(guarded)
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
// echo it back. If the client omits a version, use the operator-configured
// default or our newest supported version. Unsupported requested versions still
// return our newest supported version — the client is expected to either accept
// the downgrade or disconnect. A previously-
// initialized server accepts a repeat initialize and re-negotiates; the
// current spec does not forbid this and a strict rejection has historically
// broken clients that aggressively reconnect. A repeat initialize resets
// session-scoped dispatch state by cancelling outstanding tools/call handlers
// and invalidating the cached tools/list payload. Transport capability flags
// such as listChanged advertisement remain owned by the transport.
func (s *Server) handleInitialize(raw any) map[string]any {
	var params InitializeParams
	_ = decodeParams(raw, &params) // tolerate missing / malformed params

	if s.initialized.Load() {
		cancelled := s.cancelAllInflight()
		s.mu.Lock()
		s.invalidateToolListCacheLocked()
		s.mu.Unlock()
		slog.Info("repeat_initialize_reset",
			"cancelled_inflight", cancelled,
		)
	}

	negotiated := negotiateProtocolVersion(params.ProtocolVersion, s.DefaultProtocolVersion)

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
	caps["prompts"] = map[string]any{}

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
//   - A JSON null arguments or _meta value is treated as absent (the MCP
//     spec says null arguments behave identically to no arguments),
//     matching json.Unmarshal which leaves the destination at its zero value.
//   - Extra keys in m are ignored, matching json.Unmarshal's default.
//   - Progress token type is validated by validateProgressToken in handle; a
//     present null token is rejected here.
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
	if rawArgs, ok := m["arguments"]; ok && rawArgs != nil {
		args, ok := rawArgs.(map[string]any)
		if !ok {
			return p, fmt.Errorf("arguments must be an object")
		}
		p.Arguments = args
	}
	if rawMeta, ok := m["_meta"]; ok && rawMeta != nil {
		meta, ok := rawMeta.(map[string]any)
		if !ok {
			return p, fmt.Errorf("_meta must be an object")
		}
		p.Meta = &RequestMeta{}
		if tok, ok := meta["progressToken"]; ok {
			if tok == nil {
				return p, fmt.Errorf("_meta.progressToken must not be null")
			}
			p.Meta.ProgressToken = tok
		}
	}
	return p, nil
}

func runtimeCancellationEnvelope(toolName string, err error) map[string]any {
	code := ""
	hint := ""
	retryable := true
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
		hint = "The tool exceeded CLOCKIFY_TOOL_TIMEOUT before Clockify returned. Retry with a narrower request, check Clockify availability, or increase CLOCKIFY_TOOL_TIMEOUT for legitimately long operations."
	case errors.Is(err, context.Canceled):
		code = "cancelled"
		hint = "The tool was cancelled before completion. Retry when the client is ready to keep the request open."
	default:
		return nil
	}
	return map[string]any{
		"ok":     false,
		"action": toolName,
		"error": map[string]any{
			"code":    code,
			"message": err.Error(),
		},
		"recovery": map[string]any{
			"hint":      hint,
			"retryable": retryable,
		},
	}
}

// toolResultEnvelope builds the successful MCP tools/call envelope.
// Text content preserves the wire contract for clients that still read
// content[0].text. structuredContent reuses the same marshalled bytes via
// json.RawMessage so object-shaped tool results are not encoded once for the
// text block and again for the structured field.
func toolResultEnvelope(v any) map[string]any {
	text, structured := marshalToolResult(v)
	out := map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
	if structured != nil {
		out["structuredContent"] = structured
	}
	return out
}

// marshalToolResult serialises a tool's return value into the string payload
// of the MCP content envelope. Previously used json.MarshalIndent with a
// two-space indent; the pretty-printing cost every successful tools/call about
// 20% of its wall-clock time and doubled the allocated bytes for no observable
// benefit. The output is transported inside a JSON string field, so clients
// decode it uniformly regardless of whitespace.
func marshalToolResult(v any) (string, json.RawMessage) {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error()), nil
	}
	if jsonObjectRaw(b) {
		return string(b), json.RawMessage(b)
	}
	return string(b), nil
}

// jsonObjectRaw reports whether raw JSON may be used as MCP
// structuredContent. The spec restricts structuredContent to a JSON object,
// so arrays, scalars, and null keep text-only output.
func jsonObjectRaw(raw []byte) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\n', '\r', '\t':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// maxProgressNotificationsPerSecond bounds notifications/progress for a single
// token so a misbehaving handler cannot flood the client.
const maxProgressNotificationsPerSecond = 10

// progressTokenState tracks per-token progress so the server can enforce the
// MCP rule that progress values strictly increase, and rate-limit bursts.
type progressTokenState struct {
	lastProgress float64
	hasProgress  bool
	windowStart  time.Time
	windowCount  int
}

// ProgressGate decides whether a notifications/progress for a token at a given
// progress value may be sent. *Server implements it; Service.EmitProgress
// consults it so non-increasing or flooding progress is dropped at the source.
type ProgressGate interface {
	AllowProgress(token any, progress float64) bool
}

// AllowProgress reports whether a progress notification for token at the given
// progress value should be emitted. It returns false when the token is not
// registered (the call finished or was cancelled), when progress does not
// strictly increase, or when the per-second flood cap is exceeded.
func (s *Server) AllowProgress(token any, progress float64) bool {
	key := progressTokenKey(token)
	s.inflightMu.Lock()
	defer s.inflightMu.Unlock()
	st, ok := s.activeProgressTokens[key]
	if !ok {
		return false
	}
	if st.hasProgress && progress <= st.lastProgress {
		return false
	}
	now := time.Now()
	if st.windowStart.IsZero() || now.Sub(st.windowStart) >= time.Second {
		st.windowStart = now
		st.windowCount = 0
	}
	if st.windowCount >= maxProgressNotificationsPerSecond {
		return false
	}
	st.windowCount++
	st.lastProgress = progress
	st.hasProgress = true
	return true
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
		case string, float64, json.Number:
			// valid per JSON-RPC 2.0 spec
		default:
			return &RPCError{Code: -32600, Message: "invalid request: id must be a string or number"}
		}
	}
	return nil
}
