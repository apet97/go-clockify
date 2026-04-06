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
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// ServeHTTP starts an HTTP server that wraps the MCP server's handle() method.
// It requires a non-empty bearerToken for authentication on the /mcp endpoint.
func (s *Server) ServeHTTP(ctx context.Context, bind, bearerToken string, allowedOrigins []string, maxBodySize int64) error {
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
	mux.Handle("POST /mcp", s.handleMCP(bearerToken, allowedOrigins, maxBodySize))
	// Handle OPTIONS for CORS preflight on /mcp
	mux.Handle("OPTIONS /mcp", s.handleMCP(bearerToken, allowedOrigins, maxBodySize))

	srv := &http.Server{
		Addr:    bind,
		Handler: mux,
	}

	// Graceful shutdown on signal
	shutdownCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-shutdownCtx.Done()
		slog.Info("http_shutdown", "reason", "signal received")
		srv.Shutdown(context.Background())
	}()

	slog.Info("http_start", "bind", bind)
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bind, err)
	}

	// Auto-initialize for HTTP mode
	if !s.initialized {
		s.initialized = true
	}

	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.initialized {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"not_ready"}`))
		return
	}
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleMCP(bearerToken string, allowedOrigins []string, maxBodySize int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. CORS check
		if origin := r.Header.Get("Origin"); origin != "" {
			if !isOriginAllowed(origin, allowedOrigins) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 2. Bearer auth — constant-time comparison
		auth := r.Header.Get("Authorization")
		token := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(bearerToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// 3. Body size limit
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

		// 4. Read and parse JSON-RPC request
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
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
	}
}

func isOriginAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true // no restrictions if not configured
	}
	for _, a := range allowed {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
