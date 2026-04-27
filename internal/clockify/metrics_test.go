package clockify

import "testing"

// TestNormalizeEndpoint_IDShapesCollapsed confirms the three ID
// shapes (24-hex BSON, 32-hex UUID-without-hyphens, 36-char canonical
// UUID) all collapse to :id. These are the shapes Clockify's API
// actually returns.
func TestNormalizeEndpoint_IDShapesCollapsed(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/workspaces/5e0fa5cb6c5dc403da9f1234", "/workspaces/:id"},
		{"/workspaces/5e0fa5cb6c5dc403da9f1234/time-entries/abc1234567890def0987654321aaaaaa", "/workspaces/:id/time-entries/:id"},
		{"/workspaces/5e0fa5cb6c5dc403da9f1234/users/abcd1234-ef56-7890-abcd-1234567890ab", "/workspaces/:id/users/:id"},
		{"/user", "/user"},
		{"/workspaces", "/workspaces"},
		{"", "/"},
	}
	for _, c := range cases {
		if got := normalizeEndpoint(c.in); got != c.want {
			t.Errorf("normalizeEndpoint(%q)=%q want=%q", c.in, got, c.want)
		}
	}
}

// TestNormalizeEndpoint_NonIDShapesPreserved locks in audit finding 11
// nuance: the matcher is length-bounded (24/32/36) and does NOT
// collapse arbitrary non-ID segments. A 16-char hex token, a 40-char
// SHA, or a slug stays in the path verbatim. This is the actual
// behaviour the comment now documents; adding a regression here means
// the comment and the implementation cannot drift apart silently.
func TestNormalizeEndpoint_NonIDShapesPreserved(t *testing.T) {
	cases := []string{
		"/workspaces/short",
		"/workspaces/abc1234567890def",                         // 16 hex
		"/workspaces/abcd1234abcd1234abcd1234abcd1234abcd1234", // 40 hex
		"/workspaces/my-cool-slug",
	}
	for _, in := range cases {
		got := normalizeEndpoint(in)
		if got != in {
			t.Errorf("non-ID segment unexpectedly collapsed: normalizeEndpoint(%q)=%q want=%q", in, got, in)
		}
	}
}
