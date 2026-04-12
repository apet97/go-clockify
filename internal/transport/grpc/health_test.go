package grpctransport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/mcp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func newHealthTestHarness(t *testing.T, srv *mcp.Server) (*grpc.ClientConn, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	handler := &exchangeServer{srv: srv}
	desc := buildServiceDesc()
	healthDesc := buildHealthServiceDesc()
	hs := &healthServer{srv: srv}
	grpcSrv := grpc.NewServer(grpc.ForceServerCodec(bytesCodec{}))
	grpcSrv.RegisterService(&desc, handler)
	grpcSrv.RegisterService(&healthDesc, hs)
	go func() { _ = grpcSrv.Serve(lis) }()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(bytesCodec{})),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	cleanup := func() {
		_ = conn.Close()
		grpcSrv.GracefulStop()
		_ = lis.Close()
	}
	return conn, cleanup
}

func callHealthCheck(t *testing.T, conn *grpc.ClientConn, service string) int32 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	reqBytes := encodeHealthCheckRequestForTest(service)
	var respBytes []byte
	err := conn.Invoke(ctx, "/"+healthServiceName+"/"+healthCheckMethod, &reqBytes, &respBytes)
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	return decodeHealthCheckResponseForTest(respBytes)
}

func encodeHealthCheckRequestForTest(service string) []byte {
	if service == "" {
		return nil
	}
	b := make([]byte, 2+len(service))
	b[0] = 0x0a
	b[1] = byte(len(service))
	copy(b[2:], service)
	return b
}

func decodeHealthCheckResponseForTest(b []byte) int32 {
	if len(b) == 0 {
		return 0
	}
	if b[0] != 0x08 || len(b) < 2 {
		return -1
	}
	return int32(b[1])
}

func TestHealthCheckEmptyService(t *testing.T) {
	srv := newTestServer(t)
	srv.SetReadyCached(true)
	conn, cleanup := newHealthTestHarness(t, srv)
	defer cleanup()

	status := callHealthCheck(t, conn, "")
	if status != statusServing {
		t.Fatalf("expected SERVING (%d), got %d", statusServing, status)
	}
}

func TestHealthCheckNamedService(t *testing.T) {
	srv := newTestServer(t)
	srv.SetReadyCached(true)
	conn, cleanup := newHealthTestHarness(t, srv)
	defer cleanup()

	status := callHealthCheck(t, conn, ServiceName)
	if status != statusServing {
		t.Fatalf("expected SERVING (%d), got %d", statusServing, status)
	}
}

func TestHealthCheckNotReady(t *testing.T) {
	srv := newTestServer(t)
	conn, cleanup := newHealthTestHarness(t, srv)
	defer cleanup()

	status := callHealthCheck(t, conn, "")
	if status != statusNotServing {
		t.Fatalf("expected NOT_SERVING (%d), got %d", statusNotServing, status)
	}
}

func TestHealthCheckUnknownService(t *testing.T) {
	srv := newTestServer(t)
	srv.SetReadyCached(true)
	conn, cleanup := newHealthTestHarness(t, srv)
	defer cleanup()

	status := callHealthCheck(t, conn, "some.unknown.Service")
	if status != 3 { // SERVICE_UNKNOWN
		t.Fatalf("expected SERVICE_UNKNOWN (3), got %d", status)
	}
}

func TestHealthCheckReadyFlip(t *testing.T) {
	srv := newTestServer(t)
	conn, cleanup := newHealthTestHarness(t, srv)
	defer cleanup()

	status := callHealthCheck(t, conn, "")
	if status != statusNotServing {
		t.Fatalf("before MarkReady: expected NOT_SERVING, got %d", status)
	}

	srv.SetReadyCached(true)
	status = callHealthCheck(t, conn, "")
	if status != statusServing {
		t.Fatalf("after MarkReady: expected SERVING, got %d", status)
	}
}
