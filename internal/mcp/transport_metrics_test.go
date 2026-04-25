package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsMuxAuth(t *testing.T) {
	const testToken = "metrics-test-token-1234"

	t.Run("none_unauthenticated", func(t *testing.T) {
		mux := metricsMux(MetricsServerOptions{AuthMode: "none"})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		mux.ServeHTTP(rec, req)

		// handleMetrics will fail because no prometheus registry is set up in test
		// but it should NOT be 401
		if rec.Code == http.StatusUnauthorized {
			t.Fatal("expected NOT 401 for none auth")
		}
	})

	t.Run("static_bearer_valid", func(t *testing.T) {
		mux := metricsMux(MetricsServerOptions{
			AuthMode:    "static_bearer",
			BearerToken: testToken,
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		mux.ServeHTTP(rec, req)

		if rec.Code == http.StatusUnauthorized {
			t.Fatal("expected NOT 401 for valid static bearer")
		}
	})

	t.Run("static_bearer_invalid", func(t *testing.T) {
		mux := metricsMux(MetricsServerOptions{
			AuthMode:    "static_bearer",
			BearerToken: testToken,
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for invalid static bearer, got %d", rec.Code)
		}
	})

	t.Run("static_bearer_missing", func(t *testing.T) {
		mux := metricsMux(MetricsServerOptions{
			AuthMode:    "static_bearer",
			BearerToken: testToken,
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for missing static bearer, got %d", rec.Code)
		}
	})

	// Defense in depth: when an embedder constructs MetricsServerOptions
	// with AuthMode=static_bearer but forgets to set BearerToken,
	// subtle.ConstantTimeCompare([]byte(""), []byte("")) returns 1 — i.e.
	// any client request (including one with a bare "Bearer " prefix
	// trimming to empty) would be treated as authenticated. The handler
	// must instead refuse the request with 500 so the misconfiguration
	// is observable rather than silently fail-open.
	t.Run("static_bearer_empty_token_refuses_request", func(t *testing.T) {
		mux := metricsMux(MetricsServerOptions{
			AuthMode:    "static_bearer",
			BearerToken: "", // misconfigured: empty token
		})

		// Even an empty bearer header (which trims to "" and would
		// have matched ConstantTimeCompare's empty-vs-empty) must be
		// rejected.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer ")
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for static_bearer with empty configured token, got %d", rec.Code)
		}

		// Same for any other client-supplied token.
		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set("Authorization", "Bearer some-attacker-supplied-token")
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 for static_bearer with empty configured token (any client token), got %d", rec.Code)
		}
	})
}

// TestServeMetricsRejectsEmptyStaticBearer verifies that ServeMetrics
// refuses to start when AuthMode=static_bearer is configured without a
// bearer token. A short context ensures the test fails (rather than
// hangs) if the validation is removed: with the guard, ServeMetrics
// returns the validation error before binding; without it, the test
// would block on srv.Serve until the context cancels and returns nil.
func TestServeMetricsRejectsEmptyStaticBearer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ServeMetrics(ctx, MetricsServerOptions{
		Bind:        "127.0.0.1:0",
		AuthMode:    "static_bearer",
		BearerToken: "",
	})
	if err == nil {
		t.Fatal("expected error for static_bearer with empty bearer token, got nil (validation removed?)")
	}
	if !strings.Contains(err.Error(), "non-empty bearer token") {
		t.Fatalf("expected bearer-token validation error, got: %v", err)
	}
}

func TestServeMetricsRejectsUnknownAuthMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := ServeMetrics(ctx, MetricsServerOptions{
		Bind:     "127.0.0.1:0",
		AuthMode: "mtls", // not supported by the metrics listener
	})
	if err == nil {
		t.Fatal("expected error for unsupported auth_mode, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported auth_mode") {
		t.Fatalf("expected unsupported-auth-mode error, got: %v", err)
	}
}

// TestServeMetricsAcceptsValidConfigs spot-checks that the validation
// does not regress the happy paths.
func TestServeMetricsAcceptsValidConfigs(t *testing.T) {
	cases := []struct {
		name string
		opts MetricsServerOptions
	}{
		{"empty_bind_noop", MetricsServerOptions{Bind: ""}},
		{"empty_authmode_noauth", MetricsServerOptions{Bind: "127.0.0.1:0", AuthMode: ""}},
		{"none_authmode", MetricsServerOptions{Bind: "127.0.0.1:0", AuthMode: "none"}},
		{"static_bearer_with_token", MetricsServerOptions{Bind: "127.0.0.1:0", AuthMode: "static_bearer", BearerToken: "metrics-test-token-1234"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := ServeMetrics(ctx, tc.opts)
			// Either nil (empty bind branch) or no validation error.
			// The listener may return after ctx cancels, which is also fine.
			if err != nil && (strings.Contains(err.Error(), "non-empty bearer token") || strings.Contains(err.Error(), "unsupported auth_mode")) {
				t.Fatalf("unexpected validation error for valid config: %v", err)
			}
		})
	}
}
