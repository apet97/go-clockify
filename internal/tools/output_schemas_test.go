package tools

import (
	"reflect"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

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
