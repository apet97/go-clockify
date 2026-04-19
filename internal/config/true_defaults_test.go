package config

import (
	"os"
	"testing"
)

// TestTrueDefaults asserts that exact default behaviors match our documentation
// without any overrides from the setEnvs testing helper.
func TestTrueDefaults(t *testing.T) {
	os.Clearenv()
	// Minimum required to load without error in some transports
	os.Setenv("CLOCKIFY_API_KEY", "dummy")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AuditDurabilityMode != "best_effort" {
		t.Fatalf("Failed true defaults: expected AuditDurabilityMode=best_effort, got %q", cfg.AuditDurabilityMode)
	}

	if cfg.MetricsBind != "" {
		t.Fatalf("Failed true defaults: expected MetricsBind to be empty, making dedicated metrics optional. Got %q", cfg.MetricsBind)
	}

	if cfg.MaxMessageSize != 4194304 {
		t.Fatalf("Failed true defaults: expected MaxMessageSize=4194304 (4MB), got %d", cfg.MaxMessageSize)
	}
}
