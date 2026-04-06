package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// ServeHTTP starts an HTTP server that wraps the MCP server's handle() method.
// It requires a non-empty bearerToken for authentication on the /mcp endpoint.
// When allowAnyOrigin is false and allowedOrigins is empty, cross-origin requests
// are rejected (secure default).
func (s *Server) ServeHTTP(ctx context.Context, bind, bearerToken string, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64) error {
	if bearerToken == "" {
		return fmt.Errorf("MCP_BEARER_TOKEN is required for HTTP transport")
	}
	if bind == "" {
		bind = ":8080"
	}
	if maxBodySize <= 0 {
		maxBodySize = 2097152 // 2 MB
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.Handle("POST /mcp", s.handleMCP(bearerToken, allowedOrigins, allowAnyOrigin, maxBodySize))
	// Handle OPTIONS for CORS preflight on /mcp
	mux.Handle("OPTIONS /mcp", s.handleMCP(bearerToken, allowedOrigins, allowAnyOrigin, maxBodySize))

	srv := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Use the provided ctx for shutdown — no duplicate signal registration.
	// The caller (main.go) is responsible for signal handling on ctx.
	go func() {
		<-ctx.Done()
		slog.Info("http_shutdown", "reason", "context cancelled")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	slog.Info("http_start", "bind", bind)
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bind, err)
	}

	// Auto-initialize for HTTP mode
	if !s.initialized.Load() {
		s.initialized.Store(true)
	}

	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if !s.initialized.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
		return
	}
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleMCP(bearerToken string, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := s.requestSeq.Add(1)

		// Security headers on all responses
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")

		// 1. CORS check
		if origin := r.Header.Get("Origin"); origin != "" {
			if !isOriginAllowed(origin, allowedOrigins, allowAnyOrigin) {
				writeJSONError(w, http.StatusForbidden, "origin not allowed")
				slog.Warn("http_request", "method", r.Method, "path", r.URL.Path, "status", 403, "reason", "cors_rejected", "req_id", reqID, "duration_ms", time.Since(start).Milliseconds())
				return
			}
			if allowAnyOrigin {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// 2. Bearer auth — constant-time comparison
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			slog.Warn("http_request", "method", r.Method, "path", r.URL.Path, "status", 401, "reason", "missing_bearer_prefix", "req_id", reqID, "duration_ms", time.Since(start).Milliseconds())
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(bearerToken)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			slog.Warn("http_request", "method", r.Method, "path", r.URL.Path, "status", 401, "req_id", reqID, "duration_ms", time.Since(start).Milliseconds())
			return
		}

		// 3. Body size limit
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		// 4. Read and parse JSON-RPC request
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request too large")
			return
		}

		var req Request
		if err := json.Unmarshal(body, &req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "invalid JSON"},
			})
			return
		}

		// 5. Handle using existing server logic
		resp := s.handle(r.Context(), req)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

		// 6. Structured access log
		slog.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"rpc_method", req.Method,
			"status", 200,
			"req_id", reqID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// writeJSONError sends an error response as JSON instead of text/plain.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isOriginAllowed(origin string, allowed []string, allowAnyOrigin bool) bool {
	if allowAnyOrigin {
		return true
	}
	if len(allowed) == 0 {
		return false // reject by default when no origins configured
	}
	for _, a := range allowed {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
