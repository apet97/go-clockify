package grpctransport

import (
	"context"

	"github.com/apet97/go-clockify/internal/mcp"

	"google.golang.org/grpc"
)

const (
	healthServiceName = "grpc.health.v1.Health"
	healthCheckMethod = "Check"

	statusServing    int32 = 1
	statusNotServing int32 = 2
)

type healthChecker interface {
	checkHealth(service string) int32
}

type healthServer struct {
	srv *mcp.Server
}

func (h *healthServer) checkHealth(service string) int32 {
	switch service {
	case "", ServiceName:
		if h.srv.IsReadyCached() {
			return statusServing
		}
		return statusNotServing
	default:
		return 3 // SERVICE_UNKNOWN
	}
}

func healthCheckHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	var reqBytes []byte
	if err := dec(&reqBytes); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, _ any) (any, error) {
		service := decodeHealthCheckRequest(reqBytes)
		status := srv.(healthChecker).checkHealth(service)
		resp := encodeHealthCheckResponse(status)
		return &resp, nil
	}
	if interceptor != nil {
		return interceptor(ctx, &reqBytes, &grpc.UnaryServerInfo{
			Server:     srv,
			FullMethod: "/" + healthServiceName + "/" + healthCheckMethod,
		}, handler)
	}
	return handler(ctx, &reqBytes)
}

func buildHealthServiceDesc() grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: healthServiceName,
		HandlerType: (*healthChecker)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: healthCheckMethod,
			Handler:    healthCheckHandler,
		}},
		Metadata: "grpc-health.hand-wired",
	}
}

// decodeHealthCheckRequest extracts the service name from a protobuf-encoded
// grpc.health.v1.HealthCheckRequest. The message has a single string field
// (field 1, wire type 2). An empty message means the empty-string service.
func decodeHealthCheckRequest(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if b[0] != 0x0a { // field 1, length-delimited
		return ""
	}
	if len(b) < 2 {
		return ""
	}
	length := int(b[1])
	if length > 0x7f {
		return "" // multi-byte varint lengths are unrealistic for service names
	}
	if 2+length > len(b) {
		return ""
	}
	return string(b[2 : 2+length])
}

// encodeHealthCheckResponse produces a protobuf-encoded
// grpc.health.v1.HealthCheckResponse. The message has a single enum field
// (field 1, wire type 0). Status 0 (UNKNOWN) encodes as an empty message.
func encodeHealthCheckResponse(status int32) []byte {
	if status == 0 {
		return nil
	}
	return []byte{0x08, byte(status)}
}
