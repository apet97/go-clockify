package tools_test

// Dispatcher-level coverage for the Tier 2 project_admin group: project
// templates (list / get / create), per-project estimates, membership
// rewrites, project/user/task rates, and bulk archival. The fake upstream
// covers every endpoint the project_admin handlers touch so each tool gets at
// least one happy-path
// exercise through the real MCP dispatch pipeline.
//
// Bulk-archive is interesting because the handler keeps going on per-item
// errors and accumulates a per-id status map — the test asserts both the
// happy path and a mixed-result path so the loop body is covered too.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/testharness"
)

func newProjectAdminUpstream(t *testing.T) *testharness.FakeClockify {
	t.Helper()
	mux := http.NewServeMux()

	// List + create projects (templates use the same endpoint with is-template=true).
	mux.HandleFunc("/workspaces/test-workspace/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			if q.Get("is-template") != "true" || q.Get("page") != "2" || q.Get("page-size") != "25" {
				t.Fatalf("unexpected template-list query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":"tpl-1","name":"Standard","isTemplate":true}]`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"tpl-new","name":"Created","isTemplate":true}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/workspaces/test-workspace/projects/from-template", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["templateProjectId"] != "tpl-1" || body["name"] != "From Template" {
			t.Fatalf("unexpected from-template body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"p-from-template","name":"From Template","template":false}`))
	})

	// Per-project endpoint serves: GET (template fetch / archive merge),
	// PUT (archive flag).
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"p-1","name":"Active project","archived":false}`))
		case http.MethodPut:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "p-1"
			_ = json.NewEncoder(w).Encode(body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/estimate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["budgetEstimate"]; !ok {
			t.Fatalf("estimate patch body missing budgetEstimate: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		body["id"] = "p-1"
		_ = json.NewEncoder(w).Encode(body)
	})

	// Memberships endpoint. PATCH-only to pin SUMMARY rev 3 #6: the
	// upstream returns the full project object (not a bare membership
	// array), and PUT is rejected with 405.
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/memberships", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch && r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.Method {
		case http.MethodPatch:
			if _, ok := body["memberships"]; !ok {
				t.Fatalf("PATCH memberships missing memberships: %+v", body)
			}
		case http.MethodPost:
			if _, ok := body["userIds"]; !ok {
				t.Fatalf("POST memberships missing userIds: %+v", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		// Echo the requested members back in the membershipType-shaped
		// schema the live API returns. The handler under test should
		// hand back this inner array, not the wrapping project object.
		ms, _ := body["memberships"].([]any)
		out := map[string]any{
			"id":          "p-1",
			"name":        "Active project",
			"workspaceId": "test-workspace",
			"memberships": ms,
		}
		_ = json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/template", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["isTemplate"] != true {
			t.Fatalf("unexpected template body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"p-1","template":true}`))
	})

	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/users/u-1/cost-rate", func(w http.ResponseWriter, r *http.Request) {
		assertProjectAdminRateRequest(t, w, r, "costRate")
	})
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/users/u-1/hourly-rate", func(w http.ResponseWriter, r *http.Request) {
		assertProjectAdminRateRequest(t, w, r, "hourlyRate")
	})
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/tasks/t-1/cost-rate", func(w http.ResponseWriter, r *http.Request) {
		assertProjectAdminRateRequest(t, w, r, "costRate")
	})
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/tasks/t-1/hourly-rate", func(w http.ResponseWriter, r *http.Request) {
		assertProjectAdminRateRequest(t, w, r, "hourlyRate")
	})

	// Second project — used by the archive bulk happy path so the test can
	// assert "two ids in, two ids out".
	mux.HandleFunc("/workspaces/test-workspace/projects/p-2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["id"] = "p-2"
		_ = json.NewEncoder(w).Encode(body)
	})

	// Failing project — used to cover the archive error-accumulation branch.
	mux.HandleFunc("/workspaces/test-workspace/projects/p-fail", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	})

	return testharness.NewFakeClockify(t, mux)
}

func assertProjectAdminRateRequest(t *testing.T, w http.ResponseWriter, r *http.Request, responseKey string) {
	t.Helper()
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body := map[string]any{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body["amount"] != float64(2500) || body["since"] != "2026-05-12T00:00:00Z" {
		t.Fatalf("unexpected rate body: %+v", body)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          "p-1",
		responseKey:   body,
		"workspaceId": "test-workspace",
	})
}

