package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
)

type StreamableSessionRuntime struct {
	Server          *Server
	Close           func()
	TenantID        string
	WorkspaceID     string
	ClockifyBaseURL string
}

type StreamableSessionFactory func(context.Context, authn.Principal, string) (*StreamableSessionRuntime, error)

type StreamableHTTPOptions struct {
	Version         string
	Bind            string
	MaxBodySize     int64
	AllowedOrigins  []string
	AllowAnyOrigin  bool
	StrictHostCheck bool
	SessionTTL      time.Duration
	ReadyChecker    func(context.Context) error
	Authenticator   authn.Authenticator
	ControlPlane    *controlplane.Store
	Factory         StreamableSessionFactory
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
	if opts.Bind == "" {
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

	mgr := &streamSessionManager{
		ttl:   opts.SessionTTL,
		store: opts.ControlPlane,
		items: map[string]*streamSession{},
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
	mux.Handle("GET /mcp/events", observeHTTPH("/mcp/events", streamableEventsHandler(opts, mgr)))

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
	slog.Info("streamable_http_start", "bind", opts.Bind, "session_ttl", opts.SessionTTL.String())
	ln, err := net.Listen("tcp", opts.Bind)
	if err != nil {
		return fmt.Errorf("listen streamable_http: %w", err)
	}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func streamableRPCHandler(opts StreamableHTTPOptions, mgr *streamSessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyHTTPBaselineHeaders(w)
		if opts.StrictHostCheck && !isHostAllowed(r.Host, opts.AllowedOrigins, opts.AllowAnyOrigin) {
			writeJSONError(w, http.StatusForbidden, "host not allowed")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if !isOriginAllowed(origin, opts.AllowedOrigins, opts.AllowAnyOrigin) {
				writeJSONError(w, http.StatusForbidden, "origin not allowed")
				return
			}
			if opts.AllowAnyOrigin {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-MCP-Session-ID")
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
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
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
			id := randomID()
			session, err = mgr.create(r.Context(), id, principal, opts)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("X-MCP-Session-ID", session.id)
		} else {
			sessionID := stringsTrimSpace(r.Header.Get("X-MCP-Session-ID"))
			if sessionID == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing session id")
				return
			}
			session, err = mgr.get(sessionID)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid session")
				return
			}
			if principal.Subject != session.principal.Subject || principal.TenantID != session.principal.TenantID {
				writeJSONError(w, http.StatusForbidden, "session principal mismatch")
				return
			}
		}
		mgr.touch(session.id)
		resp := session.server.handle(r.Context(), req)
		if req.Method == "initialize" {
			mgr.syncInitializeState(session)
			resp.Result = addSessionToInitializeResult(resp.Result, session.id)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func streamableEventsHandler(opts StreamableHTTPOptions, mgr *streamSessionManager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyHTTPBaselineHeaders(w)
		if opts.StrictHostCheck && !isHostAllowed(r.Host, opts.AllowedOrigins, opts.AllowAnyOrigin) {
			writeJSONError(w, http.StatusForbidden, "host not allowed")
			return
		}
		principal, err := opts.Authenticator.Authenticate(r.Context(), r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		sessionID := stringsTrimSpace(r.Header.Get("X-MCP-Session-ID"))
		if sessionID == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing session id")
			return
		}
		session, err := mgr.get(sessionID)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid session")
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
		ch, cancel := session.events.subscribe()
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
				_, _ = fmt.Fprintf(w, "event: %s\n", event.method)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	})
}

func applyHTTPBaselineHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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
	for k, v := range m {
		out[k] = v
	}
	out["sessionId"] = sessionID
	return out
}

type streamSessionManager struct {
	mu    sync.Mutex
	ttl   time.Duration
	store *controlplane.Store
	items map[string]*streamSession
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
	m.mu.Lock()
	m.items[id] = session
	m.mu.Unlock()
	_ = m.store.PutSession(controlplane.SessionRecord{
		ID:              id,
		TenantID:        runtime.TenantID,
		Subject:         principal.Subject,
		Transport:       "streamable_http",
		CreatedAt:       session.createdAt,
		ExpiresAt:       session.expiresAt,
		LastSeenAt:      session.lastSeenAt,
		WorkspaceID:     runtime.WorkspaceID,
		ClockifyBaseURL: runtime.ClockifyBaseURL,
	})
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
	_ = m.store.PutSession(record)
}

func (m *streamSessionManager) syncInitializeState(session *streamSession) {
	if session == nil {
		return
	}
	name, version := session.server.ClientInfo()
	_ = m.store.PutSession(controlplane.SessionRecord{
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

func (m *streamSessionManager) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var expired []string
			m.mu.Lock()
			now := time.Now()
			for id, session := range m.items {
				if now.After(session.expiresAt) {
					expired = append(expired, id)
				}
			}
			m.mu.Unlock()
			for _, id := range expired {
				if session, err := m.get(id); err == nil {
					m.destroy(id, session)
				}
			}
		}
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
	if session.runtime != nil && session.runtime.Close != nil {
		session.runtime.Close()
	}
	_ = m.store.DeleteSession(id)
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
		if item.runtime != nil && item.runtime.Close != nil {
			item.runtime.Close()
		}
		_ = m.store.DeleteSession(ids[i])
	}
}

type sessionEvent struct {
	method string
	params any
}

type sessionEventHub struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan sessionEvent
	backlog     []sessionEvent
	backlogCap  int
	bufferCap   int
}

func newSessionEventHub(backlogCap, bufferCap int) *sessionEventHub {
	return &sessionEventHub{
		subscribers: map[int]chan sessionEvent{},
		backlogCap:  backlogCap,
		bufferCap:   bufferCap,
	}
}

func (h *sessionEventHub) Notify(method string, params any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	event := sessionEvent{method: method, params: params}
	h.backlog = append(h.backlog, event)
	if len(h.backlog) > h.backlogCap {
		h.backlog = append([]sessionEvent(nil), h.backlog[len(h.backlog)-h.backlogCap:]...)
	}
	for id, ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			close(ch)
			delete(h.subscribers, id)
		}
	}
	return nil
}

func (h *sessionEventHub) subscribe() (<-chan sessionEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.nextID
	h.nextID++
	ch := make(chan sessionEvent, h.bufferCap)
	for _, event := range h.backlog {
		ch <- event
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

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func stringsTrimSpace(s string) string {
	return strings.TrimSpace(s)
}
