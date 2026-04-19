//go:build !grpc

package harness

import (
	"context"
	"errors"
)

// NewGRPC is a stub for non-grpc builds. The real implementation lives in
// grpc.go behind -tags=grpc. Parity tests skip gRPC when this factory
// returns ErrGRPCUnavailable.
func NewGRPC(_ context.Context, _ Options) (Transport, error) {
	return nil, ErrGRPCUnavailable
}

// ErrGRPCUnavailable signals that the harness was compiled without the
// grpc build tag; callers should skip gRPC-specific tests rather than
// fail.
var ErrGRPCUnavailable = errors.New("harness: gRPC unavailable (build with -tags=grpc)")
