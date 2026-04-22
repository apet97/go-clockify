package config

import "testing"

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
