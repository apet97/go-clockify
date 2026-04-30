package mcp

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/metrics"
)

type StreamableSessionRuntime struct {
	Server          *Server
	Close           func()
	TenantID        string
	WorkspaceID     string
	ClockifyBaseURL string
}

type StreamableSessionFactory func(context.Context, authn.Principal, string) (*StreamableSessionRuntime, error)

const (
	MCPSessionIDHeader       = "MCP-Session-Id"
	LegacyMCPSessionIDHeader = "X-MCP-Session-ID"
)

type StreamableHTTPOptions struct {
	Version string
	Bind    string
	// Listener, if non-nil, is used in place of net.Listen("tcp", Bind).
	// Primarily for tests that need to know the bound port before the
	// server starts accepting. Bind is still consulted for the logged
	// startup address when Listener is nil.
	Listener        net.Listener
	MaxBodySize     int64
	AllowedOrigins  []string
	AllowAnyOrigin  bool
	StrictHostCheck bool
	// ExposeAuthErrors controls whether unauthenticated clients receive
	// detailed authenticator failure reasons. Default false returns a
	// generic OAuth error_description and logs details server-side only.
	ExposeAuthErrors bool
	// SanitizeUpstreamErrors controls whether tool errors returned to
	// MCP clients omit upstream Clockify response bodies. Default false
	// (verbose, useful for local development); hosted profiles flip it
	// on so upstream payloads cannot cross tenant boundaries.
	SanitizeUpstreamErrors bool
	SessionTTL             time.Duration
	ReadyChecker           func(context.Context) error
	Authenticator          authn.Authenticator
	ControlPlane           controlplane.Store
	Factory                StreamableSessionFactory
	// ProtectedResource is the unauthenticated handler for the
	// /.well-known/oauth-protected-resource metadata document. When
	// non-nil it is mounted at the canonical RFC 9728 path. nil =
	// endpoint omitted (e.g. server does not advertise OAuth 2.1
	// resource discovery).
	ProtectedResource http.Handler
	// ExtraHandlers mounts optional handlers on the streamable HTTP
	// mux before ListenAndServe — counterpart to Server.ExtraHTTPHandlers
	// for the streamable transport. Used by -tags=pprof to attach
	// /debug/pprof/* alongside /mcp. nil = no extras, default path.
	ExtraHandlers []ExtraHandler
	// IdleGraceAfterDisconnect is the maximum time a session with zero
	// active SSE subscribers may sit before the reaper evicts it early.
	// Guards against orphaned-subscriber leaks where a client drops TCP
	// mid-stream without DELETEing the session: SessionTTL alone would
	// hold the entry for up to 30 minutes. Zero uses the 5 minute default.
	IdleGraceAfterDisconnect time.Duration
	// TLSConfig, when non-nil, wraps the bound listener with tls.NewListener
	// so the streamable HTTP transport terminates TLS in-process. Required
	// when the operator selects MCP_AUTH_MODE=mtls on streamable_http —
	// without TLS, the mTLS authenticator has no VerifiedChains to read
	// from r.TLS and every request fails with "verified mTLS client
	// certificate required". When nil, the listener serves plain HTTP
	// (the long-standing default for this transport).
	TLSConfig *tls.Config
	// BehindHTTPSProxy, when true, lets the baseline-header middleware
	// emit Strict-Transport-Security on plaintext responses because a
	// trusted upstream proxy is terminating TLS for us. Without TLS in
	// front of the listener and without this flag, HSTS is skipped to
	// avoid making honest http:// URLs unreachable on misconfigured
	// dev installs. Wired from MCP_BEHIND_HTTPS_PROXY=1.
	BehindHTTPSProxy bool
}

type streamSession struct {
	id         string
	principal  authn.Principal
	server     *Server
	runtime    *StreamableSessionRuntime
	events     *sessionEventHub
	createdAt  time.Time
	expiresAt  time.Time
	lastSeenAt time.Time
}

