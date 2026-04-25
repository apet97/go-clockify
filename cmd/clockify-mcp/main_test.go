package main

import "testing"

func TestEffectiveVersionDefaultIsNotStaleReleaseLiteral(t *testing.T) {
	if version == "1.0.0" {
		t.Fatal("default version literal must not be a stale release number")
	}
	if got := effectiveVersion(); got == "1.0.0" {
		t.Fatalf("effectiveVersion() = %q, want non-stale default", got)
	}
}

func TestEffectiveVersionPrefersInjectedVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v9.9.9-test"

	if got := effectiveVersion(); got != "v9.9.9-test" {
		t.Fatalf("effectiveVersion() = %q, want injected version", got)
	}
}
