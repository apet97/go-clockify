//go:build legacy_platform

package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/metrics"
)

func TestStreamableHTTPSessionIsolation(t *testing.T) {
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-a",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	mgr := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	opts := StreamableHTTPOptions{
		Version:       "test",
		MaxBodySize:   2097152,
		SessionTTL:    30 * time.Minute,
		Authenticator: authenticator,
		ControlPlane:  store,
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			var server *Server
			activateTool := ToolDescriptor{
				Tool:    Tool{Name: "extra_tool", Description: "activated tool"},
				Handler: func(_ context.Context, _ map[string]any) (any, error) { return map[string]any{"ok": true}, nil },
			}
			search := ToolDescriptor{
				Tool: Tool{Name: "search_tools", Description: "activates tools"},
				Handler: func(_ context.Context, args map[string]any) (any, error) {
					if group, _ := args["activate_group"].(string); group == "extra" {
						if err := server.ActivateGroup(group, []ToolDescriptor{activateTool}); err != nil {
							return nil, err
						}
					}
					return map[string]any{"activated": args["activate_group"]}, nil
				},
			}
			server = NewServer("test", []ToolDescriptor{search}, nil, nil)
			return &StreamableSessionRuntime{
				Server:          server,
				Close:           func() {},
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
	}

	handler := streamableRPCHandler(opts, mgr)
	session1 := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"client-1","version":"1"}}}`)
	session2 := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"client-2","version":"2"}}}`)

	_ = callStreamSession(t, handler, session1, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_tools","arguments":{"activate_group":"extra"}}}`)
	list1 := callStreamSession(t, handler, session1, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	list2 := callStreamSession(t, handler, session2, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`)

	if !toolsListContains(list1, "extra_tool") {
		t.Fatal("session1 should include activated tool")
	}
	if toolsListContains(list2, "extra_tool") {
		t.Fatal("session2 should not inherit activated tool")
	}

	s1, err := mgr.get(context.Background(), session1, authn.Principal{}, opts)
	if err != nil {
		t.Fatalf("session1 lookup: %v", err)
	}
	s2, err := mgr.get(context.Background(), session2, authn.Principal{}, opts)
	if err != nil {
		t.Fatalf("session2 lookup: %v", err)
	}
	if s1.events.backlogLen == 0 {
		t.Fatal("session1 should have a queued listChanged notification")
	}
	if s2.events.backlogLen != 0 {
		t.Fatal("session2 should not receive session1 notifications")
	}
}

func initializeStreamSession(t *testing.T, handler http.Handler, body string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	sessionID := rec.Header().Get(MCPSessionIDHeader)
	if sessionID == "" {
		t.Fatal("missing session id header")
	}
	if legacy := rec.Header().Get(LegacyMCPSessionIDHeader); legacy != sessionID {
		t.Fatalf("legacy session id header = %q, want %q", legacy, sessionID)
	}
	return sessionID
}

func callStreamSession(t *testing.T, handler http.Handler, sessionID, body string) Response {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	handler.ServeHTTP(rec, req)
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

type failingSessionStore struct {
	controlplane.Store
	mu          sync.Mutex
	putCalls    int
	failPutOn   map[int]bool
	failDeletes bool
}

func (s *failingSessionStore) PutSession(record controlplane.SessionRecord) error {
	s.mu.Lock()
	s.putCalls++
	call := s.putCalls
	fail := s.failPutOn[call]
	s.mu.Unlock()
	if fail {
		return errors.New("put session failed")
	}
	return s.Store.PutSession(record)
}

func (s *failingSessionStore) DeleteSession(id string) error {
	s.mu.Lock()
	fail := s.failDeletes
	s.mu.Unlock()
	if fail {
		return errors.New("delete session failed")
	}
	return s.Store.DeleteSession(id)
}

// newTestStreamableStack builds a minimal streamable HTTP stack wired for
// unit tests: static bearer auth, in-memory control plane, no-op tool
// handlers. Returns the session manager + the configured opts so tests can
// hand them to streamableRPCHandler / streamableEventsHandler directly or
// mount them on a test mux.
func newTestStreamableStack(t *testing.T) (*streamSessionManager, StreamableHTTPOptions) {
	t.Helper()
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-a",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	mgr := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	opts := StreamableHTTPOptions{
		Version:       "test",
		MaxBodySize:   2097152,
		SessionTTL:    30 * time.Minute,
		Authenticator: authenticator,
		ControlPlane:  store,
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			server := NewServer("test", []ToolDescriptor{}, nil, nil)
			return &StreamableSessionRuntime{
				Server:          server,
				Close:           func() {},
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
	}
	return mgr, opts
}

type writeDeadlineRecorder struct {
	*httptest.ResponseRecorder
	mu        sync.Mutex
	deadlines []time.Time
}

func (r *writeDeadlineRecorder) SetWriteDeadline(t time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deadlines = append(r.deadlines, t)
	return nil
}

func (r *writeDeadlineRecorder) deadlineCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.deadlines)
}

func (r *writeDeadlineRecorder) deadlineAt(i int) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deadlines[i]
}

type blockingAuthenticator struct {
	entered chan struct{}
	release chan struct{}
}

func (a blockingAuthenticator) Authenticate(ctx context.Context, _ *http.Request) (authn.Principal, error) {
	close(a.entered)
	select {
	case <-a.release:
		return authn.Principal{Subject: "blocked-user", TenantID: "tenant-a"}, nil
	case <-ctx.Done():
		return authn.Principal{}, ctx.Err()
	}
}

func TestStreamableRPCHandlerAppliesPOSTWriteDeadline(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.POSTWriteTimeout = 1250 * time.Millisecond
	handler := streamableRPCHandler(opts, mgr)

	rec := &writeDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)

	if rec.deadlineCount() != 2 {
		t.Fatalf("deadline calls=%d, want set+clear", rec.deadlineCount())
	}
	if rec.deadlineAt(0).IsZero() {
		t.Fatal("first deadline should set a non-zero write deadline")
	}
	if got := time.Until(rec.deadlineAt(0)); got < time.Second || got > 2*time.Second {
		t.Fatalf("deadline delta=%s, want around 1.25s", got)
	}
	if !rec.deadlineAt(1).IsZero() {
		t.Fatalf("second deadline should clear the deadline, got %s", rec.deadlineAt(1))
	}
}

func TestStreamableRPCHandlerPOSTWriteDeadlineStartsAtFirstWrite(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	opts.Authenticator = blockingAuthenticator{entered: entered, release: release}
	opts.POSTWriteTimeout = 1250 * time.Millisecond
	handler := streamableRPCHandler(opts, mgr)

	rec := &writeDeadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{`))
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(rec, req)
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("authenticator was not called")
	}
	if got := rec.deadlineCount(); got != 0 {
		t.Fatalf("deadline should not be set before authentication returns, got %d calls", got)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
	if rec.deadlineCount() != 2 {
		t.Fatalf("deadline calls=%d, want set+clear", rec.deadlineCount())
	}
}

