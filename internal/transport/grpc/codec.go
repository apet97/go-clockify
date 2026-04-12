// Package grpctransport exposes the Clockify MCP server over gRPC using a
// raw-bytes passthrough codec and a hand-wired ServiceDesc. The design
// intentionally avoids protobuf codegen: each Exchange frame is a JSON-RPC
// 2.0 message serialized as UTF-8 bytes, and grpc-go transports them via a
// custom encoding.Codec without any .proto file compilation step.
//
// This package is a separate Go module with its own go.mod. The top-level
// build remains stdlib-only per ADR 001. Reach it from the main module only
// under -tags=grpc (see cmd/clockify-mcp/grpc_on.go).
package grpctransport

import (
	"fmt"

	"google.golang.org/grpc/encoding"
)

// codecName is the value returned by Codec.Name(). Clients must request this
// codec via grpc.ForceCodec / grpc.ForceServerCodec so that the raw-bytes
// marshalling below is selected instead of protobuf.
const codecName = "clockify-mcp-bytes"

// bytesCodec marshals and unmarshals *[]byte values directly without any
// protobuf reflection. Only *[]byte is supported on both sides; other types
// return an error to prevent silent wire corruption.
type bytesCodec struct{}

func (bytesCodec) Name() string { return codecName }

func (bytesCodec) Marshal(v any) ([]byte, error) {
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
		return nil, fmt.Errorf("grpctransport: bytesCodec only marshals *[]byte or []byte, got %T", v)
	}
}

func (bytesCodec) Unmarshal(data []byte, v any) error {
	p, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("grpctransport: bytesCodec only unmarshals into *[]byte, got %T", v)
	}
	*p = append((*p)[:0], data...)
	return nil
}

// registerCodec installs bytesCodec under its canonical name on the grpc
// encoding registry. It is safe to call multiple times — encoding.RegisterCodec
// overwrites the previous registration, and we use a package-level init to
// ensure a single installation per process. The codec is only consulted when
// ForceCodec / ForceServerCodec selects codecName.
func init() {
	encoding.RegisterCodec(bytesCodec{})
}
