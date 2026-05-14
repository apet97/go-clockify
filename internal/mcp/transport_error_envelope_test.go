//go:build legacy_platform

package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPTransportAdmissionErrorsUseJSONRPCEnvelope(t *testing.T) {
	t.Run("legacy_host_rejected", func(t *testing.T) {
		s := newTestServer()
		s.StrictHostCheck = true
		handler := s.handleMCP(testBearerAuth(t), []string{"https://app.example.com"}, false, 2097152)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		req.Host = "0.0.0.0:8080"
		handler.ServeHTTP(rec, req)

		assertJSONRPCTransportError(t, rec, http.StatusForbidden, RPCCodeHostNotAllowed, "host not allowed")
	})

	t.Run("legacy_request_too_large", func(t *testing.T) {
		s := newTestServer()
		handler := s.handleMCP(testBearerAuth(t), nil, true, 16)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		handler.ServeHTTP(rec, req)

		assertJSONRPCTransportError(t, rec, http.StatusRequestEntityTooLarge, RPCCodeRequestTooLarge, "request too large")
	})

	t.Run("streamable_missing_session", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		handler := streamableRPCHandler(opts, mgr)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		handler.ServeHTTP(rec, req)

		assertJSONRPCTransportError(t, rec, http.StatusBadRequest, RPCCodeSessionInvalid, "missing session id")
	})

	t.Run("streamable_invalid_session_get", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		handler := streamableEventsHandler(opts, mgr)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+testBearerToken)
		req.Header.Set(MCPSessionIDHeader, "not-a-session")
		handler.ServeHTTP(rec, req)

		assertJSONRPCTransportError(t, rec, http.StatusNotFound, RPCCodeSessionInvalid, "invalid session")
	})

	t.Run("streamable_rate_limited", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		handler := streamableRPCHandler(
			opts,
			mgr,
			newHTTPAdmissionLimiter(HTTPAdmissionLimits{PerIPPerMinute: 1}),
		)

		for i := 0; i < 2; i++ {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
			req.Header.Set("Authorization", "Bearer wrong-token")
			handler.ServeHTTP(rec, req)
			if i == 1 {
				assertJSONRPCTransportError(t, rec, http.StatusTooManyRequests, RPCCodeRateLimited, "rate limited")
			}
		}
	})

	t.Run("streamable_auth_failure", func(t *testing.T) {
		mgr, opts := newTestStreamableStack(t)
		handler := streamableRPCHandler(opts, mgr)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
		req.Header.Set("Authorization", "Bearer wrong-token")
		handler.ServeHTTP(rec, req)

		assertJSONRPCTransportError(t, rec, http.StatusUnauthorized, RPCCodeUnauthenticated, "authentication failed")
		if got := rec.Header().Get("WWW-Authenticate"); !strings.Contains(got, "invalid_token") {
			t.Fatalf("WWW-Authenticate = %q, want invalid_token", got)
		}
	})
}

func assertJSONRPCTransportError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus, wantCode int, wantMessage string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   *RPCError       `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode JSON-RPC error envelope: %v; body=%s", err, rec.Body.String())
	}
	if envelope.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc = %q, want 2.0; body=%s", envelope.JSONRPC, rec.Body.String())
	}
	if string(envelope.ID) != "null" {
		t.Fatalf("id = %s, want null; body=%s", string(envelope.ID), rec.Body.String())
	}
	if envelope.Error == nil {
		t.Fatalf("missing error object; body=%s", rec.Body.String())
	}
	if envelope.Error.Code != wantCode {
		t.Fatalf("error.code = %d, want %d; body=%s", envelope.Error.Code, wantCode, rec.Body.String())
	}
	if envelope.Error.Message != wantMessage {
		t.Fatalf("error.message = %q, want %q", envelope.Error.Message, wantMessage)
	}
}
