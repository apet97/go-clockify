package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestBillableTriStateFromEntryAndReportRows(t *testing.T) {
	var omitted clockify.TimeEntry
	if err := json.Unmarshal([]byte(`{"id":"e1","timeInterval":{"start":"2026-05-01T09:00:00Z","end":"2026-05-01T10:00:00Z"}}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if view := entryViewFromEntry(omitted); view.BillablePresent || view.BillableState != billableStateUnset {
		t.Fatalf("omitted billable should be unset, got %#v", view)
	}
	var explicitFalse clockify.TimeEntry
	if err := json.Unmarshal([]byte(`{"id":"e2","billable":false,"timeInterval":{"start":"2026-05-01T09:00:00Z","end":"2026-05-01T10:00:00Z"}}`), &explicitFalse); err != nil {
		t.Fatal(err)
	}
	if view := entryViewFromEntry(explicitFalse); !view.BillablePresent || view.BillableState != billableStateNonBillable {
		t.Fatalf("explicit false should be nonbillable, got %#v", view)
	}
	report := reportEntryViewFromRow(map[string]any{"id": "e3", "billable": "true"})
	if report.BillableState != billableStateBillable || !report.BillablePresent {
		t.Fatalf("report string billable not normalized: %#v", report)
	}
}

func TestDryRunStopTimerNoRunningValidationFailed(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.StopTimer(context.Background(), map[string]any{"dry_run": true})
	if err != nil {
		t.Fatalf("stop timer dry run: %v", err)
	}
	env := result.(ResultEnvelope)
	data := env.Data.(map[string]any)
	validation := data["validation"].(ValidationView)
	if validation.Status != validationStatusFailed {
		t.Fatalf("expected failed validation, got %#v", validation)
	}
}

func TestQuickReportProjectResolverHydratesMissingProjectName(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			respondJSON(t, w, clockify.User{ID: "u1"})
		case "/workspaces/ws1/user/u1/time-entries":
			respondJSON(t, w, []clockify.TimeEntry{{ID: "e1", ProjectID: "p1", TimeInterval: clockify.TimeInterval{Start: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), End: time.Now().UTC().Format(time.RFC3339)}}})
		case "/workspaces/ws1/projects":
			respondJSON(t, w, []clockify.Project{{ID: "p1", Name: "Named Project", ClientID: "c1", ClientName: "Client A"}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.QuickReport(context.Background(), map[string]any{"days": 1})
	if err != nil {
		t.Fatalf("quick report: %v", err)
	}
	data := result.Data.(QuickReportData)
	if data.TopProject == nil || data.TopProject.ProjectName != "Named Project" || data.TopProject.ClientName != "Client A" {
		t.Fatalf("project/client not resolved: %#v", data.TopProject)
	}
}

func TestWeeklyFutureRowsDefaultOmittedAndIncluded(t *testing.T) {
	today := time.Now().UTC().Format("2006-01-02")
	future := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	payload := map[string]any{"totalsByDay": []map[string]any{{"date": today, "duration": 3600}, {"date": future, "duration": 7200}}}
	appendWeeklyReportViews(payload, map[string]any{"timeZone": "UTC"}, false)
	if got := payload["weekly_day_totals"].([]WeeklyDayTotalView); len(got) != 1 || got[0].Date != today {
		t.Fatalf("future day should be omitted by default, got %#v", got)
	}
	payload = map[string]any{"totalsByDay": []map[string]any{{"date": today, "duration": 3600}, {"date": future, "duration": 7200}}}
	appendWeeklyReportViews(payload, map[string]any{"timeZone": "UTC"}, true)
	got := payload["weekly_day_totals"].([]WeeklyDayTotalView)
	if len(got) != 2 || !got[1].IsFuture {
		t.Fatalf("future day should be included and marked, got %#v", got)
	}
}

func TestUnbilledForClientFiltersDetailedReport(t *testing.T) {
	client, cleanup := newDetailedCompositeClient(t)
	defer cleanup()
	svc := New(client, "ws1")
	result, err := svc.UnbilledForClient(context.Background(), map[string]any{"client": "123456789012345678901234", "financial_start": "2026-05-01T00:00:00Z", "financial_end": "2026-06-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("unbilled for client: %v", err)
	}
	view := result.Data.(UnbilledForClientView)
	if view.ClientID == "" || len(view.Entries) != 1 || view.Financials.Earned == nil {
		t.Fatalf("unbilled view not normalized: %#v", view)
	}
}

func newDetailedCompositeClient(t *testing.T) (*clockify.Client, func()) {
	t.Helper()
	return newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/reports/detailed" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, detailedCompositePayload())
	})
}

func detailedCompositePayload() map[string]any {
	return map[string]any{
		"timeentries": []map[string]any{{
			"id":             "te1",
			"description":    "",
			"billable":       true,
			"duration":       3600,
			"earnedAmount":   10000,
			"costAmount":     4000,
			"profitAmount":   6000,
			"currency":       "USD",
			"invoicingState": "UNINVOICED",
			"project":        map[string]any{"id": "p1", "name": "Project"},
			"client":         map[string]any{"id": "c1", "name": "Client"},
		}},
		"totals": []map[string]any{{"earnedAmount": 10000, "costAmount": 4000, "currency": "USD"}},
	}
}
