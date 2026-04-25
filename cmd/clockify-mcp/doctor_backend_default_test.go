//go:build !postgres

package main

import (
	"strings"
	"testing"
)

func TestDoctorStrictCheckBackendsReportsMissingPostgresBuildTag(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict", "--check-backends"}, strictCleanDoctorEnv(nil))
	if code != 3 {
		t.Fatalf("doctor --strict --check-backends exit = %d, want 3; output:\n%s", code, out)
	}
	if !strings.Contains(out, "MCP_CONTROL_PLANE_DSN") {
		t.Fatalf("doctor backend finding missing DSN key:\n%s", out)
	}
	if !strings.Contains(out, "--check-backends requires a binary built with -tags=postgres") {
		t.Fatalf("doctor backend finding missing postgres build-tag guidance:\n%s", out)
	}
}

func TestDoctorStrictCheckBackendsRejectsNonPostgresDSN(t *testing.T) {
	code, out := runDoctorForTest(t, []string{"--strict", "--check-backends"}, strictCleanDoctorEnv(map[string]string{
		"MCP_CONTROL_PLANE_DSN": "memory",
		"MCP_ALLOW_DEV_BACKEND": "1",
	}))
	if code != 3 {
		t.Fatalf("doctor --strict --check-backends memory exit = %d, want 3; output:\n%s", code, out)
	}
	for _, want := range []string{"MCP_CONTROL_PLANE_DSN", "hosted backend checks require a postgres:// or postgresql:// control-plane DSN"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor backend finding missing %q:\n%s", want, out)
		}
	}
}
