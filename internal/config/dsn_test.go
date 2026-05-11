package config

import (
	"strings"
	"testing"
)

// TestIsDevControlPlaneDSN locks the predicate shared by Load() and
// runtime.BuildStore. memory, empty, and any file:// (or bare path)
// DSN is dev-only; only an explicit non-file scheme (e.g. postgres://)
// is treated as production-capable.
func TestIsDevControlPlaneDSN(t *testing.T) {
	cases := []struct {
		dsn string
		dev bool
	}{
		{"", true},
		{"memory", true},
		{"memory://", true},
		{"  memory  ", true}, // whitespace tolerant
		{"file:///var/lib/mcp/state.json", true},
		{"/var/lib/mcp/state.json", true}, // bare path
		{"postgres://user:pass@db.example:5432/mcp", false},
		{"postgres://db", false},
		{"mysql://db", false},
	}
	for _, tc := range cases {
		if got := IsDevControlPlaneDSN(tc.dsn); got != tc.dev {
			t.Errorf("IsDevControlPlaneDSN(%q) = %v, want %v", tc.dsn, got, tc.dev)
		}
	}
}

// TestFingerprintRedactsControlPlaneDSNPassword pins the call-site
// wiring: Config.Fingerprint() must run the DSN through
// sanitizeDSNForFingerprint so the raw password never lands in a
// startup-log fingerprint map. Drift-check guard against any future
// refactor that bypasses the helper.
func TestFingerprintRedactsControlPlaneDSNPassword(t *testing.T) {
	cfg := Config{ControlPlaneDSN: "postgres://user:supersecret@db.example:5432/mcp"}
	fp := cfg.Fingerprint()
	got, _ := fp["control_plane_dsn"].(string)
	if got == "" {
		t.Fatal("control_plane_dsn missing from fingerprint")
	}
	if got == cfg.ControlPlaneDSN {
		t.Fatalf("fingerprint surfaced raw DSN with password: %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Fatalf("fingerprint DSN must contain REDACTED marker, got %q", got)
	}
	if strings.Contains(got, "supersecret") {
		t.Fatalf("password leaked into fingerprint DSN: %q", got)
	}
}

// TestSanitizeDSNForFingerprint pins the password-redaction contract
// used by Config.Fingerprint(): when the control-plane DSN carries a
// password in URL userinfo, the password must be replaced with
// [REDACTED] before the DSN appears in any startup log; dev backends
// (empty/memory/file) are passed through unchanged because they carry
// no credential material.
func TestSanitizeDSNForFingerprint(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"memory", "memory", "memory"},
		{"file", "file:///var/lib/mcp/state.json", "file:///var/lib/mcp/state.json"},
		{"postgres with password", "postgres://user:secret@db.example:5432/mcp", "postgres://user:%5BREDACTED%5D@db.example:5432/mcp"},
		{"postgres without password", "postgres://user@db.example:5432/mcp", "postgres://user@db.example:5432/mcp"},
		{"postgres no userinfo", "postgres://db.example:5432/mcp", "postgres://db.example:5432/mcp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeDSNForFingerprint(tc.in); got != tc.want {
				t.Errorf("sanitizeDSNForFingerprint(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
