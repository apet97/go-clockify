//go:build legacy_platform

package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/policy"
	"github.com/apet97/go-clockify/internal/testharness"
)

const (
	largeWorkspaceAPIKey = "synthetic-owner-key"
	largeWorkspaceID     = "ws-large"
	largeWorkspaceUserID = "owner-1"
)

func TestLargeWorkspaceReadiness_SingleOwnerSynthetic(t *testing.T) {
	entries := largeWorkspaceEntries(11999)
	upstream := newLargeWorkspaceFakeClockify(t, entries)
	harness := testharness.NewBenchHarness(t, largeWorkspaceHarnessOpts(upstream))
	ctx := authn.WithPrincipal(context.Background(), &authn.Principal{
		Subject:  largeWorkspaceUserID,
		TenantID: largeWorkspaceID,
		AuthMode: authn.ModeStaticBearer,
	})

	summary := harness.Call(ctx, "clockify_quick_report", map[string]any{
		"days":            30,
		"include_entries": false,
	})
	if summary.Outcome != testharness.OutcomeSuccess {
		t.Fatalf("quick_report outcome=%s error=%q raw=%s", summary.Outcome, summary.ErrorMessage, string(summary.Raw))
	}
	var summaryEnv largeSummaryEnvelope
	decodeToolResult(t, summary.ResultText, &summaryEnv)
	if !summaryEnv.OK || summaryEnv.Action != "clockify_quick_report" {
		t.Fatalf("unexpected summary envelope: %+v", summaryEnv)
	}
	if summaryEnv.Data.Totals.Entries != len(entries) {
		t.Fatalf("entries total: want %d, got %d", len(entries), summaryEnv.Data.Totals.Entries)
	}
	wantSeconds := int64(len(entries) * 60)
	if summaryEnv.Data.Totals.TotalSeconds != wantSeconds {
		t.Fatalf("total seconds: want %d, got %d", wantSeconds, summaryEnv.Data.Totals.TotalSeconds)
	}
	if len(summaryEnv.Data.Entries) != 0 {
		t.Fatalf("include_entries=false should not return raw entries, got %d", len(summaryEnv.Data.Entries))
	}
	if summaryEnv.Meta.Pagination.PageSize != 200 ||
		summaryEnv.Meta.Pagination.PagesFetched != 60 ||
		summaryEnv.Meta.Pagination.EntriesTotal != len(entries) {
		t.Fatalf("unexpected pagination meta: %+v", summaryEnv.Meta.Pagination)
	}

	detailed := harness.Call(ctx, "clockify_quick_report", map[string]any{
		"days":            30,
		"include_entries": true,
		"max_entries":     100,
	})
	if detailed.Outcome != testharness.OutcomeToolError {
		t.Fatalf("quick_report outcome=%s error=%q raw=%s", detailed.Outcome, detailed.ErrorMessage, string(detailed.Raw))
	}
	if detailed.ErrorMessage == "" || !strings.Contains(detailed.ErrorMessage, "entry cap of 100 exceeded") {
		t.Fatalf("expected entry cap error, got %q", detailed.ErrorMessage)
	}
}

func BenchmarkLargeWorkspaceReadiness_SummaryReport(b *testing.B) {
	entries := largeWorkspaceEntries(11999)
	upstream := newLargeWorkspaceFakeClockify(b, entries)
	harness := testharness.NewBenchHarness(b, largeWorkspaceHarnessOpts(upstream))
	ctx := authn.WithPrincipal(context.Background(), &authn.Principal{
		Subject:  largeWorkspaceUserID,
		TenantID: largeWorkspaceID,
		AuthMode: authn.ModeStaticBearer,
	})

	b.ReportAllocs()
	for b.Loop() {
		result := harness.Call(ctx, "clockify_quick_report", map[string]any{
			"days":            30,
			"include_entries": false,
		})
		if result.Outcome != testharness.OutcomeSuccess {
			b.Fatalf("quick_report outcome=%s error=%q", result.Outcome, result.ErrorMessage)
		}
	}
}

func BenchmarkLargeWorkspaceReadiness_ListEntriesProjectFilter(b *testing.B) {
	entries := largeWorkspaceEntries(1199)
	upstream := newLargeWorkspaceProjectFilterFakeClockify(b, entries)
	harness := testharness.NewBenchHarness(b, largeWorkspaceHarnessOpts(upstream))
	ctx := authn.WithPrincipal(context.Background(), &authn.Principal{
		Subject:  largeWorkspaceUserID,
		TenantID: largeWorkspaceID,
		AuthMode: authn.ModeStaticBearer,
	})

	b.ReportAllocs()
	for b.Loop() {
		result := harness.Call(ctx, "clockify_list_entries", map[string]any{
			"start":     "2026-04-01T00:00:00Z",
			"end":       "2026-05-01T00:00:00Z",
			"project":   "Alpha",
			"page":      1,
			"page_size": 100,
		})
		if result.Outcome != testharness.OutcomeSuccess {
			b.Fatalf("list_entries outcome=%s error=%q", result.Outcome, result.ErrorMessage)
		}
	}
}

