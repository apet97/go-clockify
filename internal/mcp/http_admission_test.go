package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTTPAdmissionLimiterRejectsRepeatedPreAuthSource(t *testing.T) {
	s := newTestServer()
	handler := s.handleMCP(
		testBearerAuth(t),
		nil,
		true,
		2097152,
		newHTTPAdmissionLimiter(HTTPAdmissionLimits{PerIPPerMinute: 1}),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("first bad auth status = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer wrong-token")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second bad auth status = %d, want 429 body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatal("rate-limited response missing Retry-After")
	}
}

func TestHTTPAdmissionLimiterUsesForwardedForOnlyBehindProxy(t *testing.T) {
	s := newTestServer()
	s.BehindHTTPSProxy = true
	handler := s.handleMCP(
		testBearerAuth(t),
		nil,
		true,
		2097152,
		newHTTPAdmissionLimiter(HTTPAdmissionLimits{PerIPPerMinute: 1}),
	)

	forwardedReq := func(ip string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
		req.Header.Set("Authorization", "Bearer wrong-token")
		req.Header.Set("X-Forwarded-For", ip)
		return req
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardedReq("203.0.113.10"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("first forwarded source status = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardedReq("203.0.113.11"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("different forwarded source status = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, forwardedReq("203.0.113.10"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("repeated forwarded source status = %d, want 429", rec.Code)
	}
}

func TestStreamableHTTPAdmissionLimiterRejectsRepeatedPrincipal(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	handler := streamableRPCHandler(
		opts,
		mgr,
		newHTTPAdmissionLimiter(HTTPAdmissionLimits{PerPrincipalPerMinute: 1}),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first initialize status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"initialize"}`))
	req.Header.Set("Authorization", "Bearer "+testBearerToken)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second initialize status = %d, want 429 body=%s", rec.Code, rec.Body.String())
	}
}

func TestStreamableHTTPAdmissionLimiterCapsSSEPerSession(t *testing.T) {
	mgr, opts := newTestStreamableStack(t)
	limiter := newHTTPAdmissionLimiter(HTTPAdmissionLimits{SSEPerSession: 1})
	rpcHandler := streamableRPCHandler(opts, mgr, limiter)
	sessionID := initializeStreamSession(t, rpcHandler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	eventsHandler := streamableEventsHandler(opts, mgr, limiter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rec1 := newFlushRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req1.Header.Set("Authorization", "Bearer "+testBearerToken)
	req1.Header.Set(MCPSessionIDHeader, sessionID)

	done := make(chan struct{})
	go func() {
		eventsHandler.ServeHTTP(rec1, req1)
		close(done)
	}()

	select {
	case <-rec1.flushed:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("first SSE connection did not flush initial frame")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req2.Header.Set("Authorization", "Bearer "+testBearerToken)
	req2.Header.Set(MCPSessionIDHeader, sessionID)
	eventsHandler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second SSE status = %d, want 429 body=%s", rec2.Code, rec2.Body.String())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("first SSE connection did not stop after cancel")
	}
}

type flushRecorder struct {
	mu      sync.Mutex
	header  http.Header
	code    int
	body    strings.Builder
	flushed chan struct{}
	once    sync.Once
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{header: http.Header{}, flushed: make(chan struct{})}
}

func (r *flushRecorder) Header() http.Header {
	return r.header
}

func (r *flushRecorder) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = code
	}
}

func (r *flushRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(p)
}

func (r *flushRecorder) Flush() {
	r.once.Do(func() { close(r.flushed) })
}
