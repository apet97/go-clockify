package tools_test

// Dispatcher-level coverage for the Tier 2 approvals group. Each test drives
// the full MCP pipeline (initialize → tools/call → enforcement → handler →
// fake upstream) via dispatchTier2 so the registered handler closures, the
// schema gate, and the policy gate are all exercised end to end. Direct
// svc.submitForApproval(...)-style calls are intentionally avoided — they
// would skip the dispatcher and miss the very layers this test file is
// supposed to cover.
//
// The fake upstream is a tiny http.ServeMux with one branch per Clockify
// endpoint the approvals handlers touch; that keeps the per-test setup
// declarative and the failure messages legible.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func newApprovalsUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/workspaces/test-workspace/approval-requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page <= 1 {
				_, _ = w.Write([]byte(`[{"id":"ar-1","status":"PENDING","dateRange":{"start":"2026-04-01","end":"2026-04-07"},"owner":{"id":"u1","name":"Owner"},"creator":{"id":"u2","name":"Creator"}}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, hasStatus := body["status"]; hasStatus {
				t.Fatalf("submit must use documented period/periodStart body, got %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":"ar-new","status":"PENDING"}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/workspaces/test-workspace/approval-requests/ar-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPatch:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if _, hasStatus := body["status"]; hasStatus {
				t.Fatalf("approval status updates must use documented state body, got %#v", body)
			}
			if _, hasState := body["state"]; !hasState {
				t.Fatalf("approval PATCH missing state: %#v", body)
			}
			body["id"] = "ar-1"
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/workspaces/test-workspace/approval-requests/resubmit-entries-for-approval", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"id":"ar-resubmitted","status":"PENDING"}`))
	})

	userID := "aaaaaaaaaaaaaaaaaaaaaaaa"
	mux.HandleFunc("/workspaces/test-workspace/approval-requests/users/"+userID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"id":"ar-user-new","status":"PENDING","owner":{"id":"aaaaaaaaaaaaaaaaaaaaaaaa"}}`))
	})

	mux.HandleFunc("/workspaces/test-workspace/approval-requests/users/"+userID+"/resubmit-entries-for-approval", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"id":"ar-user-resubmitted","status":"PENDING","owner":{"id":"aaaaaaaaaaaaaaaaaaaaaaaa"}}`))
	})

	return testharness.NewFakeClockify(t, mux)
}

func TestTier2Dispatch_Approvals_ListAndGet(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_list_approval_requests",
		Args:     map[string]any{"status": "PENDING", "page": 1, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("list did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "ar-1") {
		t.Fatalf("list result missing id: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_get_approval_request",
		Args:     map[string]any{"approval_id": "ar-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "ar-1") {
		t.Fatalf("get result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Approvals_GetMissingSurfacesToolError(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_get_approval_request",
		Args:     map[string]any{"approval_id": "missing"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeToolError {
		t.Fatalf("expected tool_error for missing approval, got %q (err=%q)", res.Outcome, res.ErrorMessage)
	}
	if !res.UpstreamHit {
		t.Fatalf("404 path should still hit upstream")
	}
}

func TestTier2Dispatch_Approvals_SubmitForApproval(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "approvals",
		Tool:  "clockify_submit_for_approval",
		Args: map[string]any{
			"period":       "WEEKLY",
			"period_start": "2026-04-01T00:00:00.000Z",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("submit outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("submit did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "ar-new") {
		t.Fatalf("submit result missing created id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Approvals_ResubmitAndUserSubmitTools(t *testing.T) {
	upstream := newApprovalsUpstream(t)
	userID := "aaaaaaaaaaaaaaaaaaaaaaaa"

	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"clockify_resubmit_for_approval", map[string]any{"period": "WEEKLY", "period_start": "2026-04-01T00:00:00.000Z"}, "ar-resubmitted"},
		{"clockify_submit_for_user_approval", map[string]any{"user_id": userID, "period": "WEEKLY", "period_start": "2026-04-01T00:00:00.000Z"}, "ar-user-new"},
		{"clockify_resubmit_for_user_approval", map[string]any{"user_id": userID, "period": "WEEKLY", "period_start": "2026-04-01T00:00:00.000Z"}, "ar-user-resubmitted"},
	}
	for _, tc := range cases {
		res := dispatchTier2(t, tier2InvokeOpts{
			Group:    "approvals",
			Tool:     tc.tool,
			Args:     tc.args,
			Upstream: upstream,
		})
		if res.Outcome != testharness.OutcomeSuccess {
			t.Fatalf("%s outcome=%q err=%q raw=%s", tc.tool, res.Outcome, res.ErrorMessage, string(res.Raw))
		}
		if !strings.Contains(res.ResultText, tc.want) {
			t.Fatalf("%s result missing %s: %q", tc.tool, tc.want, res.ResultText)
		}
	}
}

func TestTier2Dispatch_Approvals_ApproveAndDryRun(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	// Dry run path scans the documented list endpoint and surfaces a preview
	// envelope without calling the per-id endpoint.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_approve_timesheet",
		Args:     map[string]any{"approval_id": "ar-1", "dry_run": true},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("approve(dry_run) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "ar-1") {
		t.Fatalf("approve(dry_run) result missing id: %q", res.ResultText)
	}

	// Live path issues documented PATCH with state=APPROVED.
	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_approve_timesheet",
		Args:     map[string]any{"approval_id": "ar-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("approve(live) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "APPROVED") {
		t.Fatalf("approve(live) did not echo APPROVED status: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Approvals_RejectAndDryRun(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_reject_timesheet",
		Args:     map[string]any{"approval_id": "ar-1", "dry_run": true},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("reject(dry_run) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group: "approvals",
		Tool:  "clockify_reject_timesheet",
		Args: map[string]any{
			"approval_id": "ar-1",
			"reason":      "incomplete entries",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("reject(live) outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "REJECTED") {
		t.Fatalf("reject(live) did not echo REJECTED: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Approvals_Withdraw(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_withdraw_approval",
		Args:     map[string]any{"approval_id": "ar-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("withdraw outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "WITHDRAWN_SUBMISSION") {
		t.Fatalf("withdraw did not echo WITHDRAWN_SUBMISSION: %q", res.ResultText)
	}
}

func TestTier2Dispatch_Approvals_SchemaValidation(t *testing.T) {
	upstream := newApprovalsUpstream(t)

	// Missing required approval_id triggers schema validation, which runs
	// before the handler — UpstreamHit must be false.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "approvals",
		Tool:     "clockify_get_approval_request",
		Args:     map[string]any{},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeInvalidParams {
		t.Fatalf("expected invalid_params, got %q (err=%q)", res.Outcome, res.ErrorMessage)
	}
	if res.UpstreamHit {
		t.Fatalf("schema-rejected call must not reach upstream")
	}
}
