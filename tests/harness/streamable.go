package harness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/controlplane"
	"github.com/apet97/go-clockify/internal/mcp"
)

// NewStreamable builds a harness over the MCP Streamable HTTP 2025-03-26
// transport. POST /mcp delivers requests; GET /mcp opens the long-lived SSE
// stream on which the server emits responses and notifications.
//
// This adapter:
//   - opens an ephemeral TCP listener (127.0.0.1:0)
//   - creates a memory control-plane store
//   - establishes an initialize round-trip to capture the session ID
//   - opens the SSE subscription
//   - parses SSE frames into Response values and routes them by request ID
//     (or onto the notifications channel for server-initiated frames)
func NewStreamable(ctx context.Context, opts Options) (Transport, error) {
	srv := buildMockServer(opts)
	bearer := opts.BearerToken
	if bearer == "" {
		bearer = "streamable-harness-token-long-enough"
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	addr := ln.Addr().String()

	store, err := controlplane.Open("memory")
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("control-plane open: %w", err)
	}

	auth, err := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: bearer})
	if err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("build authenticator: %w", err)
	}

	maxBody := opts.MaxMessageSize
	if maxBody <= 0 {
		maxBody = 4194304
	}

	h := &streamableHarness{
		opts:    opts,
		srv:     srv,
		bearer:  bearer,
		ln:      ln,
		addr:    addr,
		url:     "http://" + addr,
		client:  &http.Client{Timeout: 10 * time.Second},
		pending: map[int]chan Response{},
		notifs:  make(chan Response, 32),
		done:    make(chan struct{}),
	}
	h.runCtx, h.runCancel = context.WithCancel(ctx)

	go func() {
		_ = mcp.ServeStreamableHTTP(h.runCtx, mcp.StreamableHTTPOptions{
			Listener:      ln,
			MaxBodySize:   maxBody,
			Authenticator: auth,
			ControlPlane:  store,
			SessionTTL:    30 * time.Minute,
			Factory: func(_ context.Context, _ authn.Principal, _ string) (*mcp.StreamableSessionRuntime, error) {
				return &mcp.StreamableSessionRuntime{Server: srv}, nil
			},
		})
		close(h.done)
	}()

	healthURL := "http://" + addr + "/health"
	if err := WaitForHTTP200(ctx, healthURL); err != nil {
		_ = h.Close()
		return nil, fmt.Errorf("streamable http not ready: %w", err)
	}
	return h, nil
}

type streamableHarness struct {
	opts      Options
	srv       *mcp.Server
	bearer    string
	ln        net.Listener
	addr      string
	url       string
	client    *http.Client
	runCtx    context.Context
	runCancel context.CancelFunc
	done      chan struct{}

	sessionID string
	sseResp   *http.Response
	sseCancel context.CancelFunc

	idSeq int64

	mu      sync.Mutex
	pending map[int]chan Response
	notifs  chan Response
	closed  bool
}

func (h *streamableHarness) Name() string { return "streamable_http" }

func (h *streamableHarness) SharedServer() (*mcp.Server, bool) { return h.srv, true }

func (h *streamableHarness) nextID() int { return int(atomic.AddInt64(&h.idSeq, 1)) }

func (h *streamableHarness) Initialize(ctx context.Context) (Response, error) {
	id := h.nextID()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "harness", "version": "test"},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url+"/mcp", bytes.NewReader(body))
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
	if sid := resp.Header.Get(mcp.MCPSessionIDHeader); sid != "" {
		h.sessionID = sid
	} else {
		return Response{}, fmt.Errorf("initialize: no %s header", mcp.MCPSessionIDHeader)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{Error: &RPCError{Code: -32603, Message: fmt.Sprintf("http status %d", resp.StatusCode)}}, nil
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, err
	}
	if err := h.openSSE(); err != nil {
		return Response{}, fmt.Errorf("open SSE: %w", err)
	}
	return out, nil
}

// openSSE establishes the GET /mcp long-lived stream and kicks off the
// frame parser. Must be called after Initialize has learned the session ID.
func (h *streamableHarness) openSSE() error {
	sseCtx, cancel := context.WithCancel(h.runCtx)
	req, err := http.NewRequestWithContext(sseCtx, http.MethodGet, h.url+"/mcp", nil)
	if err != nil {
		cancel()
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+h.bearer)
	req.Header.Set(mcp.MCPSessionIDHeader, h.sessionID)
	// No timeout on the SSE client — the underlying response body is a
	// long-lived stream. client.Timeout would kill the stream prematurely.
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		cancel()
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		return fmt.Errorf("sse open: status %d body=%s", resp.StatusCode, string(body))
	}
	h.sseResp = resp
	h.sseCancel = cancel
	go h.parseSSE(resp)
	return nil
}

// parseSSE consumes `event:` / `data:` frame pairs from the SSE stream.
// Response IDs trigger pending-channel fan-out; notifications go to h.notifs.
func (h *streamableHarness) parseSSE(resp *http.Response) {
	defer func() { _ = resp.Body.Close() }()
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 1<<24)
	var eventName string
	var dataBuf bytes.Buffer
	flush := func() {
		if dataBuf.Len() == 0 {
			eventName = ""
			return
		}
		data := bytes.TrimRight(dataBuf.Bytes(), "\n")
		dataBuf.Reset()
		var r Response
		if err := json.Unmarshal(data, &r); err != nil {
			eventName = ""
			return
		}
		h.mu.Lock()
		if r.Method != "" && r.ID == 0 {
			select {
			case h.notifs <- r:
			default:
			}
			h.mu.Unlock()
			eventName = ""
			return
		}
		ch, ok := h.pending[r.ID]
		if ok {
			delete(h.pending, r.ID)
		}
		h.mu.Unlock()
		if ok {
			ch <- r
			close(ch)
		}
		eventName = ""
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, ":"):
			// keepalive / comment
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			_ = eventName
		case strings.HasPrefix(line, "data:"):
			dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			dataBuf.WriteByte('\n')
		}
	}
	// Stream ended; flush any trailing buffered event.
	flush()
}

