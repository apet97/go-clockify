package tools_test

// Dispatcher-level coverage for the Tier 2 shared_reports group. Pins
// SUMMARY rev 3 #3 / #16 / #17 plus open-questions #5 / #6:
//
//   - listSharedReports must unwrap the {reports:[…], count} envelope
//     and surface `count` in the result meta. The query param is
//     pageSize (camelCase), not page-size.
//
//   - getSharedReport hits the bare-id path /shared-reports/{id} (no
//     workspace segment) and sends ?exportType=JSON_V1 so the upstream
//     returns JSON instead of PDF/CSV/XLSX.
//
// Reports endpoints normally route through the reports.api.clockify.me
// host. In tests the FakeClockify URL is shared between primary and
// reports baseURL because Client.ReportsBaseURL() falls through to the
// primary baseURL whenever the primary URL doesn't match the canonical
// production host — see internal/clockify/client.go ReportsBaseURL.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func newSharedReportsUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	// LIST — reports/{count} envelope. Asserts the camelCase pageSize
	// query reaches the upstream; the stale page-size variant is
	// ignored by the handler post-fix and would never be set here.
	mux.HandleFunc("/workspaces/test-workspace/shared-reports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if got := r.URL.Query().Get("pageSize"); got != "25" {
			t.Fatalf("expected ?pageSize=25, got %q (full query=%q)", got, r.URL.RawQuery)
		}
		if got := r.URL.Query().Get("page-size"); got != "" {
			t.Fatalf("legacy page-size must not be sent, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"reports":[{"id":"sr-1","name":"Weekly summary","type":"SUMMARY","isPublic":true}],"count":74}`))
	})

	// GET — bare-id path without workspace segment, with
	// exportType=JSON_V1 query enforced.
	mux.HandleFunc("/shared-reports/sr-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if got := r.URL.Query().Get("exportType"); got != "JSON_V1" {
			t.Fatalf("expected ?exportType=JSON_V1, got %q (full query=%q)", got, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sr-1","totals":[],"groupOne":[],"filters":{"name":"Weekly summary"}}`))
	})

	// Workspace-prefixed single-get path must NOT be hit; the handler
	// uses the bare-id path. If the test reaches here, the path
	// regression is back.
	mux.HandleFunc("/workspaces/test-workspace/shared-reports/sr-1", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("getSharedReport must not hit the workspace-prefixed path: %s %s", r.Method, r.URL.Path)
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_SharedReports_List(t *testing.T) {
	upstream := newSharedReportsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "shared_reports",
		Tool:     "clockify_list_shared_reports",
		Args:     map[string]any{"page": 1, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("list did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"id":"sr-1"`) {
		t.Fatalf("list result missing id: %q", res.ResultText)
	}
	// Inner envelope's count should be exposed in meta.total; the
	// `count` meta key reflects the slice length the handler returns.
	if !strings.Contains(res.ResultText, `"total":74`) {
		t.Fatalf("list result missing meta.total=74: %q", res.ResultText)
	}
}

func TestTier2Dispatch_SharedReports_Get(t *testing.T) {
	upstream := newSharedReportsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "shared_reports",
		Tool:     "clockify_get_shared_report",
		Args:     map[string]any{"report_id": "sr-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("get did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"id":"sr-1"`) {
		t.Fatalf("get result missing id: %q", res.ResultText)
	}
}
