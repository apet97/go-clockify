//go:build legacy_platform

package tools_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func TestTier2Dispatch_CustomFields_GetUsesListScan(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/test-workspace/custom-fields", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"f1","name":"Region"},{"id":"f2","name":"Priority"}]`))
	})
	mux.HandleFunc("/workspaces/test-workspace/custom-fields/f1", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("single custom-field GET must not be used; live Clockify returns 405 for this path")
	})
	upstream := testharness.NewFakeClockify(t, mux)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "custom_fields",
		Tool:     "clockify_get_custom_field",
		Args:     map[string]any{"field_id": "f1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, `"id":"f1"`) || !strings.Contains(res.ResultText, `"name":"Region"`) {
		t.Fatalf("get result missing scanned custom field: %q", res.ResultText)
	}
}