func TestStreamableCORSPreflightAllowsResumeHeaders(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.AllowedOrigins = []string{"https://client.example.com"}
	handler := streamableRPCHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Header.Set("Origin", "https://client.example.com")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status %d body %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://client.example.com" {
		t.Fatalf("allow-origin = %q", got)
	}
	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	for _, want := range []string{"MCP-Session-Id", "X-MCP-Session-ID", "MCP-Protocol-Version", "Last-Event-ID"} {
		if !strings.Contains(allowHeaders, want) {
			t.Fatalf("allow-headers missing %s: %q", want, allowHeaders)
		}
	}
}

func TestStreamableReadyDoesNotLeakReason(t *testing.T) {
	_, opts := newTestStreamableStack(t)
	const leaked = "dial tcp 10.0.1.5:5432: connect: connection refused"
	opts.ReadyChecker = func(context.Context) error {
		return errors.New(leaked)
	}
	ln, addr := newLoopbackListener(t)
	opts.Listener = ln
	opts.Bind = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeStreamableHTTP(ctx, opts)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ServeStreamableHTTP did not return after shutdown")
		}
	})

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/ready")
	if err != nil {
		t.Fatalf("GET /ready: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /ready body: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", resp.StatusCode, string(bodyBytes))
	}
	var body map[string]string
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("decode /ready body: %v", err)
	}
	if body["status"] != "not_ready" {
		t.Fatalf("expected status not_ready, got %q", body["status"])
	}
	if body["reason"] != publicReadyFailureReason {
		t.Fatalf("expected generic ready reason, got %q", body["reason"])
	}
	if strings.Contains(string(bodyBytes), "10.0.1.5") || strings.Contains(string(bodyBytes), leaked) {
		t.Fatalf("ready response leaked upstream error: %s", string(bodyBytes))
	}
}