func ServeStreamableHTTP(ctx context.Context, opts StreamableHTTPOptions) error {
	if opts.Listener == nil && opts.Bind == "" {
		opts.Bind = ":8080"
	}
	if opts.MaxBodySize <= 0 {
		opts.MaxBodySize = 2097152
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = 30 * time.Minute
	}
	if opts.Authenticator == nil {
		return fmt.Errorf("streamable_http requires an authenticator")
	}
	if opts.ControlPlane == nil {
		return fmt.Errorf("streamable_http requires a control-plane store")
	}
	if opts.Factory == nil {
		return fmt.Errorf("streamable_http requires a session factory")
	}

	grace := opts.IdleGraceAfterDisconnect
	if grace <= 0 {
		grace = defaultIdleGraceAfterDisconnect
	}
	mgr := &streamSessionManager{
		ttl:                      opts.SessionTTL,
		idleGraceAfterDisconnect: grace,
		store:                    opts.ControlPlane,
		items:                    map[string]*streamSession{},
	}
	go mgr.reapLoop(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", observeHTTP("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": opts.Version})
	}))
	mux.HandleFunc("GET /ready", observeHTTP("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if opts.ReadyChecker != nil {
			checkCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := opts.ReadyChecker(checkCtx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "reason": err.Error()})
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	mux.Handle("POST /mcp", observeHTTPH("/mcp", streamableRPCHandler(opts, mgr)))
	mux.Handle("OPTIONS /mcp", observeHTTPH("/mcp", streamableRPCHandler(opts, mgr)))
	// GET /mcp is the spec-canonical SSE stream for server→client
	// notifications (MCP Streamable HTTP 2025-03-26 §3.3). GET /mcp/events
	// is the legacy alias from the pre-1.0 transport shape; it now
	// stays mounted indefinitely — under ADR-0012 removing an
	// operator-facing route is a major-version bump, and clients
	// pinned to the old path during the v0.x line still rely on it.
	mux.Handle("GET /mcp", observeHTTPH("/mcp", streamableEventsHandler(opts, mgr)))
	mux.Handle("GET /mcp/events", observeHTTPH("/mcp/events", streamableEventsHandler(opts, mgr)))
	if opts.ProtectedResource != nil {
		mux.Handle("/.well-known/oauth-protected-resource",
			observeHTTPH("/.well-known/oauth-protected-resource", opts.ProtectedResource))
	}
	// Mount opt-in extras (e.g. /debug/pprof/* under -tags=pprof). nil slice
	// is a no-op so default builds are byte-identical.
	mountExtras(mux, opts.ExtraHandlers)

	srv := &http.Server{
		Addr:              opts.Bind,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		mgr.closeAll()
	}()
	ln := opts.Listener
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", opts.Bind)
		if err != nil {
			return fmt.Errorf("listen streamable_http: %w", err)
		}
	}
	// Native TLS termination on the streamable HTTP transport. When
	// TLSConfig is non-nil the listener is wrapped so the Go HTTP
	// server completes the handshake itself; r.TLS.VerifiedChains is
	// then populated for downstream mtls authentication. Plain HTTP
	// remains the default — TLSConfig nil keeps the listener bare.
	if opts.TLSConfig != nil {
		ln = tls.NewListener(ln, opts.TLSConfig)
	}
	slog.Info("streamable_http_start",
		"bind", ln.Addr().String(),
		"session_ttl", opts.SessionTTL.String(),
		"tls", opts.TLSConfig != nil,
	)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// applyOriginPolicy enforces the shared origin/CORS contract for every
// streamable-HTTP route. POST /mcp and the SSE GET paths must answer the
// same Origin checks so a browser client cannot bypass CORS by subscribing
// to the event stream. Returns false when the request has already been
// rejected and the caller must abort.
func applyOriginPolicy(w http.ResponseWriter, r *http.Request, opts StreamableHTTPOptions) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if !isOriginAllowed(origin, opts.AllowedOrigins, opts.AllowAnyOrigin) {
		writeJSONError(w, http.StatusForbidden, "origin not allowed")
		return false
	}
	if opts.AllowAnyOrigin {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	return true
}

