package mcp

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type MetricsServerOptions struct {
	Bind        string
	AuthMode    string
	BearerToken string
}

func ServeMetrics(ctx context.Context, opts MetricsServerOptions) error {
	if strings.TrimSpace(opts.Bind) == "" {
		return nil
	}
	// Refuse to start when the configured auth mode would silently
	// downgrade to unauthenticated. internal/config/config.go enforces
	// the same invariants on the documented startup path; this guard
	// catches programmatic embedders that construct MetricsServerOptions
	// directly, where subtle.ConstantTimeCompare("","") == 1 would
	// otherwise treat any client (even one sending a bare "Bearer ")
	// as authenticated.
	switch opts.AuthMode {
	case "", "none":
	case "static_bearer":
		if strings.TrimSpace(opts.BearerToken) == "" {
			return fmt.Errorf("metrics: auth_mode=static_bearer requires a non-empty bearer token")
		}
	default:
		return fmt.Errorf("metrics: unsupported auth_mode %q", opts.AuthMode)
	}
	mux := metricsMux(opts)
	srv := &http.Server{
		Addr:              opts.Bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	ln, err := net.Listen("tcp", opts.Bind)
	if err != nil {
		return fmt.Errorf("listen metrics server: %w", err)
	}
	slog.Info("metrics_server_start", "bind", opts.Bind, "auth_mode", opts.AuthMode)
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func metricsMux(opts MetricsServerOptions) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", observeHTTP("/metrics", func(w http.ResponseWriter, r *http.Request) {
		switch opts.AuthMode {
		case "", "none":
		case "static_bearer":
			// Defense in depth: ServeMetrics already rejects this combo at
			// startup, but a caller that builds metricsMux directly must
			// also fail closed rather than treat ConstantTimeCompare("","")
			// as a valid match.
			if opts.BearerToken == "" {
				writeJSONError(w, http.StatusInternalServerError, "metrics auth misconfigured")
				return
			}
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(opts.BearerToken)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		default:
			writeJSONError(w, http.StatusInternalServerError, "invalid metrics auth mode")
			return
		}
		handleMetrics(w, r)
	}))
	return mux
}