func TestStreamableHealthEndpointOmitsVersion(t *testing.T) {
	_, opts := newTestStreamableStack(t)
	ln, addr := newLoopbackListener(t)
	opts.Listener = ln
	opts.Bind = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeStreamableHTTP(ctx, opts)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ServeStreamableHTTP did not return after shutdown")
		}
	})

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /health body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, string(bodyBytes))
	}
	var body map[string]string
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("decode /health body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
	if _, ok := body["version"]; ok {
		t.Fatalf("health response must not expose version: %+v", body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
}

func TestStreamableSessionHeaderAndStatusCodes(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	events := streamableEventsHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	sessionID := rec.Header().Get(MCPSessionIDHeader)
	if sessionID == "" {
		t.Fatalf("missing canonical session header: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(LegacyMCPSessionIDHeader); got != sessionID {
		t.Fatalf("legacy session header = %q, want %q", got, sessionID)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing session status = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, "does-not-exist")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("invalid session status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	events.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing SSE session status = %d, want 400", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, "does-not-exist")
	events.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("invalid SSE session status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(LegacyMCPSessionIDHeader, sessionID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy header status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStreamableNotificationOnlyPostAccepted(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestStreamableInitializeRequiresID(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	if rec.Header().Get(MCPSessionIDHeader) != "" {
		t.Fatal("initialize notification must not create a session")
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected -32600, got %+v", resp.Error)
	}
}

func TestStreamableInitializeFailsClosedOnSessionIDEntropyError(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	orig := randomIDRead
	t.Cleanup(func() { randomIDRead = orig })
	randomIDRead = func([]byte) (int, error) {
		return 0, errors.New("entropy unavailable")
	}
	handler := streamableRPCHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(MCPSessionIDHeader) != "" || rec.Header().Get(LegacyMCPSessionIDHeader) != "" {
		t.Fatal("entropy failure must not emit a session id header")
	}
	if len(mgr.items) != 0 {
		t.Fatalf("entropy failure created session(s): %d", len(mgr.items))
	}
	if !strings.Contains(rec.Body.String(), "session id generation failed") {
		t.Fatalf("body = %q, want generic session id failure", rec.Body.String())
	}
}

func TestStreamableSessionPersistenceFailures(t *testing.T) {
	t.Run("create_failure_aborts_session", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		base := mgr.store
		store := &failingSessionStore{Store: base, failPutOn: map[int]bool{1: true}}
		mgr.store = store
		opts.ControlPlane = store
		closed := false
		opts.Factory = func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			server := NewServer("test", nil, nil, nil)
			return &StreamableSessionRuntime{
				Server:   server,
				Close:    func() { closed = true },
				TenantID: principal.TenantID,
			}, nil
		}
		handler := streamableRPCHandler(opts, mgr)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if !closed {
			t.Fatal("runtime close was not called")
		}
		if len(mgr.items) != 0 {
			t.Fatalf("session leaked after create failure: %d", len(mgr.items))
		}
	})

	t.Run("sync_failure_aborts_initialized_session", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		base := mgr.store
		store := &failingSessionStore{Store: base, failPutOn: map[int]bool{2: true}}
		mgr.store = store
		opts.ControlPlane = store
		closed := false
		opts.Factory = func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			server := NewServer("test", nil, nil, nil)
			return &StreamableSessionRuntime{
				Server:   server,
				Close:    func() { closed = true },
				TenantID: principal.TenantID,
			}, nil
		}
		handler := streamableRPCHandler(opts, mgr)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if !closed {
			t.Fatal("runtime close was not called")
		}
		if len(mgr.items) != 0 {
			t.Fatalf("session leaked after sync failure: %d", len(mgr.items))
		}
	})

	t.Run("touch_and_delete_failures_are_observable_best_effort", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		base := mgr.store
		store := &failingSessionStore{Store: base, failPutOn: map[int]bool{3: true}}
		mgr.store = store
		opts.ControlPlane = store
		handler := streamableRPCHandler(opts, mgr)
		sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

		var logs bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
		t.Cleanup(func() { slog.SetDefault(prev) })

		baseMetric := metrics.StreamableSessionStoreErrorsTotal.Get("touch")
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		req.Header.Set(MCPSessionIDHeader, sessionID)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("touch failure should be best-effort, got status=%d body=%s", rec.Code, rec.Body.String())
		}
		if got := metrics.StreamableSessionStoreErrorsTotal.Get("touch") - baseMetric; got < 1 {
			t.Fatalf("expected touch metric increment, got %d", got)
		}

		store.mu.Lock()
		store.failDeletes = true
		store.mu.Unlock()
		baseMetric = metrics.StreamableSessionStoreErrorsTotal.Get("delete")
		session, _ := mgr.get(context.Background(), sessionID, authn.Principal{}, opts)
		mgr.destroy(sessionID, session)
		if got := metrics.StreamableSessionStoreErrorsTotal.Get("delete") - baseMetric; got < 1 {
			t.Fatalf("expected delete metric increment, got %d", got)
		}
		if strings.Contains(logs.String(), sessionID) || strings.Contains(logs.String(), `"session_id"`) {
			t.Fatalf("streamable session store log leaked session id %q:\n%s", sessionID, logs.String())
		}
		if !strings.Contains(logs.String(), `"streamable_session_store_error"`) {
			t.Fatalf("expected streamable session store warning, got:\n%s", logs.String())
		}
	})
}

