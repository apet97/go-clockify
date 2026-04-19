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
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"
)

// InlineMetricsOptions controls whether and how /metrics is exposed on the
// main HTTP listener when using the legacy HTTP transport (MCP_TRANSPORT=http).
//
// The dedicated metrics listener (MCP_METRICS_BIND / ServeMetrics) is the
// preferred enterprise pattern because it separates the metrics scrape surface
// from the MCP API surface. Inline metrics on the main listener require
// explicit operator opt-in via MCP_HTTP_INLINE_METRICS_ENABLED=1 and an
// explicit auth mode — they are disabled by default.
type InlineMetricsOptions struct {
	// Enabled: when false (default), /metrics is not mounted on the main
	// HTTP listener. Set MCP_HTTP_INLINE_METRICS_ENABLED=1 to opt in.
	Enabled bool
	// AuthMode controls auth for the inline /metrics endpoint.
	// "inherit_main_bearer": require the same bearer token as /mcp (default
	//   when Enabled=true and no explicit auth mode is set).
	// "static_bearer": require a separate BearerToken below.
	// "none": unauthenticated — operator must opt in explicitly; startup warns.
	AuthMode string
	// BearerToken is used when AuthMode == "static_bearer".
	BearerToken string
	// MainBearerToken is the /mcp token reused when AuthMode == "inherit_main_bearer".
	// Populated by ServeHTTP from the top-level bearerToken argument.
	MainBearerToken string
}

// ServeHTTP starts an HTTP server that wraps the MCP server's handle() method.
// Auth is delegated to the supplied authn.Authenticator — for legacy
// deployments operators pass a static-bearer authenticator built from
// MCP_BEARER_TOKEN; enterprise deployments can pass OIDC/forward_auth/mTLS
// authenticators through the same seam. bearerToken remains non-empty
// only because inline-metrics inheritance reuses it; nil authenticator +
// empty bearerToken is rejected so the handler never runs unauthenticated.
//
// When allowAnyOrigin is false and allowedOrigins is empty, cross-origin
// requests are rejected (secure default).
//
// inlineMetrics controls whether /metrics is mounted on the main listener.
// The default (InlineMetricsOptions{}) leaves /metrics absent — use the
// dedicated metrics listener (ServeMetrics) for the recommended pattern.
func (s *Server) ServeHTTP(ctx context.Context, bind string, authenticator authn.Authenticator, bearerToken string, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64, inlineMetrics InlineMetricsOptions) error {
	if bind == "" {
		bind = ":8080"
	}
	ln, err := net.Listen("tcp", bind)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bind, err)
	}
	return s.serveHTTP(ctx, ln, authenticator, bearerToken, allowedOrigins, allowAnyOrigin, maxBodySize, inlineMetrics)
}

// ServeHTTPListener is the listener-injection counterpart of ServeHTTP. It
// takes ownership of an already-open net.Listener (callers should use
// net.Listen("tcp", "127.0.0.1:0") to get an ephemeral port) and runs the
// same middleware stack until ctx is cancelled. Primarily for tests that
// need to know the bound port before the server starts accepting.
func (s *Server) ServeHTTPListener(ctx context.Context, ln net.Listener, authenticator authn.Authenticator, bearerToken string, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64, inlineMetrics InlineMetricsOptions) error {
	if ln == nil {
		return fmt.Errorf("ServeHTTPListener: listener must not be nil")
	}
	return s.serveHTTP(ctx, ln, authenticator, bearerToken, allowedOrigins, allowAnyOrigin, maxBodySize, inlineMetrics)
}

