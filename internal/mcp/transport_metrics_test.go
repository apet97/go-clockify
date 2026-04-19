package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
}
