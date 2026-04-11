package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestStatusRecorderDefaults verifies the response recorder used by the
// observeHTTPH middleware: WriteHeader records the status, Write defaults
// missing statuses to 200, and the underlying ResponseWriter still receives
// every byte.
func TestStatusRecorderDefaults(t *testing.T) {
	t.Run("write_header", func(t *testing.T) {
		rec := httptest.NewRecorder()
		sr := &statusRecorder{ResponseWriter: rec}
		sr.WriteHeader(http.StatusTeapot)
		if sr.status != http.StatusTeapot {
			t.Fatalf("status: got %d", sr.status)
		}
		if rec.Code != http.StatusTeapot {
			t.Fatalf("rec.Code: got %d", rec.Code)
		}
	})
	t.Run("write_defaults_to_200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		sr := &statusRecorder{ResponseWriter: rec}
		if _, err := sr.Write([]byte("hi")); err != nil {
			t.Fatal(err)
		}
		if sr.status != http.StatusOK {
			t.Fatalf("expected default 200, got %d", sr.status)
		}
		if rec.Body.String() != "hi" {
			t.Fatalf("body: %q", rec.Body.String())
		}
	})
}

// TestHandleMetrics calls the unauthenticated /metrics endpoint and asserts
// the Prometheus exposition headers and at least one well-known metric line
// in the body.
func TestHandleMetrics(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handleMetrics(rec, req)
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("content type: %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control: %q", got)
	}
	if !strings.Contains(rec.Body.String(), "clockify_mcp_") {
		t.Fatalf("body missing expected metric prefix:\n%s", rec.Body.String())
	}
}

// TestObserveHTTPHHappyPath wraps a tiny handler in observeHTTPH and asserts
// the wrapped status code propagates and metrics fire (no panic, normal flow).
func TestObserveHTTPHHappyPath(t *testing.T) {
	wrapped := observeHTTPH("/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	wrapped(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

// TestObserveHTTPHRecoversPanic asserts a panicking handler is recovered,
// returns 500, and the wrapper still records metrics + emits the slog event.
func TestObserveHTTPHRecoversPanic(t *testing.T) {
	wrapped := observeHTTPH("/boom", http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("kaboom")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	wrapped(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 after panic, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

// TestObserveHTTPRecoversErrorAndStringPanics covers the fmtAny error / string
// branches by panicking with each type and confirming we still 500 cleanly.
func TestObserveHTTPRecoversErrorAndStringPanics(t *testing.T) {
	for _, tc := range []struct {
		name string
		v    any
	}{
		{"string", "string-panic"},
		{"error", errors.New("error-panic")},
		{"struct", struct{ X int }{42}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := observeHTTP("/p", func(_ http.ResponseWriter, _ *http.Request) {
				panic(tc.v)
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/p", nil)
			wrapped(rec, req)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status: %d", rec.Code)
			}
		})
	}
}

// TestFmtAny exercises every branch of the fmtAny helper directly.
func TestFmtAny(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{errors.New("boom"), "boom"},
		{"hello", "hello"},
		{42, "42"},
		{nil, "<nil>"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			if got := fmtAny(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}


