package tools

import (
	"encoding/json"
	"testing"

	"github.com/apet97/go-clockify/internal/jsonschema"
)

func FuzzTightenInputSchema(f *testing.F) {
	for _, seed := range []string{
		`{"type":"object","properties":{"page":{"type":"integer"},"start":{"type":"string","description":"RFC3339 timestamp"}}}`,
		`{"type":"array","items":{"type":"object","properties":{"page_size":{"type":"integer"}}}}`,
		`{"anyOf":[{"type":"object","properties":{"color":{"type":"string","description":"Hex color"}}}]}`,
		`{"type":"object","additionalProperties":true}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		var schema map[string]any
		if err := json.Unmarshal([]byte(raw), &schema); err != nil {
			return
		}
		tightenInputSchema(schema)
		if _, err := json.Marshal(schema); err != nil {
			t.Fatalf("tightened schema is not JSON-marshalable: %v", err)
		}
		_ = jsonschema.Validate(schema, map[string]any{})
	})
}
