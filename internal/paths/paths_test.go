package paths

import (
	"strings"
	"testing"
)

// TestWorkspace_HappyPath locks the simple 24-char-hex case so a
// caller migration drop-in stays byte-identical to the existing
// "/workspaces/" + wsID concat.
func TestWorkspace_HappyPath(t *testing.T) {
	got, err := Workspace("5e0fa5cb6c5dc403da9f1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/workspaces/5e0fa5cb6c5dc403da9f1234" {
		t.Fatalf("unexpected result: %q", got)
	}
}

// TestWorkspace_RejectsBadID forwards every category that
// resolve.ValidateID rejects. The helper is the centralisation
// point — if a regression weakens the validator the matrix here
// catches it without every handler test having to re-cover the
// surface.
func TestWorkspace_RejectsBadID(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"bad/path",
		"bad?query",
		"bad#frag",
		"bad%2Fpath",
		"foo..bar",
		"ws\x01id",
	}
	for _, id := range bad {
		t.Run(id, func(t *testing.T) {
			if _, err := Workspace(id); err == nil {
				t.Fatalf("expected validation error for %q", id)
			}
		})
	}
}

// TestWorkspace_AppendsSubSegments locks the multi-segment join with
// percent-encoding for unsafe characters. Sub-segments come from
// validated upstream resolvers so this is the second-layer defence
// against an accidentally-unescaped value.
func TestWorkspace_AppendsSubSegments(t *testing.T) {
	cases := []struct {
		name string
		sub  []string
		want string
	}{
		{
			name: "single",
			sub:  []string{"projects"},
			want: "/workspaces/ws1/projects",
		},
		{
			name: "two_levels",
			sub:  []string{"projects", "p1"},
			want: "/workspaces/ws1/projects/p1",
		},
		{
			name: "escapes_space",
			sub:  []string{"clients", "ACME Co"},
			want: "/workspaces/ws1/clients/ACME%20Co",
		},
		{
			name: "escapes_percent",
			sub:  []string{"projects", "R&D 50%"},
			want: "/workspaces/ws1/projects/R&D%2050%25",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Workspace("ws1", c.sub...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}

// TestWorkspace_RejectsBadSubSegments makes the structural
// constraints explicit: empty segment or one already containing a
// slash both fail at construction. These are caller bugs that would
// otherwise produce silently-wrong paths.
func TestWorkspace_RejectsBadSubSegments(t *testing.T) {
	cases := []struct {
		name string
		sub  []string
		hint string
	}{
		{"empty_segment", []string{""}, "empty"},
		{"empty_in_middle", []string{"projects", "", "p1"}, "empty"},
		{"slash_in_segment", []string{"projects/p1"}, "slash"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Workspace("ws1", c.sub...)
			if err == nil {
				t.Fatalf("expected error for %s", c.hint)
			}
			if !strings.Contains(err.Error(), c.hint) {
				t.Fatalf("error %q should mention %q", err, c.hint)
			}
		})
	}
}