func BenchmarkLargeWorkspaceReadiness_FindAndUpdateEntryPageFive(b *testing.B) {
	upstream := newLargeWorkspaceFindAndUpdateFakeClockify(b)
	opts := largeWorkspaceHarnessOpts(upstream)
	opts.PolicyMode = policy.Standard
	harness := testharness.NewBenchHarness(b, opts)
	ctx := authn.WithPrincipal(context.Background(), &authn.Principal{
		Subject:  largeWorkspaceUserID,
		TenantID: largeWorkspaceID,
		AuthMode: authn.ModeStaticBearer,
	})

	b.ReportAllocs()
	for b.Loop() {
		result := harness.Call(ctx, "clockify_find_and_update_entry", map[string]any{
			"description_contains": "needle",
			"new_description":      "updated needle",
		})
		if result.Outcome != testharness.OutcomeSuccess {
			b.Fatalf("find_and_update_entry outcome=%s error=%q", result.Outcome, result.ErrorMessage)
		}
	}
}

func largeWorkspaceHarnessOpts(upstream *testharness.FakeClockify) testharness.InvokeOpts {
	return testharness.InvokeOpts{
		PolicyMode:     policy.TimeTrackingSafe,
		Principal:      &authn.Principal{Subject: largeWorkspaceUserID, TenantID: largeWorkspaceID, AuthMode: authn.ModeStaticBearer},
		Upstream:       upstream,
		ClockifyAPIKey: largeWorkspaceAPIKey,
		WorkspaceID:    largeWorkspaceID,
	}
}

func newLargeWorkspaceFakeClockify(tb testing.TB, entries []clockify.TimeEntry) *testharness.FakeClockify {
	tb.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		respondJSONLarge(tb, w, clockify.User{ID: largeWorkspaceUserID, Name: "Synthetic Owner"})
	})
	mux.HandleFunc("/workspaces/"+largeWorkspaceID+"/user/"+largeWorkspaceUserID+"/time-entries", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		if got := r.URL.Query().Get("page-size"); got != "200" {
			tb.Fatalf("expected report page-size=200, got %q", got)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		start := (page - 1) * 200
		if start >= len(entries) {
			respondJSONLarge(tb, w, []clockify.TimeEntry{})
			return
		}
		end := start + 200
		if end > len(entries) {
			end = len(entries)
		}
		respondJSONLarge(tb, w, entries[start:end])
	})
	return testharness.NewFakeClockify(tb, mux)
}

func newLargeWorkspaceProjectFilterFakeClockify(tb testing.TB, entries []clockify.TimeEntry) *testharness.FakeClockify {
	tb.Helper()
	alphaEntries := filterLargeWorkspaceEntriesByProject(entries, "p-alpha")
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		respondJSONLarge(tb, w, clockify.User{ID: largeWorkspaceUserID, Name: "Synthetic Owner"})
	})
	mux.HandleFunc("/workspaces/"+largeWorkspaceID+"/projects", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		if got := r.URL.Query().Get("name"); got != "Alpha" {
			tb.Fatalf("project resolver name=%q, want Alpha", got)
		}
		if got := r.URL.Query().Get("strict-name-search"); got != "true" {
			tb.Fatalf("strict-name-search=%q, want true", got)
		}
		respondJSONLarge(tb, w, []map[string]any{{"id": "p-alpha", "name": "Alpha"}})
	})
	mux.HandleFunc("/workspaces/"+largeWorkspaceID+"/user/"+largeWorkspaceUserID+"/time-entries", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		if got := r.URL.Query().Get("project"); got != "p-alpha" {
			tb.Fatalf("expected project pushdown p-alpha, got %q", got)
		}
		if got := r.URL.Query().Get("page-size"); got != "200" {
			tb.Fatalf("expected project-filter upstream page-size=200, got %q", got)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		start := (page - 1) * 200
		if start >= len(alphaEntries) {
			respondJSONLarge(tb, w, []clockify.TimeEntry{})
			return
		}
		end := start + 200
		if end > len(alphaEntries) {
			end = len(alphaEntries)
		}
		respondJSONLarge(tb, w, alphaEntries[start:end])
	})
	return testharness.NewFakeClockify(tb, mux)
}

