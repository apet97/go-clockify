package clockify

import (
	"encoding/json"
	"strings"
	"testing"
)

func FuzzTranslateAPIError(f *testing.F) {
	for _, seed := range []struct {
		status int
		body   string
	}{
		{401, `{"message":"Unauthorized"}`},
		{403, `{"message":"Forbidden"}`},
		{404, `{"message":"Project doesn't belong to Workspace","code":501}`},
		{429, `{"message":"rate limit exceeded"}`},
		{400, `{"message":"plan feature disabled","apiKey":"secret-value"}`},
		{500, `not json`},
	} {
		f.Add(seed.status, seed.body)
	}
	f.Fuzz(func(t *testing.T, status int, body string) {
		translation := TranslateAPIError(status, body)
		if strings.TrimSpace(translation.Message) == "" {
			t.Fatalf("empty translation message for status=%d body=%q", status, body)
		}
		if _, err := json.Marshal(translation); err != nil {
			t.Fatalf("translation is not JSON-marshalable: %v", err)
		}
	})
}
