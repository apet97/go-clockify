package clockify

import (
	"strings"
)

// normalizeEndpoint collapses a concrete Clockify API path into a template
// suitable for use as a Prometheus label value. Every segment that looks
// like a Clockify ID (24 lowercase hex characters) is replaced with `:id`,
// and any other non-letter leading segment gets the same treatment so
// cardinality stays bounded regardless of traffic patterns.
//
// Examples:
//
//	/workspaces/64abc.../time-entries/abc123... → /workspaces/:id/time-entries/:id
//	/user                                       → /user
//	/workspaces                                 → /workspaces
//
// The function is pure and allocates only on paths containing IDs, so it is
// cheap enough to call on every request in the hot path.
func normalizeEndpoint(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.ContainsAny(path, "/") {
		return "/" + path
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if isIDSegment(seg) {
			segments[i] = ":id"
		}
	}
	return strings.Join(segments, "/")
}

// isIDSegment reports whether a path segment looks like a Clockify object
// identifier. Clockify IDs are 24-character lowercase hex strings, but user
// IDs and some webhook-targeted segments can be longer (32 chars for UUIDs
// without hyphens). Both patterns collapse to `:id`.
func isIDSegment(seg string) bool {
	n := len(seg)
	if n != 24 && n != 32 && n != 36 {
		return false
	}
	for _, r := range seg {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// statusBucket maps an HTTP status code to a coarse Prometheus label value.
// Keeping this bounded ({2xx, 3xx, 4xx, 5xx, error}) means the upstream
// metric labelset remains tiny regardless of Clockify's error variety.
func statusBucket(code int) string {
	switch {
	case code == 0:
		return "error"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "other"
	}
}

// retryReason classifies a retryable error into a short label for
// clockify_upstream_retries_total{reason}. Unknown reasons fall back to "error".
func retryReason(statusCode int) string {
	switch statusCode {
	case 429:
		return "rate_limited"
	case 502:
		return "bad_gateway"
	case 503:
		return "service_unavailable"
	case 504:
		return "gateway_timeout"
	default:
		return "error"
	}
}