// TestStreamableUnifiedRouteSSE verifies that GET /mcp serves the SSE stream
// (the spec-canonical location per MCP Streamable HTTP 2025-03-26 §3.3),
// including the new per-event `id:` line required for Last-Event-ID resumability.
func TestStreamableUnifiedRouteSSE(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))
	mux.Handle("GET /mcp/events", streamableEventsHandler(opts, mgr))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sessionID := initializeStreamSession(t, mux, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	session, err := mgr.get(context.Background(), sessionID, authn.Principal{}, opts)
	if err != nil {
		t.Fatalf("session lookup: %v", err)
	}
	if err := session.events.Notify("test/event", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type: %q", ct)
	}

	// Read line by line until we see the id/event pair or context fires.
	reader := bufio.NewReader(resp.Body)
	var sawID, sawEvent bool
	for !sawID || !sawEvent {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v (sawID=%v sawEvent=%v)", err, sawID, sawEvent)
		}
		if strings.HasPrefix(line, "id: 1") {
			sawID = true
		}
		if strings.HasPrefix(line, "event: test/event") {
			sawEvent = true
		}
	}
}

// TestStreamableEventsBackCompatAlias verifies that GET /mcp/events still
// serves the SSE stream as the indefinitely-mounted legacy alias for the
// pre-1.0 transport route shape (see ADR-0012 — removing an operator-
// facing route is a major-version bump).
func TestStreamableEventsBackCompatAlias(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	aliasRequests := metrics.StreamableEventsAliasRequestsTotal.Get()

	mux := http.NewServeMux()
	mux.Handle("POST /mcp", streamableRPCHandler(opts, mgr))
	mux.Handle("GET /mcp", streamableEventsHandler(opts, mgr))
	mux.Handle("GET /mcp/events", streamableEventsHandler(opts, mgr))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	sessionID := initializeStreamSession(t, mux, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/mcp/events", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type: %q", ct)
	}
	if got := metrics.StreamableEventsAliasRequestsTotal.Get() - aliasRequests; got != 1 {
		t.Fatalf("legacy alias metric delta = %d, want 1", got)
	}
}

