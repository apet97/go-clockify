package tools

import (
	"reflect"
	"testing"
)

// TestTier1OutputSchemasMemoized pins the sync.OnceValue memoisation
// contract: every Service.Registry() call across every session must
// share the same map reference, otherwise we silently regress the
// reflection-heavy envelopeSchemaFor[T] cost on every session bootstrap.
//
// Drift check: replace `var tier1OutputSchemas = sync.OnceValue(...)`
// with `func tier1OutputSchemas() map[string]map[string]any { return
// buildTier1OutputSchemas() }` and this test fails on the pointer-
// identity line because each call allocates a fresh outer map.
func TestTier1OutputSchemasMemoized(t *testing.T) {
	a := tier1OutputSchemas()
	b := tier1OutputSchemas()
	if reflect.ValueOf(a).Pointer() != reflect.ValueOf(b).Pointer() {
		t.Fatalf("tier1OutputSchemas not memoised: outer map pointers differ across calls")
	}
	for name, schemaA := range a {
		schemaB, ok := b[name]
		if !ok {
			t.Fatalf("schema %q missing from second call", name)
		}
		if reflect.ValueOf(schemaA).Pointer() != reflect.ValueOf(schemaB).Pointer() {
			t.Fatalf("schema %q inner map not memoised: pointers differ across calls", name)
		}
	}
}
