//go:build grpc

package harness

import "errors"

// ErrGRPCUnavailable is defined in grpc_stub.go for non-grpc builds. Under
// -tags=grpc the real harness is available so the error never fires in
// practice, but we keep a no-op declaration so tests can reference the
// symbol unconditionally.
var ErrGRPCUnavailable = errors.New("harness: gRPC available (this should never be returned)")