// sendRequest POSTs a JSON-RPC request and, for streamable HTTP, decodes
// the response synchronously from the POST body. The MCP Streamable HTTP
// 2025-03-26 spec delivers tool/method responses on the POST itself;
// the SSE GET stream carries only server→client notifications. This
// differs from a naive "everything flows through SSE" assumption and
// was the cause of an earlier tools/list timeout.
func (h *streamableHarness) sendRequest(ctx context.Context, id int, method string, params any) error {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url+"/mcp", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.bearer)
	req.Header.Set(mcp.MCPSessionIDHeader, h.sessionID)
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		h.deliver(id, Response{ID: id, Error: &RPCError{Code: -32001, Message: "request body too large"}})
		return nil
	}
	if resp.StatusCode >= 400 {
		_, _ = io.ReadAll(resp.Body)
		return fmt.Errorf("http status %d", resp.StatusCode)
	}
	// Decode the POST response body. Some streamable deployments return
	// 202 Accepted with the response routed through SSE instead; in that
	// case the body is empty and the SSE reader will deliver the result.
	if resp.StatusCode == http.StatusAccepted {
		return nil
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if err == io.EOF {
			// Empty body — response will arrive on the SSE stream.
			return nil
		}
		return fmt.Errorf("decode streamable response: %w", err)
	}
	h.deliver(id, out)
	return nil
}

// deliver routes a Response to the matching pending channel; no-op if
// nothing is waiting on that ID (which can happen on late duplicates).
func (h *streamableHarness) deliver(id int, resp Response) {
	h.mu.Lock()
	ch, ok := h.pending[id]
	if ok {
		delete(h.pending, id)
	}
	h.mu.Unlock()
	if ok {
		if resp.ID == 0 {
			resp.ID = id
		}
		ch <- resp
		close(ch)
	}
}

func (h *streamableHarness) call(ctx context.Context, method string, params any) (Response, error) {
	id := h.nextID()
	ch := make(chan Response, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	if err := h.sendRequest(ctx, id, method, params); err != nil {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
		return Response{}, err
	}
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return Response{}, fmt.Errorf("streamable call timeout for method %s", method)
	}
}

func (h *streamableHarness) ListTools(ctx context.Context) (Response, error) {
	return h.call(ctx, "tools/list", map[string]any{})
}

func (h *streamableHarness) CallTool(ctx context.Context, name string, args map[string]any) (Response, error) {
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	return h.call(ctx, "tools/call", params)
}

// CallToolAsync fires the POST in a goroutine and returns immediately.
// Cancellation tests rely on this shape: they need to issue the in-flight
// request, then send a separate cancel POST on a different connection
// before the first POST returns. Running sendRequest synchronously would
// block the caller for the full handler duration — exactly what we want
// to cancel.
func (h *streamableHarness) CallToolAsync(ctx context.Context, name string, args map[string]any) (int, <-chan Response, error) {
	id := h.nextID()
	ch := make(chan Response, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	go func() {
		if err := h.sendRequest(ctx, id, "tools/call", params); err != nil {
			h.deliver(id, Response{ID: id, Error: &RPCError{Code: -32603, Message: err.Error()}})
		}
	}()
	return id, ch, nil
}

func (h *streamableHarness) Cancel(ctx context.Context, requestID int) error {
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params":  map[string]any{"requestId": requestID, "reason": "harness cancel"},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url+"/mcp", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.bearer)
	req.Header.Set(mcp.MCPSessionIDHeader, h.sessionID)
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

func (h *streamableHarness) Notifications() <-chan Response { return h.notifs }

// SendRaw POSTs raw bytes to /mcp carrying the established session ID.
// The server's parse-error handler runs after auth but before session
// lookup (see transport_streamable_http.go), so a malformed frame
// returns 200 with code=-32700 on the POST body. 413 follows the same
// mapping as ordinary requests.
func (h *streamableHarness) SendRaw(ctx context.Context, frame []byte) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url+"/mcp", bytes.NewReader(frame))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.bearer)
	if h.sessionID != "" {
		req.Header.Set(mcp.MCPSessionIDHeader, h.sessionID)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusRequestEntityTooLarge {
		return Response{Error: &RPCError{Code: -32001, Message: "request body too large"}}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return Response{Error: &RPCError{Code: -32603, Message: fmt.Sprintf("http status %d body=%s", resp.StatusCode, string(body))}}, nil
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("decode streamable SendRaw response: %w", err)
	}
	return out, nil
}

func (h *streamableHarness) MaxSupportedSize() int64 {
	if h.opts.MaxMessageSize > 0 {
		return h.opts.MaxMessageSize
	}
	return 4194304
}

func (h *streamableHarness) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	if h.sseCancel != nil {
		h.sseCancel()
	}
	if h.sseResp != nil {
		_ = h.sseResp.Body.Close()
	}
	h.runCancel()
	_ = h.ln.Close()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
	}
	return nil
}