// serveHTTP is the shared implementation for ServeHTTP and ServeHTTPListener.
// It assumes ln is already open; the public wrappers own the net.Listen
// decision (or lack thereof).
func (s *Server) serveHTTP(ctx context.Context, ln net.Listener, authenticator authn.Authenticator, bearerToken string, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64, inlineMetrics InlineMetricsOptions) error {
	if authenticator == nil {
		if bearerToken == "" {
			return fmt.Errorf("MCP_BEARER_TOKEN is required for HTTP transport when no authenticator is supplied")
		}
		auth, err := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: bearerToken})
		if err != nil {
			return fmt.Errorf("build static-bearer authenticator: %w", err)
		}
		authenticator = auth
	}
	if maxBodySize <= 0 {
		maxBodySize = 2097152 // 2 MB
	}
	// Wire the main bearer token for inherit_main_bearer so the handler
	// resolves to the right secret without exposing it in InlineMetricsOptions
	// before this call. Empty bearerToken here means the operator runs
	// OIDC/forward_auth/mTLS; inherit_main_bearer is not meaningful in
	// that mode and the caller is responsible for choosing a different
	// metrics AuthMode.
	inlineMetrics.MainBearerToken = bearerToken

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", observeHTTP("/health", s.handleHealth))
	mux.HandleFunc("GET /ready", observeHTTP("/ready", s.handleReady))
	if inlineMetrics.Enabled {
		mux.HandleFunc("GET /metrics", observeHTTP("/metrics", inlineMetricsHandler(inlineMetrics)))
	}
	mcpHandler := s.handleMCP(authenticator, allowedOrigins, allowAnyOrigin, maxBodySize)
	mux.Handle("POST /mcp", observeHTTPH("/mcp", mcpHandler))
	// Handle OPTIONS for CORS preflight on /mcp
	mux.Handle("OPTIONS /mcp", observeHTTPH("/mcp", mcpHandler))
	// Mount opt-in extras (e.g. /debug/pprof/* under -tags=pprof). nil slice
	// is a no-op so default builds are byte-identical.
	mountExtras(mux, s.ExtraHTTPHandlers)

	srv := &http.Server{
		Addr:              ln.Addr().String(),
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
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("http_shutdown_error", "error", err)
		}
	}()

	slog.Info("http_start", "bind", ln.Addr().String())

	// Install the legacy-POST notification sink so activation events
	// (tools/list_changed) are visibly dropped and counted instead of
	// silently vanishing into a nil encoder. Real server→client streaming
	// requires the Streamable HTTP (2025-03-26) transport rewrite.
	if s.hub.len() == 0 {
		s.SetNotifier(droppingNotifier{})
	}
	s.advertiseListChanged.Store(false)

	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleMetrics writes the Prometheus text format registry. Used by both the
// dedicated metrics listener (ServeMetrics) and the inline handler below.
// Auth is enforced by the callers, not here.
func handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = metrics.Default.WriteTo(w)
}

// inlineMetricsHandler returns an http.HandlerFunc that serves /metrics on
// the main HTTP listener according to the supplied InlineMetricsOptions auth
// mode. It is only installed when InlineMetricsOptions.Enabled is true.
//
// Auth mode behaviour:
//   - "inherit_main_bearer": require the main /mcp bearer token (opts.MainBearerToken).
//   - "static_bearer": require opts.BearerToken.
//   - "none": unauthenticated. Operator must opt in explicitly; main.go emits
//     a startup warning when this mode is active.
func inlineMetricsHandler(opts InlineMetricsOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch opts.AuthMode {
		case "inherit_main_bearer":
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(opts.MainBearerToken)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		case "static_bearer":
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(opts.BearerToken)) != 1 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		case "none":
			// Deliberately unauthenticated. Operator opted in explicitly.
		default:
			writeJSONError(w, http.StatusInternalServerError, "invalid inline metrics auth mode")
			return
		}
		handleMetrics(w, r)
	}
}

// statusRecorder captures the response status code so middleware can observe
// it after the handler has written. Defaults to 200 per net/http semantics.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

// observeHTTP wraps a HandlerFunc with metrics and panic recovery, recording
// both the HTTPRequestsTotal counter and HTTPRequestDuration histogram against
// a fixed, bounded path label. Use this for every mux route so /metrics
// cardinality stays predictable regardless of probe traffic.
func observeHTTP(path string, fn http.HandlerFunc) http.HandlerFunc {
	return observeHTTPH(path, http.HandlerFunc(fn))
}

// observeHTTPH is the Handler form of observeHTTP for already-constructed handlers.
func observeHTTPH(path string, h http.Handler) http.HandlerFunc {
	return func(rawW http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w := &statusRecorder{ResponseWriter: rawW}
		defer func() {
			if rec := recover(); rec != nil {
				metrics.PanicsRecoveredTotal.Inc("http")
				slog.Error("panic_recovered",
					"site", "http",
					"path", path,
					"method", r.Method,
					"panic", fmtAny(rec),
					"stack", string(debug.Stack()),
				)
				if w.status == 0 {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal server error"}`))
				}
			}
			status := w.status
			if status == 0 {
				status = http.StatusOK
			}
			statusStr := strconv.Itoa(status)
			metrics.HTTPRequestsTotal.Inc(path, r.Method, statusStr)
			metrics.HTTPRequestDuration.Observe(time.Since(start).Seconds(), path, r.Method, statusStr)
		}()
		h.ServeHTTP(w, r)
	}
}

