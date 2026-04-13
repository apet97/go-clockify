package resolve

import (
	"strings"
	"testing"
)

// BenchmarkValidateID exercises the path-injection guard that runs
// before every tool call that takes an ID argument. A regression here
// hits every dispatched call against the workspace and is the kind of
// thing that's easy to overlook because it does not fail any
// correctness test.
//
// The corpus mixes typical Clockify object-id shapes (24-char hex)
// with the rejection cases the function exists to catch. Both paths
// must stay fast.
//
// Run: go test -bench=BenchmarkValidateID -benchtime=10x ./internal/resolve
func BenchmarkValidateID(b *testing.B) {
	corpus := []string{
		"5e2c8f9b8c1f4a7d6e9b3c1a", // typical 24-char hex
		"5b1e2c0bb079873471b6f6e8",
		"deadbeefcafebabe12345678",
		"workspace-1",
		"u_abc123",
		"../../../etc/passwd",    // rejection: path traversal
		"foo?bar=1",              // rejection: contains ?
		"foo#frag",               // rejection: contains #
		"foo/bar",                // rejection: contains /
		strings.Repeat("a", 200), // rejection: too long
	}
	b.ReportAllocs()
	i := 0
	for b.Loop() {
		_ = ValidateID(corpus[i%len(corpus)], "id")
		i++
	}
}
