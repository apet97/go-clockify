package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResultEnvelopeSanitizesSecretNamedFields(t *testing.T) {
	const canary = "secret-canary-value-1234567890"

	result := ok("secret_contract", map[string]any{
		"id":            "visible-id",
		"token":         canary,
		"secret":        canary,
		"password":      canary,
		"x-api-key":     canary,
		"x-addon-token": canary,
		"nested": map[string]any{
			"access_token":  canary,
			"refresh_token": canary,
			"client_secret": canary,
			"items": []any{
				map[string]any{"authToken": canary},
			},
		},
	}, map[string]any{
		"cookie":     canary,
		"credential": canary,
		"safe_count": 1,
	})

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), canary) {
		t.Fatalf("tool result leaked secret canary: %s", raw)
	}

	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result data type = %T, want map[string]any", result.Data)
	}
	if data["id"] != "visible-id" {
		t.Fatalf("safe fields must survive sanitization, got %#v", data)
	}
	if result.Meta["safe_count"] != 1 {
		t.Fatalf("safe metadata must survive sanitization, got %#v", result.Meta)
	}
}