func TestTier2Dispatch_ProjectAdmin_TemplatesListAndGet(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_list_project_templates",
		Args:     map[string]any{"page": 2, "page_size": 25},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("list outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "tpl-1") {
		t.Fatalf("list result missing template id: %q", res.ResultText)
	}

	res = dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_get_project_template",
		Args:     map[string]any{"project_id": "p-1"},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("get outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "p-1") {
		t.Fatalf("get result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_CreateTemplate(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_create_project_template",
		Args: map[string]any{
			"name":      "New Template",
			"color":     "#abcdef",
			"billable":  true,
			"is_public": false,
			"client_id": "c-1",
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("create outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("create did not reach upstream")
	}
	if !strings.Contains(res.ResultText, "tpl-new") {
		t.Fatalf("create result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_CreateProjectFromTemplate(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_create_project_from_template",
		Args: map[string]any{
			"name":                "From Template",
			"template_project_id": "tpl-1",
			"client_id":           "c-1",
			"color":               "#abcdef",
			"is_public":           true,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("from_template outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "p-from-template") {
		t.Fatalf("from_template result missing new id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_UpdateProjectEstimate(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_update_project_estimate",
		Args: map[string]any{
			"project_id":     "p-1",
			"estimate_type":  "BUDGET",
			"estimate_value": 5000.0,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("update_estimate outcome=%q err=%q", res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "p-1") {
		t.Fatalf("update_estimate result missing id: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_SetProjectMemberships(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_set_project_memberships",
		Args: map[string]any{
			"project_id":  "p-1",
			"user_ids":    []any{"u-1", "u-2"},
			"hourly_rate": 75.0,
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("memberships outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !res.UpstreamHit {
		t.Fatalf("memberships did not reach upstream")
	}
	// SUMMARY rev 3 #6 (b): the handler must return the inner
	// memberships array, not the full project object. Both userIds we
	// requested should appear; the wrapping project's "id":"p-1" must
	// not be at the top level of Data.
	if !strings.Contains(res.ResultText, `"userId":"u-1"`) {
		t.Fatalf("set memberships did not surface u-1 in payload: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"userId":"u-2"`) {
		t.Fatalf("set memberships did not surface u-2 in payload: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_DocumentedMembershipAndTemplateTools(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	update := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_update_project_memberships",
		Args: map[string]any{
			"project_id": "p-1",
			"memberships": []any{map[string]any{
				"user_id":           "u-1",
				"membership_status": "ACTIVE",
				"hourly_rate":       map[string]any{"amount": 2500.0, "since": "2026-05-12T00:00:00Z"},
			}},
			"user_groups": map[string]any{"ids": []any{"g-1"}, "status": "ACTIVE", "contains": "CONTAINS"},
		},
		Upstream: upstream,
	})
	if update.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("update memberships outcome=%q err=%q raw=%s", update.Outcome, update.ErrorMessage, string(update.Raw))
	}
	if !strings.Contains(update.ResultText, `"membershipStatus":"ACTIVE"`) {
		t.Fatalf("update memberships result missing status: %q", update.ResultText)
	}

	assign := dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_assign_project_memberships",
		Args:     map[string]any{"project_id": "p-1", "remove": false, "user_ids": []any{"u-1"}},
		Upstream: upstream,
	})
	if assign.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("assign memberships outcome=%q err=%q raw=%s", assign.Outcome, assign.ErrorMessage, string(assign.Raw))
	}

	tpl := dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_update_project_template",
		Args:     map[string]any{"project_id": "p-1", "is_template": true},
		Upstream: upstream,
	})
	if tpl.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("update template outcome=%q err=%q raw=%s", tpl.Outcome, tpl.ErrorMessage, string(tpl.Raw))
	}
}

func TestTier2Dispatch_ProjectAdmin_DocumentedRateTools(t *testing.T) {
	upstream := newProjectAdminUpstream(t)
	cases := []struct {
		tool string
		args map[string]any
	}{
		{
			tool: "clockify_update_project_user_cost_rate",
			args: map[string]any{"project_id": "p-1", "user_id": "u-1", "amount": 2500.0, "since": "2026-05-12T00:00:00Z"},
		},
		{
			tool: "clockify_update_project_user_hourly_rate",
			args: map[string]any{"project_id": "p-1", "user_id": "u-1", "amount": 2500.0, "since": "2026-05-12T00:00:00Z"},
		},
		{
			tool: "clockify_update_task_cost_rate",
			args: map[string]any{"project_id": "p-1", "task_id": "t-1", "amount": 2500.0, "since": "2026-05-12T00:00:00Z"},
		},
		{
			tool: "clockify_update_task_hourly_rate",
			args: map[string]any{"project_id": "p-1", "task_id": "t-1", "amount": 2500.0, "since": "2026-05-12T00:00:00Z"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			res := dispatchTier2(t, tier2InvokeOpts{
				Group:    "project_admin",
				Tool:     tc.tool,
				Args:     tc.args,
				Upstream: upstream,
			})
			if res.Outcome != testharness.OutcomeSuccess {
				t.Fatalf("%s outcome=%q err=%q raw=%s", tc.tool, res.Outcome, res.ErrorMessage, string(res.Raw))
			}
			if !res.UpstreamHit {
				t.Fatalf("%s did not reach upstream", tc.tool)
			}
		})
	}
}

func TestTier2Dispatch_ProjectAdmin_ArchiveProjectsHappy(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_archive_projects",
		Args: map[string]any{
			"project_ids": []any{"p-1", "p-2"},
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("archive outcome=%q err=%q raw=%s", res.Outcome, res.ErrorMessage, string(res.Raw))
	}
	if !strings.Contains(res.ResultText, "p-1") || !strings.Contains(res.ResultText, "p-2") {
		t.Fatalf("archive result missing ids: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"archived":true`) {
		t.Fatalf("archive result missing archived flag: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_ArchiveProjectsMixedFailure(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	// One id succeeds, one returns 403 from the fake — handler must NOT
	// abort the batch and the per-id status map must surface both results.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group: "project_admin",
		Tool:  "clockify_archive_projects",
		Args: map[string]any{
			"project_ids": []any{"p-1", "p-fail"},
		},
		Upstream: upstream,
	})
	if res.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("archive outcome=%q err=%q (handler should aggregate not error)",
			res.Outcome, res.ErrorMessage)
	}
	if !strings.Contains(res.ResultText, "p-fail") {
		t.Fatalf("expected per-id failure entry for p-fail: %q", res.ResultText)
	}
	if !strings.Contains(res.ResultText, `"archived":false`) {
		t.Fatalf("expected per-id failure flag in result: %q", res.ResultText)
	}
}

func TestTier2Dispatch_ProjectAdmin_SchemaValidation(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	// Missing required project_id on get_project_template.
	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_get_project_template",
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
