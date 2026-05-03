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
//
// Write/export pinning (rev 4 — 2026-05-03 lab probes):
//
//   - createSharedReport must POST to the workspace-prefixed path with
//     body keys "type" (NOT "reportType") and "filter" (NOT "filters",
//     the upstream DTO is ReportFilterV1). filter.exportType,
//     filter.dateRangeStart, filter.dateRangeEnd are required.
//
//   - updateSharedReport must PUT to the workspace-prefixed per-id path
//     with the same body-key conventions. Bare-id PUT/PATCH both return
//     405; the previous "everything per-id is bare" generalisation was
//     wrong.
//
//   - deleteSharedReport must DELETE to the workspace-prefixed per-id
//     path and tolerate a 204 with empty body.
//
//   - exportSharedReport must GET the bare /shared-reports/{id} path
//     (no workspace) with ?exportType=PDF|CSV|XLSX|JSON_V1 (NOT a
//     /export segment with ?format=). For non-JSON exports the handler
//     must return a binary-aware envelope {contentType, filename,
//     bytes, body(base64)}; JSON_V1 stays decoded as a JSON map.

import (
	"encoding/base64"
	"encoding/json"
	"io"
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

// ---------------------------------------------------------------------------
// Write / export dispatch tests (rev 4 pin: SUMMARY changes #24-#27).
// ---------------------------------------------------------------------------

const (
	srWriteWS         = "test-workspace"
	srWriteCreatedID  = "sr-write-2"
	srWriteCreateName = "ws-prefixed create"
	srWriteRenamedTo  = "ws-prefixed rename"
)

// fakePDF is a 4-byte body the export tests use to verify the
// binary-aware envelope. It's deliberately not JSON; if the handler
// tries to json.Unmarshal it, the test will fail with a parse error.
var fakePDF = []byte{'%', 'P', 'D', 'F'}

// newSharedReportsWriteUpstream registers handlers that pin the four
// write/export tools to the canonical request shapes discovered by
// the lab on 2026-05-03 (probes/shared-reports-write.sh).
//
// Any request that lands on a path/method combination the lab proved
// is wrong (bare-id PUT/PATCH/DELETE, ws-prefixed bare GET, /export
// segment, etc.) calls t.Fatalf so a regression that re-introduces
// the wrong wiring fails loudly.
func newSharedReportsWriteUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	// CREATE — POST /workspaces/{ws}/shared-reports.
	// Body must use "type" (not "reportType") and "filter" (not
	// "filters"); filter must contain exportType/dateRangeStart/
	// dateRangeEnd. Reply with the canonical create response shape.
	mux.HandleFunc("/workspaces/"+srWriteWS+"/shared-reports", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("create: expected POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("create: read body: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("create: body not JSON: %v (raw=%s)", err, string(body))
		}
		if _, has := got["reportType"]; has {
			t.Fatalf("create: stale field 'reportType' present (must be 'type'); body=%s", string(body))
		}
		if _, has := got["filters"]; has {
			t.Fatalf("create: stale field 'filters' present (must be 'filter'); body=%s", string(body))
		}
		if got["type"] == nil || got["type"] == "" {
			t.Fatalf("create: missing required 'type'; body=%s", string(body))
		}
		filter, ok := got["filter"].(map[string]any)
		if !ok {
			t.Fatalf("create: 'filter' missing or not an object; body=%s", string(body))
		}
		for _, k := range []string{"exportType", "dateRangeStart", "dateRangeEnd"} {
			if v, has := filter[k]; !has || v == nil || v == "" {
				t.Fatalf("create: filter.%s required; body=%s", k, string(body))
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"` + srWriteCreatedID + `","workspaceId":"` + srWriteWS +
			`","name":"` + srWriteCreateName + `","type":"SUMMARY","filter":{"exportType":"JSON_V1"}}`))
	})

	// UPDATE — PUT /workspaces/{ws}/shared-reports/{id}.
	// DELETE — DELETE /workspaces/{ws}/shared-reports/{id}.
	mux.HandleFunc("/workspaces/"+srWriteWS+"/shared-reports/"+srWriteCreatedID, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("update: read body: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("update: body not JSON: %v (raw=%s)", err, string(body))
			}
			if _, has := got["reportType"]; has {
				t.Fatalf("update: stale field 'reportType' present (must be 'type'); body=%s", string(body))
			}
			if _, has := got["filters"]; has {
				t.Fatalf("update: stale field 'filters' present (must be 'filter'); body=%s", string(body))
			}
			if got["name"] != srWriteRenamedTo {
				t.Fatalf("update: expected name=%q, got %v; body=%s", srWriteRenamedTo, got["name"], string(body))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + srWriteCreatedID + `","workspaceId":"` + srWriteWS +
				`","name":"` + srWriteRenamedTo + `","type":"SUMMARY","filter":{"exportType":"JSON_V1"}}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			t.Fatalf("update/delete handler: GET landed on workspace-prefixed per-id path; live API returns 405. handler regression.")
		default:
			t.Fatalf("update/delete handler: unsupported method %s on %s", r.Method, r.URL.Path)
		}
	})

	// EXPORT — GET /shared-reports/{id} (NO workspace segment) with
	// ?exportType=PDF|CSV|XLSX|JSON_V1.
	mux.HandleFunc("/shared-reports/"+srWriteCreatedID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("export: expected GET on bare-id path, got %s", r.Method)
		}
		exportType := r.URL.Query().Get("exportType")
		if exportType == "" {
			t.Fatalf("export: missing ?exportType= query (full=%q)", r.URL.RawQuery)
		}
		if r.URL.Query().Get("format") != "" {
			t.Fatalf("export: stale ?format= query present (must be exportType); full=%q", r.URL.RawQuery)
		}
		switch exportType {
		case "JSON_V1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"` + srWriteCreatedID + `","totals":[],"filter":{"name":"json round-trip"}}`))
		case "PDF":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Disposition", "filename=Clockify_Time_Report_Summary_11%2F15%2F2023-12%2F07%2F2023.pdf")
			_, _ = w.Write(fakePDF)
		default:
			t.Fatalf("export: unexpected exportType %q in this test", exportType)
		}
	})

	// Trip-wires for paths the handler must NEVER hit.
	mux.HandleFunc("/workspaces/"+srWriteWS+"/shared-reports/"+srWriteCreatedID+"/export", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("export: handler hit /export segment (live API returns 404). regression — drop the /export suffix.")
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_SharedReports_Create(t *testing.T) {
	upstream := newSharedReportsWriteUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "shared_reports",
		Tool:  "clockify_create_shared_report",
		Args: map[string]any{
			"name":        srWriteCreateName,
			"report_type": "SUMMARY",
			"filter": map[string]any{
				"exportType":     "JSON_V1",
				"dateRangeStart": "2026-04-01T00:00:00Z",
				"dateRangeEnd":   "2026-04-30T23:59:59Z",
				"summaryFilter": map[string]any{
					"groups":     []string{"PROJECT"},
					"sortColumn": "GROUP",
				},
			},
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("create did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"id":"`+srWriteCreatedID+`"`) {
		t.Fatalf("create result missing created id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_SharedReports_Update(t *testing.T) {
	upstream := newSharedReportsWriteUpstream(t)

	// Pass report_type AND filter so the upstream body assertion can
	// verify both fields end up under the renamed keys ("type" and
	// "filter"). Without these args the previous handler's "filters"/
	// "reportType" body keys would be silently absent and the test
	// would pass for the wrong reason.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "shared_reports",
		Tool:  "clockify_update_shared_report",
		Args: map[string]any{
			"report_id":   srWriteCreatedID,
			"name":        srWriteRenamedTo,
			"report_type": "SUMMARY",
			"filter": map[string]any{
				"exportType":     "JSON_V1",
				"dateRangeStart": "2026-04-01T00:00:00Z",
				"dateRangeEnd":   "2026-04-30T23:59:59Z",
			},
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("update outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("update did not reach upstream")
	}
	if !strings.Contains(res.ResultText, srWriteRenamedTo) {
		t.Fatalf("update result missing renamed name: %q", res.ResultText)
	}
}

func TestTier2Dispatch_SharedReports_Delete(t *testing.T) {
	upstream := newSharedReportsWriteUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "shared_reports",
		Tool:  "clockify_delete_shared_report",
		Args: map[string]any{
			"report_id": srWriteCreatedID,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("delete outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("delete did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"deleted":true`) {
		t.Fatalf("delete result missing deleted flag: %q", res.ResultText)
	}
}

func TestTier2Dispatch_SharedReports_ExportPDF(t *testing.T) {
	upstream := newSharedReportsWriteUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "shared_reports",
		Tool:  "clockify_export_shared_report",
		Args: map[string]any{
			"report_id": srWriteCreatedID,
			"format":    "pdf",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("export-pdf outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("export-pdf did not reach upstream")
	}
	if !strings.Contains(res.ResultText, `"contentType":"application/pdf"`) {
		t.Fatalf("export-pdf missing contentType: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"filename":"Clockify_Time_Report_Summary_11/15/2023-12/07/2023.pdf"`) {
		t.Fatalf("export-pdf filename not URL-decoded: %q", res.ResultText)
	}
	expectedB64 := base64.StdEncoding.EncodeToString(fakePDF)
	if !strings.Contains(res.ResultText, `"body":"`+expectedB64+`"`) {
		t.Fatalf("export-pdf body not base64-encoded: want substring %q in %q", expectedB64, res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"bytes":4`) {
		t.Fatalf("export-pdf bytes count missing: %q", res.ResultText)
	}
}

func TestTier2Dispatch_SharedReports_ExportJSONStaysDecoded(t *testing.T) {
	upstream := newSharedReportsWriteUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "shared_reports",
		Tool:  "clockify_export_shared_report",
		Args: map[string]any{
			"report_id": srWriteCreatedID,
			"format":    "json",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("export-json outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	// JSON_V1 must NOT be base64-wrapped — it stays a structured map.
	if strings.Contains(res.ResultText, `"body":"`) {
		t.Fatalf("export-json must stay decoded (not wrapped in base64 envelope): %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"id":"`+srWriteCreatedID+`"`) {
		t.Fatalf("export-json missing decoded report id: %q", res.ResultText)
	}
}
