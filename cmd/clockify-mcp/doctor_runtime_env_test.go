package main

import (
	"strings"
	"testing"
)

// TestDoctorRejectsInvalidPolicy pins the post-Load validation step
// for CLOCKIFY_POLICY. Without it, doctor returned "Load() result: OK"
// even though `runtime.New()` would refuse to boot with the same env
// (`policy.FromEnv()` returns `invalid CLOCKIFY_POLICY: ...`).
// Operators using doctor as a pre-deploy gate hit the surprise at
// server-startup time.
func TestDoctorRejectsInvalidPolicy(t *testing.T) {
	code, out := runDoctorForTest(t, nil, map[string]string{
		"CLOCKIFY_API_KEY": "test-key",
		"CLOCKIFY_POLICY":  "not-a-mode",
	})
	if code != 2 {
		t.Fatalf("doctor with invalid CLOCKIFY_POLICY exit = %d, want 2; output:\n%s", code, out)
	}
	if !strings.Contains(out, "CLOCKIFY_POLICY") {
		t.Fatalf("doctor error must name CLOCKIFY_POLICY; got:\n%s", out)
	}
	if !strings.Contains(out, "Load() result:") || !strings.Contains(out, "ERROR") {
		t.Fatalf("doctor must surface the validation failure as Load() ERROR; got:\n%s", out)
	}
}

// TestDoctorRejectsInvalidBootstrapMode pins the same post-Load step
// for CLOCKIFY_BOOTSTRAP_MODE. The bootstrap validator is the only
// place that catches a typo like `full_tier_1` (note the underscore);
// before this change doctor reported OK and the user only saw the
// failure when the server tried to start.
func TestDoctorRejectsInvalidBootstrapMode(t *testing.T) {
	code, out := runDoctorForTest(t, nil, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"CLOCKIFY_BOOTSTRAP_MODE": "full_tier_1",
	})
	if code != 2 {
		t.Fatalf("doctor with invalid CLOCKIFY_BOOTSTRAP_MODE exit = %d, want 2; output:\n%s", code, out)
	}
	if !strings.Contains(out, "bootstrap mode") {
		t.Fatalf("doctor error must name bootstrap mode; got:\n%s", out)
	}
}

// TestDoctorAcceptsValidPolicyAndBootstrapMode is the negative
// drift-check: the new validators must not red on the standard
// defaults. A regression here would block every operator who pairs a
// stdio profile with the default bootstrap surface.
func TestDoctorAcceptsValidPolicyAndBootstrapMode(t *testing.T) {
	code, out := runDoctorForTest(t, nil, map[string]string{
		"CLOCKIFY_API_KEY":        "test-key",
		"CLOCKIFY_POLICY":         "time_tracking_safe",
		"CLOCKIFY_BOOTSTRAP_MODE": "full_tier1",
	})
	if code != 0 {
		t.Fatalf("doctor with valid policy+bootstrap exit = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, "Load() result:") || !strings.Contains(out, "OK") {
		t.Fatalf("doctor must report Load() OK on valid env; got:\n%s", out)
	}
}