func streamableRPCHandler(opts StreamableHTTPOptions, mgr *streamSessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyHTTPBaselineHeaders(w, r, opts.BehindHTTPSProxy)
		if opts.StrictHostCheck && !isHostAllowed(r.Host, opts.AllowedOrigins, opts.AllowAnyOrigin) {
			writeJSONError(w, http.StatusForbidden, "host not allowed")
			return
		}
		if !applyOriginPolicy(w, r, opts) {
			return
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Session-Id, X-MCP-Session-ID, MCP-Protocol-Version")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		principal, err := opts.Authenticator.Authenticate(r.Context(), r)
		if err != nil {
			logHTTPAuthFailure("streamable_http", r, err)
			writeAuthFailure(w, err, opts.ExposeAuthErrors)
			return
		}
		r = r.WithContext(authn.WithPrincipal(r.Context(), &principal))
		r.Body = http.MaxBytesReader(w, r.Body, opts.MaxBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}
		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32700, Message: "invalid JSON"}})
			return
		}
		if rpcErr := validateRequest(req); rpcErr != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Response{JSONRPC: "2.0", ID: req.ID, Error: rpcErr})
			return
		}
		var session *streamSession
		if req.Method == "initialize" {
			id, err := randomID()
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "session id generation failed")
				return
			}
			session, err = mgr.create(r.Context(), id, principal, opts)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
		} else {
			sessionID := sessionIDFromRequest(r)
			if sessionID == "" {
				writeJSONError(w, http.StatusBadRequest, "missing session id")
				return
			}
			session, err = mgr.get(sessionID)
			if err != nil {
				writeJSONError(w, http.StatusNotFound, "invalid session")
				return
			}
			if principal.Subject != session.principal.Subject || principal.TenantID != session.principal.TenantID {
				writeJSONError(w, http.StatusForbidden, "session principal mismatch")
				return
			}
			if vErr := validateProtocolVersion(r, session); vErr != nil {
				metrics.ProtocolErrorsTotal.Inc("protocol_version_mismatch")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(Response{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32600, Message: vErr.Error()}})
				return
			}
		}
		if req.Method != "initialize" {
			mgr.touch(session.id)
		}
		// HandleWithRecover wraps handle with structured panic
		// recovery so a crashing tool handler returns a stable
		// JSON-RPC tool-error envelope instead of taking the
		// connection down at the http.Server boundary. Same shape
		// emitted by stdio + gRPC for cross-transport parity.
		resp := session.server.HandleWithRecover(r.Context(), req, "streamable_http_dispatch")
		if req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if req.Method == "initialize" {
			if err := mgr.syncInitializeState(session); err != nil {
				mgr.destroy(session.id, session)
				writeJSONError(w, http.StatusInternalServerError, "session persistence failed")
				return
			}
			w.Header().Set(MCPSessionIDHeader, session.id)
			w.Header().Set(LegacyMCPSessionIDHeader, session.id)
			resp.Result = addSessionToInitializeResult(resp.Result, session.id)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func streamableEventsHandler(opts StreamableHTTPOptions, mgr *streamSessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyHTTPBaselineHeaders(w, r, opts.BehindHTTPSProxy)
		if opts.StrictHostCheck && !isHostAllowed(r.Host, opts.AllowedOrigins, opts.AllowAnyOrigin) {
			writeJSONError(w, http.StatusForbidden, "host not allowed")
			return
		}
		if !applyOriginPolicy(w, r, opts) {
			return
		}
		principal, err := opts.Authenticator.Authenticate(r.Context(), r)
		if err != nil {
			logHTTPAuthFailure("streamable_http", r, err)
			writeAuthFailure(w, err, opts.ExposeAuthErrors)
			return
		}
		r = r.WithContext(authn.WithPrincipal(r.Context(), &principal))
		sessionID := sessionIDFromRequest(r)
		if sessionID == "" {
			writeJSONError(w, http.StatusBadRequest, "missing session id")
			return
		}
		session, err := mgr.get(sessionID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "invalid session")
			return
		}
		if principal.Subject != session.principal.Subject || principal.TenantID != session.principal.TenantID {
			writeJSONError(w, http.StatusForbidden, "session principal mismatch")
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSONError(w, http.StatusInternalServerError, "streaming unsupported")
			return
		}
		mgr.touch(session.id)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		// Last-Event-ID resumability — absent/malformed falls back to full replay.
		var lastEventID uint64
		if hdr := strings.TrimSpace(r.Header.Get("Last-Event-ID")); hdr != "" {
			if n, err := strconv.ParseUint(hdr, 10, 64); err == nil {
				lastEventID = n
			}
		}
		ch, cancel := session.events.subscribeFrom(lastEventID)
		defer cancel()
		_, _ = fmt.Fprintf(w, ": session %s\n\n", session.id)
		flusher.Flush()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				_, _ = io.WriteString(w, ": keepalive\n\n")
				flusher.Flush()
			case event, ok := <-ch:
				if !ok {
					return
				}
				payload, _ := json.Marshal(map[string]any{
					"jsonrpc": "2.0",
					"method":  event.method,
					"params":  event.params,
				})
				_, _ = fmt.Fprintf(w, "id: %d\n", event.id)
				_, _ = fmt.Fprintf(w, "event: %s\n", event.method)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})
}