func TestStreamableEventsAliasFullMuxRecordsHTTPPathMetric(t *testing.T) {
	_, opts := newTestStreamableStack(t)
	ln, addr := newLoopbackListener(t)
	opts.Listener = ln
	opts.Bind = addr

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeStreamableHTTP(ctx, opts)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("ServeStreamableHTTP did not return after shutdown")
		}
	})

	client := &http.Client{Timeout: 2 * time.Second}
	initReq, _ := http.NewRequest(
		http.MethodPost,
		"http://"+addr+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`),
	)
	initReq.Header.Set("Authorization", "Bearer "+testBearerToken)
	initResp, err := client.Do(initReq)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	_ = initResp.Body.Close()
	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status: %d", initResp.StatusCode)
	}
	sessionID := initResp.Header.Get(MCPSessionIDHeader)
	if sessionID == "" {
		t.Fatal("initialize response missing session id")
	}

	httpRequests := metrics.HTTPRequestsTotal.Get("/mcp/events", http.MethodGet, "200")
	aliasRequests := metrics.StreamableEventsAliasRequestsTotal.Get()

	reqCtx, reqCancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+addr+"/mcp/events", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	resp, err := client.Do(req)
	if err != nil {
		reqCancel()
		t.Fatalf("GET /mcp/events: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		reqCancel()
		t.Fatalf("GET /mcp/events status: %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	reqCancel()

	deadline := time.After(1 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		httpDelta := metrics.HTTPRequestsTotal.Get("/mcp/events", http.MethodGet, "200") - httpRequests
		aliasDelta := metrics.StreamableEventsAliasRequestsTotal.Get() - aliasRequests
		if httpDelta == 1 && aliasDelta == 1 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("metric deltas after GET /mcp/events: http=%d alias=%d, want 1/1", httpDelta, aliasDelta)
		case <-tick.C:
		}
	}
}

func TestSessionEventHubMultipleSubscribersAllReceive(t *testing.T) {
	hub := newSessionEventHub(0, 1)
	first := make(chan sessionEvent, 1)
	second := make(chan sessionEvent, 1)
	hub.mu.Lock()
	hub.subscribers[1] = first
	hub.subscribers[2] = second
	hub.mu.Unlock()

	if err := hub.Notify("test/event", nil); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	for name, ch := range map[string]chan sessionEvent{"first": first, "second": second} {
		select {
		case event := <-ch:
			if event.method != "test/event" {
				t.Fatalf("%s subscriber event method=%q, want test/event", name, event.method)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s subscriber did not receive event", name)
		}
	}
}

// TestStreamableProtocolVersionMismatch rejects a non-initialize request
// with an unsupported or mismatched Mcp-Protocol-Version header.
func TestStreamableProtocolVersionMismatch(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	// Unsupported version entirely.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	req.Header.Set("Mcp-Protocol-Version", "1999-01-01")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unsupported: status %d body %s", rec.Code, rec.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected -32600, got %+v", resp.Error)
	}

	// Supported but wrong version for this session.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	req.Header.Set("Mcp-Protocol-Version", "2024-11-05")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched: status %d body %s", rec.Code, rec.Body.String())
	}
}

// TestStreamableInitializeProtocolVersionHeaderMismatch rejects a
// contradictory initialize handshake before creating a session. Without this
// guard the body-negotiated version won silently, and every later request from
// a proxy stamping the mismatched header failed after session creation.
func TestStreamableInitializeProtocolVersionHeaderMismatch(t *testing.T) {
	if len(SupportedProtocolVersions) < 2 {
		t.Skip("test requires at least two supported protocol versions")
	}
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+SupportedProtocolVersions[1]+`"}}`,
	))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Mcp-Protocol-Version", SupportedProtocolVersions[0])
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched initialize: status %d body %s", rec.Code, rec.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("expected -32600, got %+v", resp.Error)
	}
	if got := len(mgr.items); got != 0 {
		t.Fatalf("mismatched initialize created %d sessions", got)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Mcp-Protocol-Version", SupportedProtocolVersions[0])
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("matching initialize: status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestStreamableInitializeProtocolVersionHeaderHonorsConfiguredDefault(t *testing.T) {
	if len(SupportedProtocolVersions) < 3 {
		t.Skip("test requires at least three supported protocol versions")
	}
	configuredDefault := SupportedProtocolVersions[2]
	mgr, opts := newTestStreamableStack(t)
	opts.DefaultProtocolVersion = configuredDefault
	opts.Factory = func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
		server := NewServer("test", []ToolDescriptor{}, nil, nil)
		server.DefaultProtocolVersion = opts.DefaultProtocolVersion
		return &StreamableSessionRuntime{
			Server:          server,
			Close:           func() {},
			TenantID:        principal.TenantID,
			WorkspaceID:     "ws1",
			ClockifyBaseURL: "https://api.clockify.me/api/v1",
		}, nil
	}
	handler := streamableRPCHandler(opts, mgr)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set("Mcp-Protocol-Version", configuredDefault)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("configured-default initialize: status %d body %s", rec.Code, rec.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	result, _ := resp.Result.(map[string]any)
	if got := result["protocolVersion"]; got != configuredDefault {
		t.Fatalf("protocolVersion = %v, want %q", got, configuredDefault)
	}
}

func TestStreamableTouchPreservesNegotiationForRehydration(t *testing.T) {
	if len(SupportedProtocolVersions) < 2 {
		t.Skip("test requires at least two supported protocol versions")
	}
	negotiated := SupportedProtocolVersions[1]
	wrongButSupported := SupportedProtocolVersions[0]

	mgrA, opts := newTestStreamableStack(t)
	handlerA := streamableRPCHandler(opts, mgrA)
	sessionID := initializeStreamSession(t, handlerA,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+
			negotiated+`","clientInfo":{"name":"rehydrate-test","version":"1.0.0"}}}`)

	// Any non-initialize request refreshes LastSeenAt/ExpiresAt through
	// touch(). That rewrite must not drop the initialize negotiation
	// state stored for cross-instance rehydration.
	resp := callStreamSession(t, handlerA, sessionID, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	if resp.Error != nil {
		t.Fatalf("tools/list after initialize returned error: %+v", resp.Error)
	}
	record, ok := opts.ControlPlane.Session(sessionID)
	if !ok {
		t.Fatalf("session %q missing from control plane", sessionID)
	}
	if record.ProtocolVersion != negotiated {
		t.Fatalf("persisted protocol version after touch = %q, want %q", record.ProtocolVersion, negotiated)
	}
	if record.ClientName != "rehydrate-test" || record.ClientVersion != "1.0.0" {
		t.Fatalf("persisted client info after touch = %q/%q, want rehydrate-test/1.0.0",
			record.ClientName, record.ClientVersion)
	}

	// Simulate a second replica: empty local session cache, same durable
	// control plane, same runtime factory. The wrong but supported header
	// must still be rejected after MarkInitialized restores the persisted
	// negotiated version.
	mgrB := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: opts.ControlPlane,
		items: map[string]*streamSession{},
	}
	handlerB := streamableRPCHandler(opts, mgrB)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	req.Header.Set("Mcp-Protocol-Version", wrongButSupported)
	handlerB.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rehydrated mismatch: status %d body %s", rec.Code, rec.Body.String())
	}
	var mismatchResp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &mismatchResp); err != nil {
		t.Fatalf("decode mismatch response: %v", err)
	}
	if mismatchResp.Error == nil || mismatchResp.Error.Code != -32600 {
		t.Fatalf("expected -32600 after rehydration, got %+v", mismatchResp.Error)
	}
}

