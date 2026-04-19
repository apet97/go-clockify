//go:build grpc

package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/mcp"
	grpctransport "github.com/apet97/go-clockify/internal/transport/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// NewGRPC builds a harness over the gRPC transport using an in-memory
// bufconn listener so tests can run without touching a real TCP socket.
// Only compiled when -tags=grpc is set; a stub factory returning an error
// is provided for default builds.
func NewGRPC(ctx context.Context, opts Options) (Transport, error) {
	srv := buildMockServer(opts)
	lis := bufconn.Listen(1 << 20)

	var auth authn.Authenticator
	bearer := opts.BearerToken
	if bearer != "" {
		a, err := authn.New(authn.Config{Mode: authn.ModeStaticBearer, BearerToken: bearer})
		if err != nil {
			return nil, fmt.Errorf("build authenticator: %w", err)
		}
		auth = a
	}

	maxRecv := int(opts.MaxMessageSize)
	if maxRecv <= 0 {
		maxRecv = 4194304
	}
	srv.MaxMessageSize = int64(maxRecv)

	runCtx, runCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		_ = grpctransport.Serve(runCtx, grpctransport.Options{
			Listener:      lis,
			Server:        srv,
			MaxRecvSize:   maxRecv,
			Authenticator: auth,
		})
		close(done)
	}()

	// Wait briefly for the server goroutine to register the service. In
	// practice this is <5ms; we poll up to 1s before giving up.
	// insecure.NewCredentials() is intentional here: the gRPC connection
	// rides over an in-memory bufconn that never touches the network. A
	// TLS handshake on top of a memory pipe would add no security
	// property and break nothing in the transport harness.
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()), // nosemgrep
		grpc.WithDefaultCallOptions(grpc.ForceCodec(harnessBytesCodec{})),
	)
	if err != nil {
		runCancel()
		_ = lis.Close()
		return nil, fmt.Errorf("grpc NewClient: %w", err)
	}

	h := &grpcHarness{
		opts:      opts,
		srv:       srv,
		conn:      conn,
		lis:       lis,
		runCtx:    runCtx,
		runCancel: runCancel,
		done:      done,
		pending:   map[int]chan Response{},
		notifs:    make(chan Response, 32),
		raws:      make(chan Response, 4),
		bearer:    bearer,
		maxRecv:   int64(maxRecv),
	}

	if err := h.openStream(); err != nil {
		_ = conn.Close()
		runCancel()
		_ = lis.Close()
		return nil, err
	}
	return h, nil
}

type grpcHarness struct {
	opts      Options
	srv       *mcp.Server
	conn      *grpc.ClientConn
	lis       *bufconn.Listener
	runCtx    context.Context
	runCancel context.CancelFunc
	done      chan struct{}

	bearer  string
	maxRecv int64

	stream     grpc.ClientStream
	streamCtx  context.Context
	streamDone context.CancelFunc

	idSeq int64

	mu      sync.Mutex
	pending map[int]chan Response
	notifs  chan Response
	// raws mirrors the stdio harness: the server's parse-error reply
	// to a malformed request has no id, so the pending-map lookup
	// misses and the frame would otherwise be dropped. SendRaw
	// serialises through rawMu and reads from raws.
	raws   chan Response
	rawMu  sync.Mutex
	closed bool
}

func (h *grpcHarness) Name() string { return "grpc" }

// SharedServer exposes the per-harness mcp.Server so parity tests can
// fire server-initiated notifications (tools/list_changed, progress,
// etc.) through the notifier hub. The registered streamNotifier will
// deliver them over the Exchange stream.
func (h *grpcHarness) SharedServer() (*mcp.Server, bool) { return h.srv, true }

func (h *grpcHarness) nextID() int { return int(atomic.AddInt64(&h.idSeq, 1)) }

