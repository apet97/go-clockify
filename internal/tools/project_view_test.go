package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestProjectViewMapsRatesMembersAndEstimates(t *testing.T) {
	project := clockify.Project{
		ID:         "p1",
		Name:       "Money Project",
		Billable:   true,
		Duration:   "3600000",
		HourlyRate: &clockify.Rate{Amount: 15000, Currency: "USD"},
		CostRate:   &clockify.Rate{Amount: 8000, Currency: "USD"},
		TimeEstimate: map[string]any{
			"estimate": "PT2H",
		},
		Memberships: []clockify.ProjectMembership{{
			UserID:           "u1",
			MembershipType:   "PROJECT",
			MembershipStatus: "ACTIVE",
			HourlyRate:       &clockify.Rate{Amount: 17500, Currency: "USD"},
			CostRate:         &clockify.Rate{Amount: 9000, Currency: "USD"},
		}},
	}
	view := projectViewFromProject(project)
	if view.Rates.ProjectHourlyMoney == nil || view.Rates.ProjectHourlyMoney.AmountCents != 15000 {
		t.Fatalf("project hourly money not mapped: %#v", view.Rates.ProjectHourlyMoney)
	}
	if len(view.Rates.Members) != 1 || view.Rates.Members[0].HourlyMoney.AmountCents != 17500 {
		t.Fatalf("member rates not mapped: %#v", view.Rates.Members)
	}
	if view.Estimates.DurationSeconds == nil || *view.Estimates.DurationSeconds != 3600 {
		t.Fatalf("duration seconds = %#v, want 3600", view.Estimates.DurationSeconds)
	}
	if view.Estimates.TimeEstimateSeconds == nil || *view.Estimates.TimeEstimateSeconds != 7200 {
		t.Fatalf("time estimate seconds = %#v, want 7200", view.Estimates.TimeEstimateSeconds)
	}
	if view.EstimateProgress.PercentUsed == nil || *view.EstimateProgress.PercentUsed != 50 {
		t.Fatalf("percent used = %#v, want 50", view.EstimateProgress.PercentUsed)
	}
}

func TestTaskViewMapsTaskRatesAndEstimate(t *testing.T) {
	task := clockify.Task{
		ID:             "t1",
		Name:           "Build",
		ProjectID:      "p1",
		Billable:       true,
		Duration:       "PT1H",
		Estimate:       "PT2H",
		BudgetEstimate: 25000,
		HourlyRate:     &clockify.Rate{Amount: 12000, Currency: "EUR"},
		CostRate:       &clockify.Rate{Amount: 7000, Currency: "EUR"},
	}
	view := taskViewFromTask(task, nil)
	if view.Rates.HourlyMoney == nil || view.Rates.HourlyMoney.AmountCents != 12000 {
		t.Fatalf("task hourly money not mapped: %#v", view.Rates.HourlyMoney)
	}
	if view.Estimates.DurationSeconds == nil || *view.Estimates.DurationSeconds != 3600 {
		t.Fatalf("task duration seconds = %#v, want 3600", view.Estimates.DurationSeconds)
	}
	if view.EstimateProgress.PercentUsed == nil || *view.EstimateProgress.PercentUsed != 50 {
		t.Fatalf("task percent used = %#v, want 50", view.EstimateProgress.PercentUsed)
	}
}

