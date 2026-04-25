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
	if !strings.Contains(out, "not built with -tags=postgres") {
		t.Fatalf("doctor backend finding missing postgres build-tag guidance:\n%s", out)
	}
}
