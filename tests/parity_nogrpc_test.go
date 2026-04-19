//go:build !grpc

package e2e_test

import (
	"github.com/apet97/go-clockify/tests/harness"
)

// allFactories returns only the stdlib-only transports when the binary
// is built without -tags=grpc. The gRPC factory is unavailable and
// would only add a Skip branch to every parity subtest.
func allFactories() map[string]harness.Factory {
	return defaultFactories()
}
