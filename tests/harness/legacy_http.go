package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
)

// NewLegacyHTTP builds a harness over the legacy POST-only /mcp transport
// (mcp.Server.ServeHTTPListener). Each call is a single HTTP POST; there is
// no server→client notification channel, so Notifications() returns nil.
func NewLegacyHTTP(ctx context.Context, opts Options) (Transport, error) {
	srv := buildMockServer(opts)
	bearer := opts.BearerToken
	if bearer == "" {
		bearer = "legacy-http-harness-token"
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	maxBody := opts.MaxMessageSize
	if maxBody <= 0 {
		maxBody = 4194304
	}

	h := &legacyHTTPHarness{
		opts:   opts,
		bearer: bearer,
		ln:     ln,
		url:    "http://" + ln.Addr().String() + "/mcp",
		client: &http.Client{Timeout: 10 * time.Second},
		done:   make(chan struct{}),
	}
	h.runCtx, h.runCancel = context.WithCancel(ctx)

	auth, err := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: bearer})
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("build authenticator: %w", err)
	}

	go func() {
		_ = srv.ServeHTTPListener(h.runCtx, ln, auth, bearer, nil, true, maxBody, mcp.InlineMetricsOptions{})
		close(h.done)
	}()

	// Wait for /health to answer so Initialize doesn't race the goroutine.
	healthURL := "http://" + ln.Addr().String() + "/health"
	if err := WaitForHTTP200(ctx, healthURL); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("legacy http not ready: %w", err)
	}
	return h, nil
}

type legacyHTTPHarness struct {
	opts      Options
	bearer    string
	ln        net.Listener
	url       string
	client    *http.Client
	runCtx    context.Context
	runCancel context.CancelFunc
	done      chan struct{}
	idSeq     int64
}

func (h *legacyHTTPHarness) Name() string { return "legacy_http" }

func (h *legacyHTTPHarness) nextID() int { return int(atomic.AddInt64(&h.idSeq, 1)) }

func (h *legacyHTTPHarness) do(ctx context.Context, method string, params any) (Response, error) {
	id := h.nextID()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.bearer)
	resp, err := h.client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return Response{Error: &RPCError{Code: -32001, Message: "request body too large"}}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Response{Error: &RPCError{Code: -32603, Message: fmt.Sprintf("http status %d", resp.StatusCode)}}, nil
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, err
	}
	return out, nil
}

func (h *legacyHTTPHarness) Initialize(ctx context.Context) (Response, error) {
	return h.do(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "harness", "version": "test"},
	})
}

func (h *legacyHTTPHarness) ListTools(ctx context.Context) (Response, error) {
	return h.do(ctx, "tools/list", map[string]any{})
}

func (h *legacyHTTPHarness) CallTool(ctx context.Context, name string, args map[string]any) (Response, error) {
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	return h.do(ctx, "tools/call", params)
}

func (h *legacyHTTPHarness) CallToolAsync(ctx context.Context, name string, args map[string]any) (int, <-chan Response, error) {
	// Legacy HTTP is request/response; wrap the synchronous call in a goroutine
	// so cancel-parity tests have the same shape as streamable/stdio even though
	// Cancel is a no-op here.
	id := h.nextID()
	ch := make(chan Response, 1)
	go func() {
		resp, err := h.do(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
		if err != nil {
			resp = Response{Error: &RPCError{Code: -32603, Message: err.Error()}}
		}
		resp.ID = id
		ch <- resp
		close(ch)
	}()
	return id, ch, nil
}

// Cancel is a no-op on legacy HTTP. The spec has no cancellation channel on
// a single-shot POST; callers can still cancel the outbound ctx, which
// surfaces as ctx.Err() at the client side.
func (h *legacyHTTPHarness) Cancel(_ context.Context, _ int) error { return nil }

func (h *legacyHTTPHarness) Notifications() <-chan Response { return nil }

func (h *legacyHTTPHarness) MaxSupportedSize() int64 {
	if h.opts.MaxMessageSize > 0 {
		return h.opts.MaxMessageSize
	}
	return 4194304
}

func (h *legacyHTTPHarness) Close() error {
	h.runCancel()
	_ = h.ln.Close()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
	}
	return nil
}