func TestStreamSessionManagerConcurrentRehydrationCoalesces(t *testing.T) {
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}
	const sessionID = "rehydrate-race-session"
	now := time.Now().UTC()
	principal := authn.Principal{Subject: "user-1", TenantID: "tenant-1"}
	if err := store.PutSession(controlplane.SessionRecord{
		ID:              sessionID,
		TenantID:        principal.TenantID,
		Subject:         principal.Subject,
		Transport:       "streamable_http",
		ProtocolVersion: SupportedProtocolVersions[0],
		ClientName:      "race-client",
		ClientVersion:   "1",
		CreatedAt:       now,
		ExpiresAt:       now.Add(30 * time.Minute),
		LastSeenAt:      now,
		WorkspaceID:     "ws-1",
		ClockifyBaseURL: "https://api.clockify.me/api/v1",
	}); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	mgr := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	var factoryCalls atomic.Int32
	var closedRuntimes atomic.Int32
	releaseFactory := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseFactory) }) })
	opts := StreamableHTTPOptions{
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			factoryCalls.Add(1)
			<-releaseFactory
			return &StreamableSessionRuntime{
				Server:          NewServer("test", nil, nil, nil),
				Close:           func() { closedRuntimes.Add(1) },
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws-1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
		ControlPlane: store,
		SessionTTL:   30 * time.Minute,
	}

	const callers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(callers)
	results := make([]*streamSession, callers)
	errs := make([]error, callers)
	for i := range callers {
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = mgr.get(context.Background(), sessionID, principal, opts)
		}(i)
	}
	close(start)

	deadline := time.After(time.Second)
	for factoryCalls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected concurrent rehydration factory calls, got %d", factoryCalls.Load())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	releaseOnce.Do(func() { close(releaseFactory) })
	wg.Wait()

	var first *streamSession
	for i, err := range errs {
		if err != nil {
			t.Fatalf("get[%d]: %v", i, err)
		}
		if results[i] == nil {
			t.Fatalf("get[%d] returned nil session", i)
		}
		if first == nil {
			first = results[i]
			continue
		}
		if results[i] != first {
			t.Fatalf("get[%d] returned a distinct session pointer: %p != %p", i, results[i], first)
		}
	}
	if got := len(mgr.items); got != 1 {
		t.Fatalf("manager retained %d sessions, want 1", got)
	}
	if factoryCalls.Load() < 2 {
		t.Fatalf("factory calls = %d, want >= 2 to exercise duplicate discard", factoryCalls.Load())
	}
	if closed := closedRuntimes.Load(); closed != factoryCalls.Load()-1 {
		t.Fatalf("discarded runtime closes = %d, want %d", closed, factoryCalls.Load()-1)
	}
	if first.server.NegotiatedProtocolVersion() != SupportedProtocolVersions[0] {
		t.Fatalf("rehydrated protocol = %q, want %q", first.server.NegotiatedProtocolVersion(), SupportedProtocolVersions[0])
	}
	name, version := first.server.ClientInfo()
	if name != "race-client" || version != "1" {
		t.Fatalf("rehydrated client info = %q/%q, want race-client/1", name, version)
	}
}

