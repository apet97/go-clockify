package grpctransport

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
