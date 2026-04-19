package harness

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/apet97/go-clockify/internal/mcp"
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
		srv:     srv,
		clientW: clientW,
		clientR: clientR,
		serverR: serverR,
		serverW: serverW,
		pending: map[int]chan Response{},
		notifs:  make(chan Response, 16),
		raws:    make(chan Response, 4),
		done:    make(chan struct{}),
	}
	h.runCtx, h.runCancel = context.WithCancel(ctx)
	go func() {
		_ = srv.Run(h.runCtx, serverR, serverW)
		// When Run exits (ctx cancelled, or scanner failed on an
		// oversize frame), close both pipe halves so the client side
		// unblocks: the readLoop sees EOF and drains pending channels
		// with a transport-down error, and any pending Write on
		// clientW fails with io.ErrClosedPipe instead of hanging.
		_ = serverW.Close()
		_ = serverR.CloseWithError(errTransportDown)
		close(h.done)
	}()
	go h.readLoop()
	return h, nil
}

// errTransportDown is the sentinel error returned by the server-side
// pipe reader after Run exits, so writes on the client side unblock
// with a distinguishable error rather than a generic io.ErrClosedPipe.
var errTransportDown = fmt.Errorf("stdio transport shut down (possibly oversize frame)")

type stdioHarness struct {
	opts      Options
	srv       *mcp.Server
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
	// raws receives anonymous server responses (no id, no method —
	// typically the -32700 parse-error frame a malformed request
	// triggers). SendRaw holds rawMu for the duration of a raw
	// exchange and reads from this channel.
	raws   chan Response
	rawMu  sync.Mutex
	closed bool
}

func (h *stdioHarness) Name() string { return "stdio" }

func (h *stdioHarness) SharedServer() (*mcp.Server, bool) { return h.srv, true }

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
			continue
		}
		// No pending match. An anonymous error frame (no id, Error set)
		// is the parse-error reply to a SendRaw; forward it on raws so
		// the caller can assert on it. Other unmatched frames are late
		// duplicates and silently dropped to preserve prior behaviour.
		if r.ID == 0 && r.Method == "" && r.Error != nil {
			select {
			case h.raws <- r:
			default:
			}
		}
	}
	// Scanner exited — server closed or errored (e.g. on oversize frame).
	// Flush every pending channel with a synthetic transport-down error so
	// callers see "something went wrong" rather than hanging until ctx.
	h.mu.Lock()
	for id, ch := range h.pending {
		delete(h.pending, id)
		err := scanner.Err()
		msg := "stdio transport closed"
		if err != nil {
			msg = "stdio transport error: " + err.Error()
		}
		ch <- Response{ID: id, Error: &RPCError{Code: -32603, Message: msg}}
		close(ch)
	}
	h.mu.Unlock()
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
	// Run the write in a goroutine so an unbounded-blocking Pipe write
	// (e.g. when the server goroutine has already exited on an oversize
	// frame error) surfaces as a transport error in the response channel
	// after the run context expires, instead of deadlocking the caller.
	go func() {
		if _, werr := h.clientW.Write(b); werr != nil {
			h.mu.Lock()
			ch2, ok := h.pending[id]
			if ok {
				delete(h.pending, id)
			}
			h.mu.Unlock()
			if ok {
				ch2 <- Response{ID: id, Error: &RPCError{Code: -32603, Message: "stdio write failed: " + werr.Error()}}
				close(ch2)
			}
		}
	}()
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

// SendRaw writes raw bytes (plus a trailing newline for line-delimited
// framing) to the server and waits for the server's anonymous error
// reply. Only one SendRaw runs at a time per harness; concurrent calls
// serialise on rawMu. Pending matched responses are unaffected.
func (h *stdioHarness) SendRaw(ctx context.Context, frame []byte) (Response, error) {
	h.rawMu.Lock()
	defer h.rawMu.Unlock()
	// Drain any stale raws from prior exchanges before writing.
	select {
	case <-h.raws:
	default:
	}
	buf := frame
	if len(buf) == 0 || buf[len(buf)-1] != '\n' {
		buf = append(append([]byte{}, frame...), '\n')
	}
	if _, err := h.clientW.Write(buf); err != nil {
		return Response{}, fmt.Errorf("stdio SendRaw write: %w", err)
	}
	select {
	case r := <-h.raws:
		return r, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
}

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
