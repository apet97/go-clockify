package grpctransport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// stubResourceProvider returns the bare minimum needed to enable the
// resources capability on the test server so resources/subscribe is not
// rejected with -32601. ListResources / Read can return empty since the
// scope test never exercises those paths.
type stubResourceProvider struct{}

func (stubResourceProvider) ListResources(_ context.Context) ([]mcp.Resource, error) {
	return nil, nil
}
func (stubResourceProvider) ListResourceTemplates(_ context.Context) ([]mcp.ResourceTemplate, error) {
	return nil, nil
}
func (stubResourceProvider) ReadResource(_ context.Context, _ string) ([]mcp.ResourceContents, error) {
	return nil, nil
}

// twoStreamHarness opens two Exchange streams against one shared
// *mcp.Server so the test can exercise the per-notifier subscription
// scope through the real gRPC code path (AddNotifier + ctx wrap).
func twoStreamHarness(t *testing.T, srv *mcp.Server) (a, b grpc.ClientStream, cleanup func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	handler := &exchangeServer{srv: srv}
	desc := buildServiceDesc()
	grpcSrv := grpc.NewServer(grpc.ForceServerCodec(bytesCodec{}))
	grpcSrv.RegisterService(&desc, handler)
	go func() { _ = grpcSrv.Serve(lis) }()

	open := func() (grpc.ClientStream, *grpc.ClientConn) {
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
		stream, err := conn.NewStream(ctx, streamDesc, "/"+ServiceName+"/"+ExchangeMethod)
		if err != nil {
			t.Fatalf("open stream: %v", err)
		}
		return stream, conn
	}

	streamA, connA := open()
	streamB, connB := open()

	cleanup = func() {
		_ = streamA.CloseSend()
		_ = streamB.CloseSend()
		_ = connA.Close()
		_ = connB.Close()
		grpcSrv.GracefulStop()
		_ = lis.Close()
	}
	return streamA, streamB, cleanup
}

// initialize sends initialize and reads back the reply so the server
// passes the requiresInitialized guard for subsequent calls. Failures
// are fatal — the test cannot proceed without the handshake.
func initializeStream(t *testing.T, stream grpc.ClientStream) {
	t.Helper()
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`)
	if err := stream.SendMsg(&payload); err != nil {
		t.Fatalf("initialize send: %v", err)
	}
	var reply []byte
	if err := stream.RecvMsg(&reply); err != nil {
		t.Fatalf("initialize recv: %v", err)
	}
	notif := []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if err := stream.SendMsg(&notif); err != nil {
		t.Fatalf("initialized notif: %v", err)
	}
}

// TestExchange_ResourceSubscription_PerStream pins the gRPC fix on the
// real wire path: stream A subscribes to URI X, stream B does not, then
// the server emits NotifyResourceUpdated for X. Only stream A must
// observe the notification frame. Before the per-notifier scoping change
// stream B would have received the leak because the resource
// subscription set lived on the shared *mcp.Server and the notifierHub
// broadcast to every active stream.
func TestExchange_ResourceSubscription_PerStream(t *testing.T) {
	srv := mcp.NewServer("test", nil, nil, nil)
	srv.ResourceProvider = stubResourceProvider{}
	streamA, streamB, cleanup := twoStreamHarness(t, srv)
	defer cleanup()

	initializeStream(t, streamA)
	initializeStream(t, streamB)

	const uri = "clockify://workspace/ws/entry/grpc-scope"

	subscribe := []byte(`{"jsonrpc":"2.0","id":2,"method":"resources/subscribe","params":{"uri":"` + uri + `"}}`)
	if err := streamA.SendMsg(&subscribe); err != nil {
		t.Fatalf("subscribe send: %v", err)
	}
	var subReply []byte
	if err := streamA.RecvMsg(&subReply); err != nil {
		t.Fatalf("subscribe recv: %v", err)
	}

	srv.NotifyResourceUpdated(uri, mcp.ResourceUpdateDelta{})

	// Stream A must observe the notification frame within a generous
	// window — bufconn round-trips are sub-millisecond in practice.
	var frameA []byte
	doneA := make(chan error, 1)
	go func() { doneA <- streamA.RecvMsg(&frameA) }()
	select {
	case err := <-doneA:
		if err != nil {
			t.Fatalf("stream A recv: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("stream A did not receive the subscribed update within 2s")
	}
	var envelope map[string]any
	if err := json.Unmarshal(frameA, &envelope); err != nil {
		t.Fatalf("stream A unmarshal: %v; raw=%s", err, frameA)
	}
	if envelope["method"] != "notifications/resources/updated" {
		t.Fatalf("stream A: expected resources/updated, got %v (raw=%s)", envelope["method"], frameA)
	}

	// Stream B must NOT receive any frame in the same window. Use a
	// reasonable deadline (much longer than bufconn round-trip) so a
	// regression that re-broadcasts to every notifier surfaces here.
	var frameB []byte
	doneB := make(chan error, 1)
	go func() { doneB <- streamB.RecvMsg(&frameB) }()
	select {
	case err := <-doneB:
		t.Fatalf("stream B received a notification frame it never subscribed to (err=%v, frame=%s)", err, frameB)
	case <-time.After(500 * time.Millisecond):
		// Pass — B observed nothing within the window.
	}
}
