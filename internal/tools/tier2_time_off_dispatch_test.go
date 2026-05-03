package tools_test

// Dispatcher-level coverage for tier2 time_off.listTimeOffRequests.
// Pins SUMMARY rev 3 #5: the upstream is POST-only and returns a
// {count, requests} envelope. A regression back to GET surfaces here
// as 405 from the mux; a regression that drops the envelope unwrap
// surfaces as a missing id in the result text.
//
// The other 11 tools in the time_off group are not currently fixed
// nor pinned; this file deliberately stays scoped to the
// list-requests handler.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func newTimeOffUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/workspaces/test-workspace/time-off/requests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		// Sanity-check the camelCase pageSize was sent in the body.
		if _, ok := body["pageSize"]; !ok {
			t.Fatalf("expected pageSize in body, got %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"requests":[{"id":"r-1","policyId":"p-1","status":{"statusType":"PENDING"}}]}`))
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_TimeOff_ListRequestsPostsAndUnwrapsEnvelope(t *testing.T) {
	upstream := newTimeOffUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "time_off",
		Tool:     "clockify_list_time_off_requests",
		Args:     map[string]any{"page": 1, "page_size": 25, "status": "PENDING"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("list did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"id":"r-1"`) {
		t.Fatalf("list result missing request id: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"total":1`) {
		t.Fatalf("list result missing meta.total=1: %q", res.ResultText)
	}
}
