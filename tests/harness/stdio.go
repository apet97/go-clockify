package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// NewStdio constructs a TransportHarness wrapping an mcp.Server.Run loop
// communicating over an in-memory io.Pipe pair. The harness owns both the
// server goroutine and the response reader; Close terminates both.
//
// Request IDs are monotonic atomic counters; tests that need to pair a
// response with its request use the ID from CallToolAsync.
func NewStdio(ctx context.Context, opts Options) (Transport, error) {
	srv := buildMockServer(opts)

	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()

	h := &stdioHarness{
		opts:    opts,
		clientW: clientW,
		clientR: clientR,
		serverR: serverR,
		serverW: serverW,
		pending: map[int]chan Response{},
		notifs:  make(chan Response, 16),
		done:    make(chan struct{}),
	}
	h.runCtx, h.runCancel = context.WithCancel(ctx)
	go func() {
		_ = srv.Run(h.runCtx, serverR, serverW)
		close(h.done)
	}()
	go h.readLoop()
	return h, nil
}

type stdioHarness struct {
	opts      Options
	clientW   io.WriteCloser
	clientR   io.ReadCloser
	serverR   io.ReadCloser
	serverW   io.WriteCloser
	runCtx    context.Context
	runCancel context.CancelFunc
	done      chan struct{}

	idSeq int64

	mu      sync.Mutex
	pending map[int]chan Response
	notifs  chan Response
	closed  bool
}

func (h *stdioHarness) Name() string { return "stdio" }

func (h *stdioHarness) nextID() int { return int(atomic.AddInt64(&h.idSeq, 1)) }

// readLoop consumes newline-delimited JSON-RPC frames from the server and
// routes each to the correct response channel (or to notifs for
// server-initiated frames).
func (h *stdioHarness) readLoop() {
	scanner := bufio.NewScanner(h.clientR)
	buf := make([]byte, 0, 1<<20)
	scanner.Buffer(buf, 1<<24) // 16 MiB ceiling; payloads should not hit this
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Response
		if err := json.Unmarshal(line, &r); err != nil {
			continue
		}
		h.mu.Lock()
		if r.Method != "" && r.ID == 0 {
			// notification — fan out or drop if buffer full
			select {
			case h.notifs <- r:
			default:
			}
			h.mu.Unlock()
			continue
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
	}
}

func (h *stdioHarness) send(method string, params any) (int, <-chan Response, error) {
	id := h.nextID()
	b, err := encodeRequest(id, method, params)
	if err != nil {
		return 0, nil, err
	}
	ch := make(chan Response, 1)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0, nil, fmt.Errorf("stdio harness closed")
	}
	h.pending[id] = ch
	h.mu.Unlock()
	if _, err := h.clientW.Write(b); err != nil {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
		return 0, nil, err
	}
	return id, ch, nil
}

func (h *stdioHarness) call(ctx context.Context, method string, params any) (Response, error) {
	_, ch, err := h.send(method, params)
	if err != nil {
		return Response{}, err
	}
	select {
	case r := <-ch:
		return r, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
}

func (h *stdioHarness) Initialize(ctx context.Context) (Response, error) {
	return h.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "harness", "version": "test"},
	})
}

func (h *stdioHarness) ListTools(ctx context.Context) (Response, error) {
	return h.call(ctx, "tools/list", map[string]any{})
}

func (h *stdioHarness) CallTool(ctx context.Context, name string, args map[string]any) (Response, error) {
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	return h.call(ctx, "tools/call", params)
}

func (h *stdioHarness) CallToolAsync(_ context.Context, name string, args map[string]any) (int, <-chan Response, error) {
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	return h.send("tools/call", params)
}

func (h *stdioHarness) Cancel(_ context.Context, requestID int) error {
	b, err := encodeNotification("notifications/cancelled", map[string]any{
		"requestId": requestID,
		"reason":    "harness cancel",
	})
	if err != nil {
		return err
	}
	_, err = h.clientW.Write(b)
	return err
}

func (h *stdioHarness) Notifications() <-chan Response { return h.notifs }

func (h *stdioHarness) MaxSupportedSize() int64 {
	if h.opts.MaxMessageSize > 0 {
		return h.opts.MaxMessageSize
	}
	return 4194304
}

func (h *stdioHarness) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	h.runCancel()
	_ = h.clientW.Close()
	_ = h.serverW.Close()
	_ = h.serverR.Close()
	<-h.done
	_ = h.clientR.Close()
	return nil
}
