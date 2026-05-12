package tools

import (
	"reflect"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
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

// TestSchemaForTypeMemoized pins the schemaTypeCache invariant: a
// given reflect.Type produces the same map reference on every call.
// Covers a struct (clockify.TimeEntry), the time.Time primitive, and a
// pointer dereference (*clockify.Project must share *clockify.Project's
// post-deref cache entry).
//
// Drift check: delete the cache hit branch in schemaForType so every
// call falls through to computeSchemaForType, and this test fails on
// the first identity assertion.
func TestSchemaForTypeMemoized(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
	}{
		{"clockify.TimeEntry", reflect.TypeFor[clockify.TimeEntry]()},
		{"time.Time", reflect.TypeFor[time.Time]()},
		{"*clockify.Project", reflect.TypeFor[*clockify.Project]()},
	}
	for _, tc := range cases {
		a := schemaForType(tc.typ)
		b := schemaForType(tc.typ)
		if reflect.ValueOf(a).Pointer() != reflect.ValueOf(b).Pointer() {
			t.Fatalf("schemaForType(%s) not memoised: pointers differ across calls", tc.name)
		}
	}

	// Same post-deref type must share a cache entry regardless of how
	// many pointer layers wrap it.
	plain := schemaForType(reflect.TypeFor[clockify.Project]())
	pointered := schemaForType(reflect.TypeFor[*clockify.Project]())
	if reflect.ValueOf(plain).Pointer() != reflect.ValueOf(pointered).Pointer() {
		t.Fatalf("schemaForType(Project) and schemaForType(*Project) returned distinct maps; deref-cache invariant broken")
	}
}
