package grpctransport

import (
	"google.golang.org/grpc"
)

// ServiceName is the fully-qualified gRPC service name clients must use when
// dialling. It mirrors the notional proto package "clockify.mcp.v1.MCP" even
// though no .proto file is actually compiled.
const ServiceName = "clockify.mcp.v1.MCP"

// ExchangeMethod is the single bidirectional streaming RPC exposed by the
// service. Each frame is a JSON-RPC 2.0 message encoded with bytesCodec.
const ExchangeMethod = "Exchange"

// mcpServerIface is the minimal handler contract the transport registers
// against the hand-wired ServiceDesc. A server implementation receives a
// bidirectional stream and drives the JSON-RPC loop until the client closes
// or the server context cancels.
type mcpServerIface interface {
	Exchange(stream grpc.ServerStream) error
}

// exchangeServerHandler adapts the ServiceDesc StreamHandler signature to
// the mcpServerIface contract. grpc-go invokes it once per incoming stream.
func exchangeServerHandler(srv any, stream grpc.ServerStream) error {
	return srv.(mcpServerIface).Exchange(stream)
}

// buildServiceDesc constructs the hand-written grpc.ServiceDesc that replaces
// a protoc-generated descriptor. The metadata string is informational only —
// no .proto file is required for wire-level operation because bytesCodec
// handles all marshalling.
func buildServiceDesc() grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*mcpServerIface)(nil),
		Methods:     nil,
		Streams: []grpc.StreamDesc{{
			StreamName:    ExchangeMethod,
			Handler:       exchangeServerHandler,
			ServerStreams: true,
			ClientStreams: true,
		}},
		Metadata: "clockify-mcp-grpc.hand-wired",
	}
}
