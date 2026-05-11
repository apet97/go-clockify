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

// TestStatusBucket locks the coarse-grained mapping that backs the
// {2xx,3xx,4xx,5xx,error,other} label on
// clockify_upstream_request_duration_seconds. The switch is the only
// authority for the label cardinality; without this test a refactor
// that widens or narrows a band (e.g. treating 1xx as "2xx") could
// silently change Prometheus dashboards.
func TestStatusBucket(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "error"},
		{100, "other"},
		{200, "2xx"},
		{201, "2xx"},
		{204, "2xx"},
		{299, "2xx"},
		{300, "3xx"},
		{301, "3xx"},
		{304, "3xx"},
		{399, "3xx"},
		{400, "4xx"},
		{401, "4xx"},
		{403, "4xx"},
		{404, "4xx"},
		{429, "4xx"},
		{499, "4xx"},
		{500, "5xx"},
		{502, "5xx"},
		{503, "5xx"},
		{504, "5xx"},
		{599, "5xx"},
		{600, "other"},
	}
	for _, c := range cases {
		got := statusBucket(c.code)
		if got != c.want {
			t.Errorf("statusBucket(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

// TestRetryReason pins the four retryable-error labels exported on
// clockify_upstream_retries_total{reason}. Non-retryable codes must
// fall through to "error" so we don't grow the labelset with one bucket
// per Clockify 4xx variant.
func TestRetryReason(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{429, "rate_limited"},
		{502, "bad_gateway"},
		{503, "service_unavailable"},
		{504, "gateway_timeout"},
		{400, "error"},
		{401, "error"},
		{403, "error"},
		{404, "error"},
		{500, "error"},
		{0, "error"},
	}
	for _, c := range cases {
		got := retryReason(c.code)
		if got != c.want {
			t.Errorf("retryReason(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}