// validateProtocolVersion enforces Mcp-Protocol-Version on non-initialize
// requests. Absent = accept (pre-2025-03-26 clients). Present but unsupported
// or mismatched against the session's negotiated version = reject.
func validateProtocolVersion(r *http.Request, session *streamSession) error {
	v := strings.TrimSpace(r.Header.Get("Mcp-Protocol-Version"))
	if v == "" {
		return nil
	}
	if !slices.Contains(SupportedProtocolVersions, v) {
		return fmt.Errorf("unsupported Mcp-Protocol-Version %q", v)
	}
	if negotiated := session.server.NegotiatedProtocolVersion(); negotiated != "" && v != negotiated {
		return fmt.Errorf("Mcp-Protocol-Version %q does not match session %q", v, negotiated)
	}
	return nil
}

// applyHTTPBaselineHeaders writes the static security headers all
// streamable-HTTP responses share. HSTS is conditional: per
// ChatGPT's audit, advertising Strict-Transport-Security on a
// plaintext response makes honest http:// URLs unreachable for
// clients that cache it (browsers in particular). Emit only when
// the connection actually carries TLS (r.TLS != nil) or when the
// operator has declared a trusted HTTPS-terminating proxy in front
// of us via MCP_BEHIND_HTTPS_PROXY=1.
func applyHTTPBaselineHeaders(w http.ResponseWriter, r *http.Request, behindHTTPSProxy bool) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r != nil && (r.TLS != nil || behindHTTPSProxy) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "()")
}

func addSessionToInitializeResult(result any, sessionID string) any {
	m, ok := result.(map[string]any)
	if !ok {
		return result
	}
	out := make(map[string]any, len(m)+1)
	maps.Copy(out, m)
	out["sessionId"] = sessionID
	return out
}

// defaultIdleGraceAfterDisconnect is the shipped default for early eviction
// of orphaned sessions — conservatively longer than any realistic SSE retry
// backoff so a transient TCP blip does not drop a legitimate reconnecting
// client, but much shorter than SessionTTL so dead sessions do not sit for
// the full TTL holding memory and metric counters.
const defaultIdleGraceAfterDisconnect = 5 * time.Minute

type streamSessionManager struct {
	mu                       sync.Mutex
	ttl                      time.Duration
	idleGraceAfterDisconnect time.Duration
	store                    controlplane.Store
	items                    map[string]*streamSession
}

