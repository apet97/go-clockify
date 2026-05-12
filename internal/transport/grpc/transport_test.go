package grpctransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// bufconnDialer returns a grpc.DialOption that routes via the in-memory
// listener so the test does not touch a real TCP port.
func bufconnDialer(lis *bufconn.Listener) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	})
}

// newTestServer builds a minimal mcp.Server with no tools, no enforcement,
// and no activator. It is sufficient to exercise initialize and ping, which
// are handled entirely inside the protocol core.
func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	return mcp.NewServer("test", nil, nil, nil)
}

// newBufconnHarness spins up the gRPC transport on an in-memory listener and
// returns a client stream bound to the Exchange method. The returned cleanup
// stops the grpc.Server gracefully.
func newBufconnHarness(t *testing.T, srv *mcp.Server) (grpc.ClientStream, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	handler := &exchangeServer{srv: srv}
	desc := buildServiceDesc()
	grpcSrv := grpc.NewServer(grpc.ForceServerCodec(bytesCodec{}))
	grpcSrv.RegisterService(&desc, handler)
	go func() { _ = grpcSrv.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		bufconnDialer(lis),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(bytesCodec{})),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}

	streamDesc := &grpc.StreamDesc{
		StreamName:    ExchangeMethod,
		ServerStreams: true,
		ClientStreams: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	fullMethod := "/" + ServiceName + "/" + ExchangeMethod
	stream, err := conn.NewStream(ctx, streamDesc, fullMethod)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	cleanup := func() {
		_ = stream.CloseSend()
		_ = conn.Close()
		grpcSrv.GracefulStop()
		_ = lis.Close()
	}
	return stream, cleanup
}

