package confirmation

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// FingerprintExcludedKeys is the set of argument keys stripped before
// fingerprinting. dry_run flips the request between preview and
// execution and would otherwise invalidate every token between mint
// and verify. confirmation_token is the binding itself — including it
// in its own fingerprint would be circular.
var FingerprintExcludedKeys = []string{
	"dry_run",
	"confirmation_token",
}

// BuildArgumentFingerprint returns a base64url-encoded SHA-256 hash
// of the canonical JSON serialization of args, with the
// FingerprintExcludedKeys removed.
//
// Canonical JSON: encoding/json sorts map keys alphabetically by
// default, including in nested map[string]any. Slices preserve order
// (intentional — argument order matters for tool semantics like
// tag_ids). Numbers, booleans, strings, and nil round-trip
// deterministically. The args map is assumed to be JSON-decodable
// (the MCP tools/call dispatch hands us a map[string]any decoded from
// the request body), so json.Marshal can never fail here in practice;
// we still return a stable empty-string sentinel on the unlikely
// error path so the caller's "binding mismatch" path is reached
// instead of a panic.
func BuildArgumentFingerprint(args map[string]any) string {
	cleaned := stripExcludedKeys(args)
	payload, err := json.Marshal(cleaned)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// stripExcludedKeys returns a shallow copy of args without the
// FingerprintExcludedKeys. Returns an empty (non-nil) map when args
// is nil so the resulting fingerprint is the same as an empty-args
// call rather than panicking on the type-assert path below.
func stripExcludedKeys(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	if args == nil {
		return out
	}
	excluded := make(map[string]struct{}, len(FingerprintExcludedKeys))
	for _, k := range FingerprintExcludedKeys {
		excluded[k] = struct{}{}
	}
	for k, v := range args {
		if _, skip := excluded[k]; skip {
			continue
		}
		out[k] = v
	}
	return out
}