func TestListProjectsEnrichmentMapsSummaryReportMoney(t *testing.T) {
	var reportCalls int
	var reportBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			if got := r.URL.Query().Get("hydrated"); got != "true" {
				t.Fatalf("hydrated query = %q, want true", got)
			}
			respondJSON(t, w, []clockify.Project{{
				ID:         "p1",
				Name:       "Project",
				Billable:   true,
				Duration:   "3600000",
				HourlyRate: &clockify.Rate{Amount: 10000, Currency: "USD"},
				CostRate:   &clockify.Rate{Amount: 4000, Currency: "USD"},
			}})
		case r.URL.Path == "/workspaces/ws1/projects/p1/tasks" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Task{{
				ID:         "t1",
				Name:       "Task",
				ProjectID:  "p1",
				Billable:   true,
				Duration:   "PT1H",
				Estimate:   "PT2H",
				HourlyRate: &clockify.Rate{Amount: 9000, Currency: "USD"},
				CostRate:   &clockify.Rate{Amount: 3000, Currency: "USD"},
			}})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			reportCalls++
			if err := json.NewDecoder(r.Body).Decode(&reportBody); err != nil {
				t.Fatalf("decode report body: %v", err)
			}
			respondJSON(t, w, map[string]any{
				"groupOne": []map[string]any{{
					"_id":      "p1",
					"duration": 7200,
					"amounts": []map[string]any{
						{"type": "EARNED", "value": 20000, "currency": "USD"},
						{"type": "COST", "value": 8000, "currency": "USD"},
						{"type": "PROFIT", "value": 12000, "currency": "USD"},
					},
					"children": []map[string]any{{
						"_id":      "t1",
						"duration": 3600,
						"amounts": []map[string]any{
							{"type": "EARNED", "value": 9000, "currency": "USD"},
							{"type": "COST", "value": 3000, "currency": "USD"},
							{"type": "PROFIT", "value": 6000, "currency": "USD"},
						},
					}},
				}},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.ListProjects(context.Background(), map[string]any{
		"financial_start": "2023-05-13T00:00:00Z",
		"financial_end":   "2026-05-13T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	projects := result.Data.([]ProjectView)
	if reportCalls != 1 {
		t.Fatalf("reports calls = %d, want 1", reportCalls)
	}
	if reportBody["dateRangeStart"] != "2023-05-13T00:00:00Z" || reportBody["dateRangeEnd"] != "2026-05-13T00:00:00Z" {
		t.Fatalf("financial range not forwarded: %#v", reportBody)
	}
	filter := reportBody["summaryFilter"].(map[string]any)
	groups := filter["groups"].([]any)
	if len(groups) != 3 || groups[0] != "CLIENT" || groups[1] != "PROJECT" || groups[2] != "TASK" {
		t.Fatalf("summary groups = %#v", groups)
	}
	if projects[0].Financials.Source != entryFinancialSourceReportsAPI || projects[0].Financials.Earned.AmountCents != 20000 {
		t.Fatalf("project report financials not mapped: %#v", projects[0].Financials)
	}
	if len(projects[0].Tasks) != 1 || projects[0].Tasks[0].Financials.Earned.AmountCents != 9000 {
		t.Fatalf("task report financials not mapped: %#v", projects[0].Tasks)
	}
}

func TestProjectEnrichmentFallsBackToDerivedRates(t *testing.T) {
	project := clockify.Project{
		ID:         "p1",
		Name:       "Fallback",
		Billable:   true,
		Duration:   "3600000",
		HourlyRate: &clockify.Rate{Amount: 10000, Currency: "USD"},
		CostRate:   &clockify.Rate{Amount: 4000, Currency: "USD"},
	}
	view := projectViewFromProject(project)
	fin := derivedProjectFinancials(project, view.Estimates, financialRangeFromArgs(nil))
	if fin.Source != entryFinancialSourceDerivedRates {
		t.Fatalf("source = %s, want derived_rates", fin.Source)
	}
	if fin.Earned == nil || fin.Earned.AmountCents != 10000 {
		t.Fatalf("earned = %#v, want 10000 cents", fin.Earned)
	}
	if fin.Profit == nil || fin.Profit.AmountCents != 6000 {
		t.Fatalf("profit = %#v, want 6000 cents", fin.Profit)
	}
}

func TestProjectFinancialsOmitProfitForDifferentCurrencies(t *testing.T) {
	earned := MoneyFromCents(10000, "USD")
	cost := MoneyFromCents(7000, "EUR")
	fin := projectFinancialsFromReport(reportEntryMoney{earned: &earned, cost: &cost}, financialRangeFromArgs(nil))
	if fin.Profit != nil {
		t.Fatalf("profit should be omitted for mixed currencies: %#v", fin.Profit)
	}
	if fin.Reason == "" {
		t.Fatalf("expected mixed-currency reason")
	}
}

func TestProjectEnrichmentFailureDoesNotFailTool(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/projects" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Project{{ID: "p1", Name: "Project"}})
		case r.URL.Path == "/workspaces/ws1/projects/p1/tasks" && r.Method == http.MethodGet:
			respondJSON(t, w, []clockify.Task{})
		case r.URL.Path == "/workspaces/ws1/reports/summary" && r.Method == http.MethodPost:
			http.Error(w, "reports down", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EntryFinancialReports = true
	result, err := svc.ListProjects(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListProjects should not fail on enrichment failure: %v", err)
	}
	projects := result.Data.([]ProjectView)
	if projects[0].Financials.Source != entryFinancialSourceUnavailable {
		t.Fatalf("financial source = %s, want unavailable", projects[0].Financials.Source)
	}
	if result.Meta["financials"].(map[string]any)["reports_api_error"] == nil {
		t.Fatalf("expected reports_api_error meta, got %#v", result.Meta)
	}
}
