package safety

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

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
