//go:build legacy_platform

package tools_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func TestTier2Dispatch_GroupsHolidays_ListUserGroupsAdminPagination(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/workspaces/test-workspace/user-groups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		q := r.URL.Query()
		if q.Get("page") != "2" || q.Get("page-size") != "25" {
			t.Fatalf("unexpected user-group list query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"g1","name":"Engineering"}]`))
	})
	upstream := testharness.NewFakeClockify(t, mux)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "groups_holidays",
		Tool:     "clockify_list_user_groups_admin",
		Args:     map[string]any{"page": 2, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "g1") {
		t.Fatalf("list result missing group id: %q", res.ResultText)
	}
}