func TestStreamableHTTP_CrossPodCancel_IsBestEffort(t *testing.T) {
	authenticator, err := authn.New(authn.Config{
		Mode:            authn.ModeStaticBearer,
		BearerToken:     testBearerToken,
		DefaultTenantID: "tenant-a",
	})
	if err != nil {
		t.Fatalf("authenticator: %v", err)
	}
	store, err := controlplane.Open("memory")
	if err != nil {
		t.Fatalf("control plane: %v", err)
	}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	finished := make(chan error, 1)
	blockingTool := ToolDescriptor{
		Tool: Tool{
			Name:        "blocking_tool",
			Description: "blocks until released",
			InputSchema: map[string]any{"type": "object"},
		},
		ReadOnlyHint: true,
		Handler: func(ctx context.Context, _ map[string]any) (any, error) {
			started <- struct{}{}
			select {
			case <-ctx.Done():
				finished <- ctx.Err()
				return nil, ctx.Err()
			case <-release:
				finished <- nil
				return map[string]any{"released": true}, nil
			}
		},
	}
	opts := StreamableHTTPOptions{
		Version:       "test",
		MaxBodySize:   2097152,
		SessionTTL:    30 * time.Minute,
		Authenticator: authenticator,
		ControlPlane:  store,
		Factory: func(_ context.Context, principal authn.Principal, _ string) (*StreamableSessionRuntime, error) {
			return &StreamableSessionRuntime{
				Server:          NewServer("test", []ToolDescriptor{blockingTool}, nil, nil),
				Close:           func() {},
				TenantID:        principal.TenantID,
				WorkspaceID:     "ws1",
				ClockifyBaseURL: "https://api.clockify.me/api/v1",
			}, nil
		},
	}
	mgrA := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	mgrB := &streamSessionManager{
		ttl:   30 * time.Minute,
		store: store,
		items: map[string]*streamSession{},
	}
	handlerA := streamableRPCHandler(opts, mgrA)
	handlerB := streamableRPCHandler(opts, mgrB)
	sessionID := initializeStreamSession(t, handlerA, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)

	type callOutcome struct {
		status int
		body   string
		resp   Response
	}
	callDone := make(chan callOutcome, 1)
	go func() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
			`{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"blocking_tool","arguments":{}}}`,
		))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		req.Header.Set(MCPSessionIDHeader, sessionID)
		handlerA.ServeHTTP(rec, req)
		outcome := callOutcome{status: rec.Code, body: rec.Body.String()}
		_ = json.Unmarshal(rec.Body.Bytes(), &outcome.resp)
		callDone <- outcome
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for blocking tool to start")
	}
	mgrA.mu.Lock()
	sessionA := mgrA.items[sessionID]
	mgrA.mu.Unlock()
	if sessionA == nil {
		t.Fatal("session A missing after initialize")
		return
	}
	if got := sessionA.server.InflightCount(); got != 1 {
		t.Fatalf("expected instance A to track one in-flight call, got %d", got)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":42,"reason":"cross-pod best effort"}}`,
	))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	handlerB.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("cross-pod cancellation notification status %d body %s", rec.Code, rec.Body.String())
	}
	mgrB.mu.Lock()
	_, rehydratedOnB := mgrB.items[sessionID]
	mgrB.mu.Unlock()
	if !rehydratedOnB {
		t.Fatal("instance B should rehydrate the session before accepting cancellation")
	}

	select {
	case err := <-finished:
		t.Fatalf("cross-pod cancellation propagated to instance A unexpectedly: %v", err)
	case outcome := <-callDone:
		t.Fatalf("instance A call completed before local release: status=%d body=%s", outcome.status, outcome.body)
	case <-time.After(150 * time.Millisecond):
	}
	if got := sessionA.server.InflightCount(); got != 1 {
		t.Fatalf("cross-pod cancellation should leave instance A call in-flight, got %d", got)
	}

	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("blocking tool should finish normally after local release, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for blocking tool to finish after release")
	}
	select {
	case outcome := <-callDone:
		if outcome.status != http.StatusOK {
			t.Fatalf("tools/call status %d body %s", outcome.status, outcome.body)
		}
		if outcome.resp.Error != nil {
			t.Fatalf("tools/call returned error after local release: %+v", outcome.resp.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tools/call response after release")
	}
	if got := sessionA.server.InflightCount(); got != 0 {
		t.Fatalf("expected instance A inflight map to drain after completion, got %d", got)
	}
}

// TestStreamableProtocolVersionAbsent allows non-initialize requests without
// the Mcp-Protocol-Version header (pre-2025-03-26 clients).
func TestStreamableProtocolVersionAbsent(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(opts, mgr)
	sessionID := initializeStreamSession(t, handler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	missingHeaders := metrics.ProtocolVersionHeaderMissingTotal.Get()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("absent header: status %d body %s", rec.Code, rec.Body.String())
	}
	if got := metrics.ProtocolVersionHeaderMissingTotal.Get() - missingHeaders; got != 1 {
		t.Fatalf("missing-header metric delta = %d, want 1", got)
	}
}

func TestStreamableProtocolVersionRequired(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	opts.RequireProtocolVersionHeader = true
	rpcHandler := streamableRPCHandler(opts, mgr)
	eventsHandler := streamableEventsHandler(opts, mgr)
	sessionID := initializeStreamSession(t, rpcHandler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	missingHeaders := metrics.ProtocolVersionHeaderMissingTotal.Get()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	rpcHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("strict absent POST status %d body %s", rec.Code, rec.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode strict POST response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32600 || !strings.Contains(resp.Error.Message, "missing Mcp-Protocol-Version") {
		t.Fatalf("strict POST error = %+v, want -32600 missing-header error", resp.Error)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	req.Header.Set("Mcp-Protocol-Version", SupportedProtocolVersions[0])
	rpcHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("strict header-present POST status %d body %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	req.Header.Set(MCPSessionIDHeader, sessionID)
	eventsHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("strict absent GET status %d body %s", rec.Code, rec.Body.String())
	}
	if got := metrics.ProtocolVersionHeaderMissingTotal.Get() - missingHeaders; got != 2 {
		t.Fatalf("missing-header metric delta = %d, want 2", got)
	}
}

func toolsListContains(resp Response, name string) bool {
	result, ok := resp.Result.(map[string]any)
	if !ok {
		return false
	}
	items, ok := result["tools"].([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tool["name"] == name {
			return true
		}
	}
	return false
}