// TestExchangeInitialize verifies a minimal initialize round-trip through
// the gRPC transport. The request is framed as JSON-RPC, transported as raw
// bytes via the custom codec, dispatched through mcp.Server, and the reply
// read back on the same stream.
func TestExchangeInitialize(t *testing.T) {
	srv := newTestServer(t)
	stream, cleanup := newBufconnHarness(t, srv)
	defer cleanup()

	initPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`)
	if err := stream.SendMsg(&initPayload); err != nil {
		t.Fatalf("send initialize: %v", err)
	}

	var reply []byte
	if err := stream.RecvMsg(&reply); err != nil {
		t.Fatalf("recv initialize reply: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(reply, &parsed); err != nil {
		t.Fatalf("unmarshal reply: %v; raw=%s", err, reply)
	}
	if parsed["jsonrpc"] != "2.0" {
		t.Fatalf("expected jsonrpc=2.0, got %v", parsed["jsonrpc"])
	}
	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T (%v)", parsed["result"], parsed["result"])
	}
	if _, hasCaps := result["capabilities"]; !hasCaps {
		t.Fatalf("initialize result missing capabilities: %v", result)
	}
}

// TestExchangePing confirms a second round-trip on the same stream works and
// that the server treats ping as a no-op success.
func TestExchangePing(t *testing.T) {
	srv := newTestServer(t)
	stream, cleanup := newBufconnHarness(t, srv)
	defer cleanup()

	initPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if err := stream.SendMsg(&initPayload); err != nil {
		t.Fatalf("send initialize: %v", err)
	}
	var initReply []byte
	if err := stream.RecvMsg(&initReply); err != nil {
		t.Fatalf("recv initialize: %v", err)
	}

	pingPayload := []byte(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if err := stream.SendMsg(&pingPayload); err != nil {
		t.Fatalf("send ping: %v", err)
	}
	var pingReply []byte
	if err := stream.RecvMsg(&pingReply); err != nil {
		t.Fatalf("recv ping: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(pingReply, &parsed); err != nil {
		t.Fatalf("unmarshal ping reply: %v; raw=%s", err, pingReply)
	}
	if parsed["error"] != nil {
		t.Fatalf("unexpected error on ping: %v", parsed["error"])
	}
}

// TestExchangeInvalidJSON asserts the transport returns a JSON-RPC parse
// error (-32700) on malformed input without tearing down the stream. The
// stream should remain usable for subsequent valid requests.
func TestExchangeInvalidJSON(t *testing.T) {
	srv := newTestServer(t)
	stream, cleanup := newBufconnHarness(t, srv)
	defer cleanup()

	bad := []byte(`{not json`)
	if err := stream.SendMsg(&bad); err != nil {
		t.Fatalf("send invalid: %v", err)
	}
	var reply []byte
	if err := stream.RecvMsg(&reply); err != nil {
		t.Fatalf("recv parse error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(reply, &parsed); err != nil {
		t.Fatalf("unmarshal parse-error reply: %v; raw=%s", err, reply)
	}
	errObj, ok := parsed["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", parsed)
	}
	if code, _ := errObj["code"].(float64); int(code) != -32700 {
		t.Fatalf("expected code -32700, got %v", errObj["code"])
	}
}

type panicSendStream struct {
	grpc.ServerStream
	ctx     context.Context
	recv    chan []byte
	panicAt int
	sends   int
}

func (s *panicSendStream) SetHeader(metadata.MD) error  { return nil }
func (s *panicSendStream) SendHeader(metadata.MD) error { return nil }
func (s *panicSendStream) SetTrailer(metadata.MD)       {}
func (s *panicSendStream) Context() context.Context     { return s.ctx }

func (s *panicSendStream) SendMsg(any) error {
	s.sends++
	if s.sends == s.panicAt {
		panic("send pump boom")
	}
	return nil
}

func (s *panicSendStream) RecvMsg(m any) error {
	frame, ok := <-s.recv
	if !ok {
		return io.EOF
	}
	dst, ok := m.(*[]byte)
	if !ok {
		return nil
	}
	*dst = append((*dst)[:0], frame...)
	return nil
}

func TestExchangeSendPumpPanicDoesNotDeadlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &panicSendStream{
		ctx:     ctx,
		recv:    make(chan []byte, 1),
		panicAt: 1,
	}
	stream.recv <- []byte(`{not json`)
	close(stream.recv)

	srv := newTestServer(t)
	baseMetric := metrics.GRPCSendPumpPanicsTotal.Get()
	done := make(chan error, 1)
	go func() {
		done <- (&exchangeServer{srv: srv}).Exchange(stream)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Exchange returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Exchange deadlocked after send-pump panic")
	}
	if got := metrics.GRPCSendPumpPanicsTotal.Get() - baseMetric; got != 1 {
		t.Fatalf("send-pump panic metric delta=%d, want 1", got)
	}
}

func TestStreamNotifierDropsSlowConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sends := make(chan []byte, 1)
	sends <- []byte(`{"jsonrpc":"2.0","method":"already_queued"}`)
	notifier := newStreamNotifier(ctx, sends)

	before := metrics.GRPCNotificationDropsTotal.Get("slow_consumer")
	started := time.Now()
	err := notifier.Notify("notifications/tools/list_changed", nil)
	if !errors.Is(err, errGRPCNotificationQueueFull) {
		t.Fatalf("expected slow-consumer drop error, got %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Notify blocked for %s on full send queue", elapsed)
	}
	if got := metrics.GRPCNotificationDropsTotal.Get("slow_consumer") - before; got != 1 {
		t.Fatalf("notification drop metric delta=%d, want 1", got)
	}
}

func TestPlaintextGRPCNonLoopbackDetection(t *testing.T) {
	cases := []struct {
		name string
		addr net.Addr
		want bool
	}{
		{name: "unspecified_ipv4", addr: &net.TCPAddr{IP: net.IPv4zero, Port: 9090}, want: true},
		{name: "unspecified_ipv6", addr: &net.TCPAddr{IP: net.IPv6zero, Port: 9090}, want: true},
		{name: "external", addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.5"), Port: 9090}, want: true},
		{name: "loopback", addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}, want: false},
		{name: "non_tcp", addr: fakeAddr("bufconn"), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := grpcPlaintextNonLoopback(tc.addr); got != tc.want {
				t.Fatalf("grpcPlaintextNonLoopback(%v) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

type fakeAddr string

func (a fakeAddr) Network() string { return string(a) }
func (a fakeAddr) String() string  { return string(a) }

// TestExchangeServeRealListener exercises the public Serve function against
// a real TCP loopback listener to make sure context-driven shutdown works.
// This test is the only one that binds to a local port; it uses :0 for OS
// allocation and closes in <1s.
func TestExchangeServeRealListener(t *testing.T) {
	srv := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	errCh := make(chan error, 1)
	go func() {
		defer wg.Done()
		errCh <- Serve(ctx, Options{Bind: "127.0.0.1:0", Server: srv})
	}()

	// Serve blocks on net.Listen + grpc.Serve; cancel the context and expect
	// a clean shutdown within the drain budget.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != io.EOF {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(12 * time.Second):
		t.Fatalf("Serve did not shut down within 12s after ctx cancel")
	}
	wg.Wait()
}

func TestServeMarksNotReadyBeforeGracefulDrain(t *testing.T) {
	srv := newTestServer(t)
	srv.SetReadyCached(true)
	lis := bufconn.Listen(1024 * 1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{Listener: lis, Server: srv})
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		bufconnDialer(lis),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(bytesCodec{})),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}

	streamDesc := &grpc.StreamDesc{
		StreamName:    ExchangeMethod,
		ServerStreams: true,
		ClientStreams: true,
	}
	streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer streamCancel()
	stream, err := conn.NewStream(streamCtx, streamDesc, "/"+ServiceName+"/"+ExchangeMethod)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("open stream: %v", err)
	}

	cancel()
	deadline := time.After(time.Second)
	for srv.IsReadyCached() {
		select {
		case <-deadline:
			_ = stream.CloseSend()
			_ = conn.Close()
			t.Fatal("server remained ready after shutdown started")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	select {
	case err := <-errCh:
		_ = conn.Close()
		t.Fatalf("Serve returned before active stream drained: %v", err)
	default:
	}

	_ = stream.CloseSend()
	_ = conn.Close()
	select {
	case err := <-errCh:
		if err != nil && err != io.EOF {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not exit after stream closed")
	}
}

// TestServeEnforcesMaxSendMsgSize pins the symmetric send cap added so the
// server cannot stream a response frame larger than MaxRecvSize back to a
// client. Before the cap, gRPC's default MaxSendMsgSize (math.MaxInt32 ≈ 2
// GiB) left the server free to push arbitrarily large notifications or
// tool results across an Exchange stream — a DoS surface that the
// inbound cap could not mitigate. Pinning send at MaxRecvSize closes the
// asymmetry.
//
// Drift check: delete the grpc.MaxSendMsgSize line in transport.go and
// this test passes through to the success path, failing the assertion
// "expected ResourceExhausted, got <result>".
func TestServeEnforcesMaxSendMsgSize(t *testing.T) {
	// MaxRecvSize is set just large enough to admit initialize+tools/call
	// inbound frames but reject the server's oversized response.
	const cap = 4096
	const oversize = cap * 4

	bigPayload := make([]byte, oversize)
	for i := range bigPayload {
		bigPayload[i] = 'x'
	}
	bigTool := mcp.ToolDescriptor{
		Tool: mcp.Tool{
			Name:        "big_tool",
			Description: "Returns a payload larger than the size cap",
			InputSchema: map[string]any{"type": "object"},
		},
		Handler: func(_ context.Context, _ map[string]any) (any, error) {
			return map[string]any{"payload": string(bigPayload)}, nil
		},
	}
	srv := mcp.NewServer("test", []mcp.ToolDescriptor{bigTool}, nil, nil)

	lis := bufconn.Listen(1024 * 1024)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, Options{Listener: lis, Server: srv, MaxRecvSize: cap})
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		bufconnDialer(lis),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(bytesCodec{})),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	defer func() { _ = conn.Close() }()

	streamDesc := &grpc.StreamDesc{
		StreamName:    ExchangeMethod,
		ServerStreams: true,
		ClientStreams: true,
	}
	streamCtx, streamCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer streamCancel()
	stream, err := conn.NewStream(streamCtx, streamDesc, "/"+ServiceName+"/"+ExchangeMethod)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer func() { _ = stream.CloseSend() }()

	initPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`)
	if err := stream.SendMsg(&initPayload); err != nil {
		t.Fatalf("send initialize: %v", err)
	}
	var initReply []byte
	if err := stream.RecvMsg(&initReply); err != nil {
		t.Fatalf("recv initialize reply: %v", err)
	}

	callPayload := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"big_tool","arguments":{}}}`)
	if err := stream.SendMsg(&callPayload); err != nil {
		t.Fatalf("send tools/call: %v", err)
	}

	var reply []byte
	recvErr := stream.RecvMsg(&reply)
	if recvErr == nil {
		// If the cap was not enforced the client would receive the
		// full oversized payload back through the stream.
		t.Fatalf("expected ResourceExhausted from server-side send cap; got success reply (%d bytes)", len(reply))
	}
	// gRPC surfaces send-size violations as ResourceExhausted on the
	// receiver side; we accept any error containing the marker because
	// the exact wrapping varies across grpc-go releases.
	msg := recvErr.Error()
	if !strings.Contains(msg, "ResourceExhausted") &&
		!strings.Contains(msg, "received message larger") {
		t.Fatalf("expected ResourceExhausted-style error, got %v", recvErr)
	}
}
