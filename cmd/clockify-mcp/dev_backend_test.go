package main

import "testing"

// TestIsDevControlPlaneDSN locks the C1 predicate: memory, empty,
// file://, and bare paths are dev; anything with an explicit
// non-file scheme (postgres://, mysql://, etc.) is production.
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
		if got := isDevControlPlaneDSN(tc.dsn); got != tc.dev {
			t.Errorf("isDevControlPlaneDSN(%q) = %v, want %v", tc.dsn, got, tc.dev)
		}
	}
}
