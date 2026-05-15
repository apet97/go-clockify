package clockify

import "strings"

// normalizeEndpoint collapses a concrete Clockify API path into a template
// by replacing ID-shaped segments (24/32/36-char hex — BSON ObjectID or
// UUID) with `:id`. It keys the circuit breaker on the resource shape
// rather than on every concrete ID, keeping its per-endpoint state map
// bounded. The function is pure and allocates only on paths containing IDs.
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
// identifier: a 24-character hex string (BSON ObjectID), a 32-character hex
// string (UUID without hyphens), or a 36-character canonical UUID.
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
