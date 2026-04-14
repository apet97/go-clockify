package tools_test

// Dispatcher-level coverage for the Tier 2 project_admin group: project
// templates (list / get / create), per-project estimates, membership
// rewrites, and bulk archival. The fake upstream covers every endpoint
// the six handlers touch so each tool gets at least one happy-path
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
			_, _ = w.Write([]byte(`[{"id":"tpl-1","name":"Standard","isTemplate":true}]`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"tpl-new","name":"Created","isTemplate":true}`))
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Per-project endpoint serves: GET (template fetch / archive merge),
	// PUT (estimate update / archive flag).
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

	// Memberships endpoint.
	mux.HandleFunc("/workspaces/test-workspace/projects/p-1/memberships", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]any{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		body["id"] = "p-1"
		_ = json.NewEncoder(w).Encode(body)
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

func TestTier2Dispatch_ProjectAdmin_TemplatesListAndGet(t *testing.T) {
	upstream := newProjectAdminUpstream(t)

	res := dispatchTier2(t, tier2InvokeOpts{
		Group:    "project_admin",
		Tool:     "clockify_list_project_templates",
		Args:     map[string]any{},
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