func (m *streamSessionManager) create(ctx context.Context, id string, principal authn.Principal, opts StreamableHTTPOptions) (*streamSession, error) {
	runtime, err := opts.Factory(ctx, principal, id)
	if err != nil {
		return nil, err
	}
	session := &streamSession{
		id:         id,
		principal:  principal,
		server:     runtime.Server,
		runtime:    runtime,
		events:     newSessionEventHub(64, 32),
		createdAt:  time.Now().UTC(),
		expiresAt:  time.Now().UTC().Add(m.ttl),
		lastSeenAt: time.Now().UTC(),
	}
	session.server.SetNotifier(session.events)
	session.server.advertiseListChanged.Store(true)
	session.server.AuditSessionID = id
	session.server.AuditTenantID = runtime.TenantID
	session.server.AuditSubject = principal.Subject
	session.server.AuditTransport = "streamable_http"
	if opts.SanitizeUpstreamErrors {
		session.server.SanitizeUpstreamErrors = true
	}
	m.mu.Lock()
	m.items[id] = session
	m.mu.Unlock()
	if err := m.putSession("create", controlplane.SessionRecord{
		ID:              id,
		TenantID:        runtime.TenantID,
		Subject:         principal.Subject,
		Transport:       "streamable_http",
		CreatedAt:       session.createdAt,
		ExpiresAt:       session.expiresAt,
		LastSeenAt:      session.lastSeenAt,
		WorkspaceID:     runtime.WorkspaceID,
		ClockifyBaseURL: runtime.ClockifyBaseURL,
	}); err != nil {
		m.mu.Lock()
		delete(m.items, id)
		m.mu.Unlock()
		session.closeRuntime()
		return nil, err
	}
	return session, nil
}

func (m *streamSessionManager) get(id string) (*streamSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.items[id]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	if time.Now().After(session.expiresAt) {
		go m.destroy(id, session)
		return nil, fmt.Errorf("session expired")
	}
	return session, nil
}

func (m *streamSessionManager) touch(id string) {
	m.mu.Lock()
	session, ok := m.items[id]
	if !ok {
		m.mu.Unlock()
		return
	}
	session.lastSeenAt = time.Now().UTC()
	session.expiresAt = session.lastSeenAt.Add(m.ttl)
	record := controlplane.SessionRecord{
		ID:              session.id,
		TenantID:        session.runtime.TenantID,
		Subject:         session.principal.Subject,
		Transport:       "streamable_http",
		CreatedAt:       session.createdAt,
		ExpiresAt:       session.expiresAt,
		LastSeenAt:      session.lastSeenAt,
		WorkspaceID:     session.runtime.WorkspaceID,
		ClockifyBaseURL: session.runtime.ClockifyBaseURL,
	}
	m.mu.Unlock()
	_ = m.putSession("touch", record)
}

func (m *streamSessionManager) syncInitializeState(session *streamSession) error {
	if session == nil {
		return nil
	}
	name, version := session.server.ClientInfo()
	return m.putSession("sync_initialize", controlplane.SessionRecord{
		ID:              session.id,
		TenantID:        session.runtime.TenantID,
		Subject:         session.principal.Subject,
		Transport:       "streamable_http",
		ProtocolVersion: session.server.NegotiatedProtocolVersion(),
		ClientName:      name,
		ClientVersion:   version,
		CreatedAt:       session.createdAt,
		ExpiresAt:       session.expiresAt,
		LastSeenAt:      session.lastSeenAt,
		WorkspaceID:     session.runtime.WorkspaceID,
		ClockifyBaseURL: session.runtime.ClockifyBaseURL,
	})
}

func (m *streamSessionManager) putSession(operation string, record controlplane.SessionRecord) error {
	if m.store == nil {
		return nil
	}
	if err := m.store.PutSession(record); err != nil {
		recordStreamableSessionStoreError(operation, record.ID, err)
		return err
	}
	return nil
}

func (m *streamSessionManager) deleteSession(operation, id string) error {
	if m.store == nil {
		return nil
	}
	if err := m.store.DeleteSession(id); err != nil {
		recordStreamableSessionStoreError(operation, id, err)
		return err
	}
	return nil
}

func recordStreamableSessionStoreError(operation, sessionID string, err error) {
	if err == nil {
		return
	}
	metrics.StreamableSessionStoreErrorsTotal.Inc(operation)
	slog.Warn("streamable_session_store_error",
		"operation", operation,
		"session_id", sessionID,
		"error", err.Error(),
	)
}

func (m *streamSessionManager) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapOnce(time.Now())
		}
	}
}

