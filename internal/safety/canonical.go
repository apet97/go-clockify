// Package safety implements the runtime safety rails for destructive,
// administrative, and raw-write tool calls: confirmation-token issuance
// and validation (TokenStore), risk-class policy lookup
// (RequirementForRisk), and canonical-JSON hashing used to bind a
// confirmation token to the exact argument payload that previewed it
// (HashCanonical).
package safety

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// HashCanonical returns a hex-encoded SHA-256 of v after stripping
// non-deterministic fields ("confirm_token", "dry_run") and recursively
// normalising map/slice contents. The resulting digest is stable across
// equivalent argument shapes so a confirmation-token preview hash can be
// matched against a follow-up call's arguments byte-for-byte.
func HashCanonical(v any) string {
	normalized := normalizeJSONValue(v)
	b, _ := json.Marshal(normalized)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func normalizeJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, value := range t {
			switch key {
			case "confirm_token", "dry_run":
				continue
			default:
				out[key] = normalizeJSONValue(value)
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = normalizeJSONValue(t[i])
		}
		return out
	default:
		return v
	}
}
