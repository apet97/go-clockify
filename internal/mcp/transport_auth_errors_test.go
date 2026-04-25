package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/authn"
)

const secretAuthDetail = "secret-issuer-detail-XYZ123"

type rejectingAuthenticator struct {
	err error
}

func (a rejectingAuthenticator) Authenticate(context.Context, *http.Request) (authn.Principal, error) {
	return authn.Principal{}, a.err
}

func TestUnauthorizedDoesNotExposeOIDCDetailsByDefault(t *testing.T) {
	detailErr := errors.New("oidc issuer rejected token at secret-issuer-detail-XYZ123")

	for _, tt := range []struct {
		name string
		run  func(*testing.T) *httptest.ResponseRecorder
	}{
		{
			name: "legacy_http",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				s := newTestServer()
				handler := s.handleMCP(rejectingAuthenticator{err: detailErr}, nil, true, 2097152)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
		{
			name: "streamable_http_rpc",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				mgr, opts := newTestStreamableStack(t)
				opts.Authenticator = rejectingAuthenticator{err: detailErr}
				handler := streamableRPCHandler(opts, mgr)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
		{
			name: "streamable_http_events",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				mgr, opts := newTestStreamableStack(t)
				opts.Authenticator = rejectingAuthenticator{err: detailErr}
				handler := streamableEventsHandler(opts, mgr)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.run(t)
			assertUnauthorizedBodyAndHeader(t, rec)
			assertDoesNotContain(t, rec.Body.String(), secretAuthDetail, "body")
			assertDoesNotContain(t, rec.Header().Get("WWW-Authenticate"), secretAuthDetail, "WWW-Authenticate")
		})
	}
}

func TestUnauthorizedCanExposeDetailsWhenExplicitlyEnabled(t *testing.T) {
	detailErr := errors.New("oidc issuer rejected token at secret-issuer-detail-XYZ123")

	for _, tt := range []struct {
		name string
		run  func(*testing.T) *httptest.ResponseRecorder
	}{
		{
			name: "legacy_http",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				s := newTestServer()
				s.ExposeAuthErrors = true
				handler := s.handleMCP(rejectingAuthenticator{err: detailErr}, nil, true, 2097152)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
		{
			name: "streamable_http_rpc",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				mgr, opts := newTestStreamableStack(t)
				opts.Authenticator = rejectingAuthenticator{err: detailErr}
				opts.ExposeAuthErrors = true
				handler := streamableRPCHandler(opts, mgr)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
		{
			name: "streamable_http_events",
			run: func(t *testing.T) *httptest.ResponseRecorder {
				t.Helper()
				mgr, opts := newTestStreamableStack(t)
				opts.Authenticator = rejectingAuthenticator{err: detailErr}
				opts.ExposeAuthErrors = true
				handler := streamableEventsHandler(opts, mgr)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
				handler.ServeHTTP(rec, req)
				return rec
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.run(t)
			assertUnauthorizedBodyAndHeader(t, rec)
			assertContains(t, rec.Body.String(), secretAuthDetail, "body")
			assertContains(t, rec.Header().Get("WWW-Authenticate"), secretAuthDetail, "WWW-Authenticate")
		})
	}
}

func assertUnauthorizedBodyAndHeader(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
	assertContains(t, rec.Body.String(), "invalid_token", "body")
	assertContains(t, rec.Header().Get("WWW-Authenticate"), "invalid_token", "WWW-Authenticate")
}

func assertContains(t *testing.T, got, needle, field string) {
	t.Helper()
	if !strings.Contains(got, needle) {
		t.Fatalf("%s does not contain %q: %s", field, needle, got)
	}
}

func assertDoesNotContain(t *testing.T, got, needle, field string) {
	t.Helper()
	if strings.Contains(got, needle) {
		t.Fatalf("%s leaked %q: %s", field, needle, got)
	}
}
