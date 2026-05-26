package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestStartTimer_ForceProjectsBlocksProjectlessStart(t *testing.T) {
	postIssued := false
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{})
		case r.URL.Path == "/workspaces/ws1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Workspace{ID: "ws1", WorkspaceSettings: map[string]any{"forceProjects": true}})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			postIssued = true
			respondJSON(t, w, clockify.TimeEntry{ID: "should-not-create"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.StartTimerArgs(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "requires a project") {
		t.Fatalf("expected requires a project error, got %v", err)
	}
	if postIssued {
		t.Fatal("timer POST was issued despite forceProjects projectless guard")
	}
}

func TestTimerStartInheritsProjectBillable(t *testing.T) {
	var postBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects/p1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{ID: "p1", Billable: true})
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("decode timer post body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "te1", ProjectID: "p1", Billable: true})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.StartTimerArgs(context.Background(), map[string]any{"project_id": "p1"})
	mustOK(t, res, err, oneUserToolEntriesTimerStart)
	if postBody["billable"] != true {
		t.Fatalf("timer POST billable = %#v, want true (body %#v)", postBody["billable"], postBody)
	}
}

func TestStartTimerForwardsCustomFields(t *testing.T) {
	var postBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects/p1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{ID: "p1", Billable: true})
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{})
		case r.URL.Path == "/workspaces/ws1/time-entries" && r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("decode timer post body: %v", err)
			}
			respondJSON(t, w, clockify.TimeEntry{ID: "te1", ProjectID: "p1", Billable: true})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	res, err := svc.StartTimerArgs(context.Background(), map[string]any{
		"project_id": "p1",
		"custom_fields": []any{
			map[string]any{"field_id": "6a00f6bc2568d3d293061e2a", "value": "Site A"},
		},
	})
	mustOK(t, res, err, oneUserToolEntriesTimerStart)
	fields, ok := postBody["customFields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("timer POST customFields = %T %#v, want one value", postBody["customFields"], postBody["customFields"])
	}
	field, _ := fields[0].(map[string]any)
	if field["customFieldId"] != "6a00f6bc2568d3d293061e2a" || field["value"] != "Site A" {
		t.Fatalf("timer POST custom field not normalized: %#v", field)
	}
}

func TestStartTimer_RunningTimerBlocks(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects/p1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Project{ID: "p1", Billable: true})
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{{
				ID:           "running1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-16T10:00:00Z"},
			}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.StartTimerArgs(context.Background(), map[string]any{"project_id": "p1"})
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("expected already running error, got %v", err)
	}
}

func TestStopTimer_ProjectErrorIsExplained(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.TimeEntry{{
				ID:           "running1",
				UserID:       "u1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-16T10:00:00Z"},
			}})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodPatch:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":"Project is either required field or given project is archived","code":400}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	_, err := svc.StopTimer(context.Background(), map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "assign a project with clockify_entries_update") {
		t.Fatalf("expected project recovery error, got %v", err)
	}
}

func TestStopTimerRetriesTransientEmptyRunningList(t *testing.T) {
	oldDelays := stopTimerNoRunningRetryDelays
	stopTimerNoRunningRetryDelays = []time.Duration{0}
	t.Cleanup(func() {
		stopTimerNoRunningRetryDelays = oldDelays
	})

	runningReads := 0
	patches := 0
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/user" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.User{ID: "u1", Name: "Test"})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodGet:
			runningReads++
			if runningReads == 1 {
				respondJSON(t, w, []clockify.TimeEntry{})
				return
			}
			respondJSON(t, w, []clockify.TimeEntry{{
				ID:           "running1",
				UserID:       "u1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-16T10:00:00Z"},
			}})
		case r.URL.Path == "/workspaces/ws1/user/u1/time-entries" && r.Method == http.MethodPatch:
			patches++
			respondJSON(t, w, clockify.TimeEntry{
				ID:           "running1",
				UserID:       "u1",
				TimeInterval: clockify.TimeInterval{Start: "2026-05-16T10:00:00Z", End: "2026-05-16T10:15:00Z"},
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	out, err := svc.StopTimer(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("stop timer: %v", err)
	}
	env, ok := out.(ToolResult)
	if !ok {
		t.Fatalf("result type = %T, want ToolResult", out)
	}
	entry, ok := env.Data.(EntryView)
	if !ok || entry.ID != "running1" {
		t.Fatalf("stopped entry = %T %#v, want running1", env.Data, env.Data)
	}
	if runningReads != 2 {
		t.Fatalf("running timer reads = %d, want 2", runningReads)
	}
	if patches != 1 {
		t.Fatalf("patches = %d, want 1", patches)
	}
}