func newLargeWorkspaceFindAndUpdateFakeClockify(tb testing.TB) *testharness.FakeClockify {
	tb.Helper()
	target := clockify.TimeEntry{
		ID:          "entry-target",
		Description: "needle",
		ProjectID:   "p-alpha",
		ProjectName: "Alpha",
		TimeInterval: clockify.TimeInterval{
			Start: "2026-04-05T09:00:00Z",
			End:   "2026-04-05T10:00:00Z",
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		respondJSONLarge(tb, w, clockify.User{ID: largeWorkspaceUserID, Name: "Synthetic Owner"})
	})
	mux.HandleFunc("/workspaces/"+largeWorkspaceID+"/user/"+largeWorkspaceUserID+"/time-entries", func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		if got := r.URL.Query().Get("page-size"); got != "200" {
			tb.Fatalf("expected find/update page-size=200, got %q", got)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page < 5 {
			respondJSONLarge(tb, w, largeWorkspaceNoiseEntries(page, 200))
			return
		}
		if page == 5 {
			respondJSONLarge(tb, w, []clockify.TimeEntry{target})
			return
		}
		respondJSONLarge(tb, w, []clockify.TimeEntry{})
	})
	mux.HandleFunc("/workspaces/"+largeWorkspaceID+"/time-entries/"+target.ID, func(w http.ResponseWriter, r *http.Request) {
		requireLargeWorkspaceAPIKey(tb, r)
		if r.Method != http.MethodPut {
			tb.Fatalf("expected PUT target entry, got %s", r.Method)
		}
		updated := target
		updated.Description = "updated needle"
		respondJSONLarge(tb, w, updated)
	})
	return testharness.NewFakeClockify(tb, mux)
}

func requireLargeWorkspaceAPIKey(tb testing.TB, r *http.Request) {
	tb.Helper()
	if got := r.Header.Get("X-Api-Key"); got != largeWorkspaceAPIKey {
		tb.Fatalf("unexpected X-Api-Key: %q", got)
	}
}

func largeWorkspaceEntries(n int) []clockify.TimeEntry {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	out := make([]clockify.TimeEntry, n)
	for i := range out {
		start := base.Add(time.Duration(i) * time.Minute)
		projectID := "p-alpha"
		projectName := "Alpha"
		if i%2 == 1 {
			projectID = "p-beta"
			projectName = "Beta"
		}
		out[i] = clockify.TimeEntry{
			ID:          fmt.Sprintf("entry-%05d", i),
			ProjectID:   projectID,
			ProjectName: projectName,
			TimeInterval: clockify.TimeInterval{
				Start: start.Format(time.RFC3339),
				End:   start.Add(time.Minute).Format(time.RFC3339),
			},
		}
	}
	return out
}

func filterLargeWorkspaceEntriesByProject(entries []clockify.TimeEntry, projectID string) []clockify.TimeEntry {
	out := make([]clockify.TimeEntry, 0, len(entries)/2)
	for _, entry := range entries {
		if entry.ProjectID == projectID {
			out = append(out, entry)
		}
	}
	return out
}

func largeWorkspaceNoiseEntries(page, n int) []clockify.TimeEntry {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(page) * time.Hour)
	out := make([]clockify.TimeEntry, n)
	for i := range out {
		start := base.Add(time.Duration(i) * time.Minute)
		out[i] = clockify.TimeEntry{
			ID:          fmt.Sprintf("noise-%02d-%03d", page, i),
			Description: "not the target",
			ProjectID:   "p-alpha",
			ProjectName: "Alpha",
			TimeInterval: clockify.TimeInterval{
				Start: start.Format(time.RFC3339),
				End:   start.Add(time.Minute).Format(time.RFC3339),
			},
		}
	}
	return out
}

func decodeToolResult(tb testing.TB, text string, out any) {
	tb.Helper()
	if err := json.Unmarshal([]byte(text), out); err != nil {
		tb.Fatalf("decode tool result: %v\n%s", err, text)
	}
}

func respondJSONLarge(tb testing.TB, w http.ResponseWriter, v any) {
	tb.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		tb.Fatalf("encode json: %v", err)
	}
}

type largeSummaryEnvelope struct {
	OK     bool   `json:"ok"`
	Action string `json:"action"`
	Data   struct {
		Totals struct {
			Entries      int   `json:"entries"`
			TotalSeconds int64 `json:"totalSeconds"`
		} `json:"totals"`
		Entries []json.RawMessage `json:"entries,omitempty"`
	} `json:"data"`
	Meta struct {
		Pagination struct {
			PageSize     int `json:"page_size"`
			PagesFetched int `json:"pages_fetched"`
			EntriesTotal int `json:"entries_total"`
		} `json:"pagination"`
	} `json:"meta"`
}
