package tools

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestEntryViewEnrichmentMapsReportsAPIMoney(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			respondJSON(t, w, map[string]any{
				"timeEntries": []map[string]any{{
					"get_id":            "entry-1",
					"approvalRequestId": "approval-1",
					"locked":            true,
					"type":              "REGULAR",
					"userId":            "user-1",
					"userName":          "Firstname Lastname",
					"clientId":          "client-1",
					"clientName":        "Client A",
					"projectId":         "project-1",
					"projectName":       "Project A",
					"taskId":            "task-1",
					"taskName":          "Task A",
					"customFieldValues": []map[string]any{{"name": "Phase", "value": "Build"}},
					"tags":              []any{map[string]any{"id": "tag-1", "name": "Feature"}},
					"amounts": []map[string]any{
						{"type": "EARNED", "value": 12500, "currency": "USD"},
						{"type": "COST", "value": 5000, "currency": "USD"},
						{"type": "PROFIT", "value": 7500, "currency": "USD"},
					},
				}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	views, meta := svc.enrichEntryViews(context.Background(), "ws1", []clockify.TimeEntry{testMoneyEntry("entry-1", nil, nil, true)})

	if len(views) != 1 {
		t.Fatalf("views length = %d, want 1", len(views))
	}
	fin := views[0].Financials
	if fin.Source != entryFinancialSourceReportsAPI {
		t.Fatalf("source = %s, want reports_api", fin.Source)
	}
	assertMoneyCents(t, fin.Earned, 12500, "USD")
	assertMoneyCents(t, fin.Cost, 5000, "USD")
	assertMoneyCents(t, fin.Profit, 7500, "USD")
	if views[0].Approval == nil || views[0].Approval.RequestID != "approval-1" {
		t.Fatalf("approval not enriched from detailed report row: %#v", views[0].Approval)
	}
	if views[0].Audit == nil || views[0].Audit.Locked == nil || !*views[0].Audit.Locked {
		t.Fatalf("audit locked state not enriched: %#v", views[0].Audit)
	}
	if views[0].Entities == nil || views[0].Entities.Project == nil || views[0].Entities.Project.Name != "Project A" {
		t.Fatalf("entities not enriched: %#v", views[0].Entities)
	}
	if views[0].ProjectID != "project-1" || views[0].TaskID != "task-1" || len(views[0].TagIDs) != 1 || views[0].TagIDs[0] != "tag-1" {
		t.Fatalf("top-level entry fields not filled from report row: %#v", views[0])
	}
	if meta["reports_api_matched"] != 1 {
		t.Fatalf("unexpected financial meta: %#v", meta)
	}
}

func TestReportRowEntryIDNormalizesDetailedReportAliases(t *testing.T) {
	for _, row := range []map[string]any{
		{"_id": "entry-1"},
		{"id": "entry-1"},
		{"get_id": "entry-1"},
		{"entryId": "entry-1"},
		{"timeEntryId": "entry-1"},
		{"timeEntry": map[string]any{"get_id": "entry-1"}},
	} {
		if got := reportRowEntryID(row); got != "entry-1" {
			t.Fatalf("reportRowEntryID(%#v) = %q, want entry-1", row, got)
		}
	}
}

func TestEntryViewEnrichmentFallsBackToDerivedRates(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			respondJSON(t, w, map[string]any{"timeEntries": []map[string]any{}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	views, _ := svc.enrichEntryViews(context.Background(), "ws1", []clockify.TimeEntry{
		testMoneyEntry("entry-1", &clockify.Rate{Amount: 10000, Currency: "USD"}, &clockify.Rate{Amount: 4000, Currency: "USD"}, true),
	})

	fin := views[0].Financials
	if fin.Source != entryFinancialSourceDerivedRates {
		t.Fatalf("source = %s, want derived_rates", fin.Source)
	}
	assertMoneyCents(t, fin.Earned, 10000, "USD")
	assertMoneyCents(t, fin.Cost, 4000, "USD")
	assertMoneyCents(t, fin.Profit, 6000, "USD")
	if fin.Rate == nil || fin.Rate.HourlyScope != RateScopeEntry || fin.Rate.CostScope != RateScopeEntry {
		t.Fatalf("unexpected rate details: %#v", fin.Rate)
	}
}

func TestEntryViewEnrichmentOmitsProfitForDifferentCurrencies(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws1/reports/detailed" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		respondJSON(t, w, map[string]any{"timeEntries": []map[string]any{}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	views, _ := svc.enrichEntryViews(context.Background(), "ws1", []clockify.TimeEntry{
		testMoneyEntry("entry-1", &clockify.Rate{Amount: 10000, Currency: "USD"}, &clockify.Rate{Amount: 4000, Currency: "EUR"}, true),
	})

	fin := views[0].Financials
	if fin.Source != entryFinancialSourceDerivedRates {
		t.Fatalf("source = %s, want derived_rates", fin.Source)
	}
	assertMoneyCents(t, fin.Earned, 10000, "USD")
	assertMoneyCents(t, fin.Cost, 4000, "EUR")
	if fin.Profit != nil {
		t.Fatalf("profit = %#v, want omitted", fin.Profit)
	}
	if fin.Reason == "" {
		t.Fatalf("expected reason explaining omitted profit")
	}
}

func TestEntryViewEnrichmentFailureDoesNotFailEntry(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/reports/detailed" && r.Method == http.MethodPost:
			http.Error(w, "reports down", http.StatusBadGateway)
		case r.URL.Path == "/workspaces/ws1" && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Workspace{ID: "ws1"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	views, meta := svc.enrichEntryViews(context.Background(), "ws1", []clockify.TimeEntry{testMoneyEntry("entry-1", nil, nil, true)})

	if len(views) != 1 || views[0].ID != "entry-1" {
		t.Fatalf("entry not preserved after enrichment failure: %#v", views)
	}
	if views[0].Financials.Source != entryFinancialSourceUnavailable {
		t.Fatalf("source = %s, want unavailable", views[0].Financials.Source)
	}
	if meta["reports_api_error"] == nil {
		t.Fatalf("expected reports_api_error meta, got %#v", meta)
	}
}

func testMoneyEntry(id string, hourly, cost *clockify.Rate, billable bool) clockify.TimeEntry {
	start := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	return clockify.TimeEntry{
		ID:          id,
		Description: "Money entry",
		Billable:    billable,
		HourlyRate:  hourly,
		CostRate:    cost,
		TimeInterval: clockify.TimeInterval{
			Start: start.Format(time.RFC3339),
			End:   start.Add(time.Hour).Format(time.RFC3339),
		},
	}
}

func assertMoneyCents(t *testing.T, got *MoneyView, want int64, currency string) {
	t.Helper()
	if got == nil {
		t.Fatalf("money is nil, want %d %s", want, currency)
	}
	if got.AmountCents != want || got.Currency != currency {
		t.Fatalf("money = %+v, want cents=%d currency=%s", *got, want, currency)
	}
}