func (h *grpcHarness) openStream() error {
	ctx, cancel := context.WithCancel(h.runCtx)
	if h.bearer != "" {
		md := metadata.Pairs("authorization", "Bearer "+h.bearer)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	fullMethod := "/" + grpctransport.ServiceName + "/" + grpctransport.ExchangeMethod
	streamDesc := &grpc.StreamDesc{
		StreamName:    grpctransport.ExchangeMethod,
		ServerStreams: true,
		ClientStreams: true,
	}
	stream, err := h.conn.NewStream(ctx, streamDesc, fullMethod)
	if err != nil {
		cancel()
		return fmt.Errorf("open grpc stream: %w", err)
	}
	h.stream = stream
	h.streamCtx = ctx
	h.streamDone = cancel
	go h.readLoop()
	return nil
}

func (h *grpcHarness) readLoop() {
	for {
		var frame []byte
		if err := h.stream.RecvMsg(&frame); err != nil {
			return
		}
		if len(frame) == 0 {
			continue
		}
		var r Response
		if err := json.Unmarshal(frame, &r); err != nil {
			continue
		}
		h.mu.Lock()
		if r.Method != "" && r.ID == 0 {
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
		// Unmatched frame. An anonymous error frame is the parse-error
		// reply to SendRaw — forward it on raws. Other unmatched frames
		// are late duplicates; drop.
		if r.ID == 0 && r.Method == "" && r.Error != nil {
			select {
			case h.raws <- r:
			default:
			}
		}
	}
}

func (h *grpcHarness) sendRequest(id int, method string, params any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	return h.stream.SendMsg(&body)
}

func (h *grpcHarness) call(ctx context.Context, method string, params any) (Response, error) {
	id := h.nextID()
	ch := make(chan Response, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	if err := h.sendRequest(id, method, params); err != nil {
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
		return Response{}, fmt.Errorf("grpc call timeout for method %s", method)
	}
}

func (h *grpcHarness) Initialize(ctx context.Context) (Response, error) {
	return h.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "harness", "version": "test"},
	})
}

func (h *grpcHarness) ListTools(ctx context.Context) (Response, error) {
	return h.call(ctx, "tools/list", map[string]any{})
}

func (h *grpcHarness) CallTool(ctx context.Context, name string, args map[string]any) (Response, error) {
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	return h.call(ctx, "tools/call", params)
}

func (h *grpcHarness) CallToolAsync(_ context.Context, name string, args map[string]any) (int, <-chan Response, error) {
	id := h.nextID()
	ch := make(chan Response, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	params := map[string]any{"name": name}
	if args != nil {
		params["arguments"] = args
	}
	if err := h.sendRequest(id, "tools/call", params); err != nil {
		h.mu.Lock()
		delete(h.pending, id)
		h.mu.Unlock()
		return 0, nil, err
	}
	return id, ch, nil
}

func (h *grpcHarness) Cancel(_ context.Context, requestID int) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params":  map[string]any{"requestId": requestID, "reason": "harness cancel"},
	})
	if err != nil {
		return err
	}
	return h.stream.SendMsg(&body)
}

func (h *grpcHarness) Notifications() <-chan Response { return h.notifs }

// SendRaw pushes bytes through the bidi stream without JSON-wrapping.
// The server's DispatchMessage returns the -32700 envelope when the
// frame fails to unmarshal, and the gRPC transport forwards it on the
// same stream. The harness readLoop routes that anonymous error frame
// onto raws (see the readLoop edit above).
func (h *grpcHarness) SendRaw(ctx context.Context, frame []byte) (Response, error) {
	h.rawMu.Lock()
	defer h.rawMu.Unlock()
	// Drain stale raws from any prior exchange.
	select {
	case <-h.raws:
	default:
	}
	payload := append([]byte{}, frame...)
	if err := h.stream.SendMsg(&payload); err != nil {
		return Response{}, fmt.Errorf("grpc SendRaw: %w", err)
	}
	select {
	case r := <-h.raws:
		return r, nil
	case <-ctx.Done():
		return Response{}, ctx.Err()
	}
}

func (h *grpcHarness) MaxSupportedSize() int64 { return h.maxRecv }

func (h *grpcHarness) Close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	if h.stream != nil {
		_ = h.stream.CloseSend()
	}
	if h.streamDone != nil {
		h.streamDone()
	}
	_ = h.conn.Close()
	h.runCancel()
	_ = h.lis.Close()
	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
	}
	return nil
}

// harnessBytesCodec mirrors the internal grpctransport bytesCodec: raw
// []byte in/out with matching codec name. We duplicate rather than import
// the unexported type so the harness stays test-only and doesn't leak
// into production packages.
type harnessBytesCodec struct{}

func (harnessBytesCodec) Name() string { return "clockify-mcp-bytes" }

func (harnessBytesCodec) Marshal(v any) ([]byte, error) {
	switch p := v.(type) {
	case *[]byte:
		if p == nil {
			return nil, nil
		}
		out := make([]byte, len(*p))
		copy(out, *p)
		return out, nil
	case []byte:
		out := make([]byte, len(p))
		copy(out, p)
		return out, nil
	default:
		return nil, fmt.Errorf("harness bytesCodec: unsupported type %T", v)
	}
}

func (harnessBytesCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("harness bytesCodec: expected *[]byte, got %T", v)
	}
	*p = append((*p)[:0], data...)
	return nil
}