// fmtAny safely stringifies a panic value without importing fmt in hot paths.
func fmtAny(v any) string {
	switch x := v.(type) {
	case error:
		return x.Error()
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"version": s.Version,
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if s.ReadyChecker != nil {
		if err := s.checkReady(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "not_ready", "reason": err.Error()})
			return
		}
	}
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

const readyCacheTTL = 15 * time.Second

func (s *Server) checkReady(ctx context.Context) error {
	s.readyMu.Lock()
	defer s.readyMu.Unlock()

	if time.Since(s.readyAt) < readyCacheTTL {
		if s.readyCached {
			return nil
		}
		return fmt.Errorf("upstream unhealthy (cached)")
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := s.ReadyChecker(checkCtx)
	s.readyAt = time.Now()
	s.readyCached = err == nil
	return err
}

func (s *Server) handleMCP(authenticator authn.Authenticator, allowedOrigins []string, allowAnyOrigin bool, maxBodySize int64) http.HandlerFunc {
	if s.hub.len() == 0 {
		s.SetNotifier(droppingNotifier{})
	}
	s.advertiseListChanged.Store(false)
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := s.requestSeq.Add(1)

		// Security headers on all responses. HSTS/CSP/Referrer-Policy are
		// emitted alongside the legacy nosniff+no-store pair for defense in
		// depth; operators running without a reverse proxy still get the
		// baseline modern browser hardening.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "()")

		// DNS rebinding protection: when strict host checking is enabled,
		// reject Host headers that don't match the configured origin
		// allowlist. This complements the Origin check for clients that
		// omit Origin and is specifically the attack vector DNS rebinding
		// uses against loopback-bound services.
		if s.StrictHostCheck && !isHostAllowed(r.Host, allowedOrigins, allowAnyOrigin) {
			writeJSONError(w, http.StatusForbidden, "host not allowed")
			slog.Warn("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"host", r.Host,
				"status", 403,
				"reason", "host_rejected",
				"req_id", reqID,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return
		}

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
				w.Header().Set("Vary", "Origin")
			}
		}

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		// 2. Auth — delegate to the supplied authn.Authenticator. This
		// matches the streamable_http transport and unlocks OIDC /
		// forward_auth / mTLS on the legacy HTTP path. Static-bearer
		// deployments still reach constant-time compare via the
		// staticBearerAuthenticator branch.
		principal, err := authenticator.Authenticate(r.Context(), r)
		if err != nil {
			authn.WriteUnauthorized(w, "invalid_token", err.Error())
			slog.Warn("http_request", "method", r.Method, "path", r.URL.Path, "status", 401, "reason", "auth_failed", "req_id", reqID, "duration_ms", time.Since(start).Milliseconds())
			return
		}
		r = r.WithContext(authn.WithPrincipal(r.Context(), &principal))

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
			_ = json.NewEncoder(w).Encode(Response{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: -32700, Message: "invalid JSON"},
			})
			return
		}
		if rpcErr := validateRequest(req); rpcErr != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   rpcErr,
			})
			return
		}

		// 5. Handle using existing server logic
		resp := s.handle(r.Context(), req)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)

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
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
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

// isHostAllowed protects against DNS rebinding attacks when the operator has
// opted into strict mode. In strict mode the Host header must match either a
// loopback literal or the host component of one of the configured allowed
// origins. AllowAnyOrigin disables the check entirely.
//
// Loopback is always accepted regardless of allowlist so curl
// 127.0.0.1:8080 smoke tests continue to work.
func isHostAllowed(host string, allowed []string, allowAnyOrigin bool) bool {
	if allowAnyOrigin {
		return true
	}
	if host == "" {
		return false
	}
	h := canonicalHost(host)
	switch strings.ToLower(h) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	for _, a := range allowed {
		// allowed entries look like "https://example.com"; extract host.
		ah := a
		if i := strings.Index(ah, "://"); i >= 0 {
			ah = ah[i+3:]
		}
		if i := strings.IndexAny(ah, "/?#"); i >= 0 {
			ah = ah[:i]
		}
		if strings.EqualFold(canonicalHost(ah), h) {
			return true
		}
	}
	return false
}

func canonicalHost(host string) string {
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(parsedHost, "[]")
	}
	return strings.Trim(host, "[]")
}