// reapOnce walks the session map and returns IDs that should be evicted
// at `now`. Extracted from reapLoop so tests can drive eviction on a fake
// clock without sleeping. Two eviction rules apply:
//
//  1. TTL expired — now > session.expiresAt. The original rule; evicts
//     sessions whose last request is older than SessionTTL (30 min default).
//  2. Orphaned subscriber — session has zero live SSE subscribers AND has
//     not been touched within `idleGraceAfterDisconnect`. Closes the gap
//     where a client drops TCP mid-stream without DELETEing the session:
//     without this, the session sits holding memory and the server's
//     notifier installation until TTL fires. The grace is sized to tolerate
//     any realistic SSE retry backoff, so a legitimate reconnecting client
//     that re-establishes within the grace is not dropped.
//
// destroy() is called outside the map lock because it takes m.mu itself
// and additionally calls into the control-plane store.
// reapEntry bundles an id+session pointer captured under the map lock so
// destroy() can be called outside the lock without a second m.get() call.
// m.get() uses real time.Now() (not the test's fake clock) and launches an
// asynchronous destroy goroutine on expiry, which makes tests that pass a
// fixed `now` non-deterministic. Capturing the pointer here avoids that.
type reapEntry struct {
	id      string
	session *streamSession
}

func (m *streamSessionManager) reapOnce(now time.Time) {
	type reapReason struct {
		reapEntry
		reason string
	}
	var evict []reapReason
	m.mu.Lock()
	for id, session := range m.items {
		if now.After(session.expiresAt) {
			evict = append(evict, reapReason{reapEntry{id, session}, "ttl"})
			continue
		}
		if m.idleGraceAfterDisconnect > 0 &&
			session.events.SubscriberCount() == 0 &&
			now.Sub(session.lastSeenAt) > m.idleGraceAfterDisconnect {
			evict = append(evict, reapReason{reapEntry{id, session}, "orphan"})
		}
	}
	m.mu.Unlock()
	for _, e := range evict {
		metrics.StreamableSessionsReapedTotal.Inc(e.reason)
		m.destroy(e.id, e.session)
	}
}

func (m *streamSessionManager) destroy(id string, session *streamSession) {
	m.mu.Lock()
	delete(m.items, id)
	m.mu.Unlock()
	if session == nil {
		return
	}
	session.events.close()
	session.closeRuntime()
	_ = m.deleteSession("delete", id)
}

func (m *streamSessionManager) closeAll() {
	m.mu.Lock()
	items := make([]*streamSession, 0, len(m.items))
	ids := make([]string, 0, len(m.items))
	for id, item := range m.items {
		ids = append(ids, id)
		items = append(items, item)
	}
	m.items = map[string]*streamSession{}
	m.mu.Unlock()
	for i, item := range items {
		item.events.close()
		item.closeRuntime()
		_ = m.deleteSession("close_all", ids[i])
	}
}

func (s *streamSession) closeRuntime() {
	if s != nil && s.runtime != nil && s.runtime.Close != nil {
		s.runtime.Close()
	}
}

type sessionEvent struct {
	id     uint64
	method string
	params any
}

// sessionEventHub delivers Server-Sent Events to the active SSE subscriber
// attached to a single streamable_http session. A new subscription replaces
// and closes older subscriptions so each server JSON-RPC message is delivered
// on one stream only. The backlog is a fixed-capacity ring buffer so event
// append is zero-alloc in steady state — the prior implementation reallocated
// the underlying slice and deep-copied the live tail every time the cap was
// exceeded.
//
// Ring buffer invariants:
//   - `backlog` is pre-allocated to backlogCap and is never grown.
//   - `backlogLen` is the number of live events, 0 ≤ backlogLen ≤ cap.
//   - `backlogStart` is the index of the oldest live event; events
//     walk forward as (backlogStart+i) mod cap for 0 ≤ i < backlogLen.
//   - Overflow overwrites the oldest event in place and advances
//     backlogStart; event IDs are monotonic so the replay walk still
//     returns events in order.
type sessionEventHub struct {
	mu           sync.Mutex
	nextID       int
	lastEventID  uint64
	subscribers  map[int]chan sessionEvent
	backlog      []sessionEvent
	backlogStart int
	backlogLen   int
	bufferCap    int
}

func newSessionEventHub(backlogCap, bufferCap int) *sessionEventHub {
	return &sessionEventHub{
		subscribers: map[int]chan sessionEvent{},
		backlog:     make([]sessionEvent, backlogCap),
		bufferCap:   bufferCap,
	}
}

