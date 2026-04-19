//go:build grpc

package e2e_test

import (
	"github.com/apet97/go-clockify/tests/harness"
)

// allFactories returns every transport including gRPC when -tags=grpc
// is set. Parity tests then run one extra subtest row per case.
func allFactories() map[string]harness.Factory {
	m := defaultFactories()
	m["grpc"] = harness.NewGRPC
	return m
}