func (h *sessionEventHub) Notify(method string, params any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastEventID++
	event := sessionEvent{id: h.lastEventID, method: method, params: params}

	ringCap := len(h.backlog)
	if ringCap > 0 {
		idx := (h.backlogStart + h.backlogLen) % ringCap
		h.backlog[idx] = event
		if h.backlogLen < ringCap {
			h.backlogLen++
		} else {
			// Ring full — overwrite oldest and slide the window forward.
			h.backlogStart = (h.backlogStart + 1) % ringCap
		}
	}

	for id, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			metrics.SSESubscriberDropsTotal.Inc("slow_subscriber")
			close(ch)
			delete(h.subscribers, id)
		}
		break
	}
	return nil
}

func (h *sessionEventHub) subscribe() (<-chan sessionEvent, func()) {
	return h.subscribeFrom(0)
}

// subscribeFrom replays backlog events with id > lastEventID (0 = replay all).
// Events trimmed from the ring buffer by overflow are unrecoverable — SSE
// best-effort semantics. The replay walk iterates the live window in order
// so Last-Event-ID based resumption receives events in the order they were
// originally published.
func (h *sessionEventHub) subscribeFrom(lastEventID uint64) (<-chan sessionEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextID
	h.nextID++
	for existingID, existing := range h.subscribers {
		close(existing)
		delete(h.subscribers, existingID)
	}
	// The channel buffer sizes the replay AND live-event headroom: we
	// must be able to push every retained backlog entry without blocking
	// (no reader has run yet — we are still inside the caller's
	// subscribe() frame) AND leave bufferCap slots for concurrent Notify
	// traffic that arrives before the subscriber starts draining.
	// Using h.bufferCap alone deadlocks the replay whenever backlogLen
	// exceeds bufferCap; sizing to bufferCap+backlogLen gives the same
	// live-event envelope as before while making the replay push
	// safe-by-construction. Bounded at bufferCap + backlogCap total.
	ch := make(chan sessionEvent, h.bufferCap+h.backlogLen)
	ringCap := len(h.backlog)
	// Detect resume misses: client asked for Last-Event-ID=X, but the
	// oldest retained event in the ring is > X+1, so events (X, oldest)
	// were trimmed by ring overflow. SSE semantics accept the loss but
	// operators should see it happen — bursty publishers outpacing a
	// replay window are an early warning of under-sized backlogCap.
	if lastEventID > 0 && h.backlogLen > 0 {
		oldest := h.backlog[h.backlogStart%ringCap].id
		if oldest > lastEventID+1 {
			metrics.SSEReplayMissesTotal.Inc()
		}
	}
	for i := 0; i < h.backlogLen; i++ {
		event := h.backlog[(h.backlogStart+i)%ringCap]
		if event.id > lastEventID {
			ch <- event
		}
	}
	h.subscribers[id] = ch
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if existing, ok := h.subscribers[id]; ok {
			close(existing)
			delete(h.subscribers, id)
		}
	}
}

func (h *sessionEventHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, ch := range h.subscribers {
		close(ch)
		delete(h.subscribers, id)
	}
}

// SubscriberCount reports the number of live SSE subscribers. Used by
// the session reaper to detect orphaned sessions (zero subscribers past
// the idle grace). Takes h.mu briefly so the count is consistent with
// Notify/subscribe/close observers.
func (h *sessionEventHub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers)
}

var randomIDRead = rand.Read

func randomID() (string, error) {
	var b [16]byte
	n, err := randomIDRead(b[:])
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	if n != len(b) {
		return "", fmt.Errorf("generate session id: %w", io.ErrUnexpectedEOF)
	}
	return hex.EncodeToString(b[:]), nil
}

func stringsTrimSpace(s string) string {
	return strings.TrimSpace(s)
}

func sessionIDFromRequest(r *http.Request) string {
	if sid := stringsTrimSpace(r.Header.Get(MCPSessionIDHeader)); sid != "" {
		return sid
	}
	return stringsTrimSpace(r.Header.Get(LegacyMCPSessionIDHeader))
}
