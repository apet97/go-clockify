package tools

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// TestIsReportsPath pins which raw paths route to the reports host. Only the
// /reports* and /shared-reports* subtrees under the pinned workspace qualify;
// everything else stays on the main host.
func TestIsReportsPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/workspaces/ws1/reports/summary", true},
		{"/workspaces/ws1/reports/detailed", true},
		{"/workspaces/ws1/shared-reports", true},
		{"/workspaces/ws1/shared-reports/sr-1", true},
		{"/workspaces/ws1/clients", false},
		{"/workspaces/ws1/projects/p1", false},
		{"/user", false},
		{"/workspaces/ws1", false},
		// "reports" must be the third segment, not a project name or deeper key.
		{"/workspaces/ws1/projects/reports", false},
	}
	for _, c := range cases {
		if got := isReportsPath(c.path); got != c.want {
			t.Errorf("isReportsPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// TestRawAPIReportWriteUsesReportsHostOrRejects is the authoritative M1 gate:
// documented report and shared-report write paths must reach upstream via the
// reports-host helpers with the correct method (the prior bug routed them
// through the main-host Post/Put/Delete, which 404s on production). PATCH to a
// reports path has no Clockify endpoint and must return a clean unsupported
// error without any upstream call.
func TestRawAPIReportWriteUsesReportsHostOrRejects(t *testing.T) {
	t.Run("reports_post", func(t *testing.T) {
		var gotMethod, gotPath string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			respondJSON(t, w, map[string]any{"totals": []any{}})
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		svc.RawWriteDocumentedOnly = true
		if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "POST",
			"path":   "/workspaces/{workspaceId}/reports/summary",
			"body":   map[string]any{"dateRangeStart": "2024-01-01T00:00:00Z"},
		}); err != nil {
			t.Fatalf("raw report POST rejected: %v", err)
		}
		if gotMethod != http.MethodPost || gotPath != "/workspaces/ws1/reports/summary" {
			t.Fatalf("report POST reached upstream as %s %s; want POST /workspaces/ws1/reports/summary", gotMethod, gotPath)
		}
	})

	t.Run("shared_reports_post", func(t *testing.T) {
		var gotMethod, gotPath string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			respondJSON(t, w, map[string]any{"id": "sr-1"})
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		svc.RawWriteDocumentedOnly = true
		if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "POST",
			"path":   "/workspaces/{workspaceId}/shared-reports",
			"body":   map[string]any{"name": "Q1"},
		}); err != nil {
			t.Fatalf("raw shared-report POST rejected: %v", err)
		}
		if gotMethod != http.MethodPost || gotPath != "/workspaces/ws1/shared-reports" {
			t.Fatalf("shared-report POST reached upstream as %s %s; want POST /workspaces/ws1/shared-reports", gotMethod, gotPath)
		}
	})

	t.Run("shared_reports_put", func(t *testing.T) {
		var gotMethod, gotPath string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			respondJSON(t, w, map[string]any{"id": "sr-1"})
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		svc.RawWriteDocumentedOnly = true
		if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "PUT",
			"path":   "/workspaces/{workspaceId}/shared-reports/sr-1",
			"body":   map[string]any{"name": "Q1 renamed"},
		}); err != nil {
			t.Fatalf("raw shared-report PUT rejected: %v", err)
		}
		// The prior bug routed PUT through the main host and, worse, the
		// PutReports helper issued a POST. Assert the verb is PUT.
		if gotMethod != http.MethodPut || gotPath != "/workspaces/ws1/shared-reports/sr-1" {
			t.Fatalf("shared-report PUT reached upstream as %s %s; want PUT /workspaces/ws1/shared-reports/sr-1", gotMethod, gotPath)
		}
	})

	t.Run("shared_reports_delete", func(t *testing.T) {
		var gotMethod, gotPath string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		svc.RawWriteDocumentedOnly = true
		if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "DELETE",
			"path":   "/workspaces/{workspaceId}/shared-reports/sr-1",
		}); err != nil {
			t.Fatalf("raw shared-report DELETE rejected: %v", err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/workspaces/ws1/shared-reports/sr-1" {
			t.Fatalf("shared-report DELETE reached upstream as %s %s; want DELETE /workspaces/ws1/shared-reports/sr-1", gotMethod, gotPath)
		}
	})

	t.Run("reports_patch_unsupported", func(t *testing.T) {
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("reports PATCH reached upstream: %s %s", r.Method, r.URL.Path)
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		// Documented-only would already reject (no documented reports PATCH),
		// so disable it to prove the reports-host dispatch itself rejects PATCH.
		svc.RawWriteDocumentedOnly = false
		_, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "PATCH",
			"path":   "/workspaces/{workspaceId}/reports/summary",
			"body":   map[string]any{"x": 1},
		})
		if err == nil {
			t.Fatal("reports PATCH succeeded, want unsupported rejection")
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "unsupported") || !strings.Contains(msg, "reports") {
			t.Fatalf("reports PATCH err = %v; want an unsupported-reports message", err)
		}
	})
}

// TestRawAPIReportGetUsesReportsHost pins that a raw GET to a reports path also
// routes to the reports host (sensitive-read gate notwithstanding, reports is
// not in the sensitive set). It must arrive with repeated query keys intact.
func TestRawAPIReportGetUsesReportsHost(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		respondJSON(t, w, map[string]any{"reports": []any{}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.EnableRawTools = true
	svc.EnableRawGet = true
	if _, err := svc.RawAPIGet(context.Background(), map[string]any{
		"path":  "/workspaces/{workspaceId}/shared-reports",
		"query": map[string]any{"type": []any{"summary", "detailed"}},
	}); err != nil {
		t.Fatalf("raw report GET rejected: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/workspaces/ws1/shared-reports" {
		t.Fatalf("report GET reached upstream as %s %s; want GET /workspaces/ws1/shared-reports", gotMethod, gotPath)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse upstream query %q: %v", gotQuery, err)
	}
	if got := values["type"]; len(got) != 2 || got[0] != "summary" || got[1] != "detailed" {
		t.Fatalf("report GET query type = %v; want [summary detailed]", got)
	}
}

// TestRawAPIQueryPreservesRepeatedValues is the authoritative M2 gate: array
// query values must encode as repeated keys (status=active&status=archived),
// not a comma-joined scalar, on both GET and DELETE which are the methods that
// carry a raw query.
func TestRawAPIQueryPreservesRepeatedValues(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		var gotQuery string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			respondJSON(t, w, map[string]any{"ok": true})
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawGet = true
		if _, err := svc.RawAPIGet(context.Background(), map[string]any{
			"path":  "/workspaces/{workspaceId}/projects",
			"query": map[string]any{"status": []any{"active", "archived"}},
		}); err != nil {
			t.Fatalf("raw GET rejected: %v", err)
		}
		values, err := url.ParseQuery(gotQuery)
		if err != nil {
			t.Fatalf("parse upstream query %q: %v", gotQuery, err)
		}
		if got := values["status"]; len(got) != 2 || got[0] != "active" || got[1] != "archived" {
			t.Fatalf("GET status = %v; want [active archived] as repeated keys; raw=%q", got, gotQuery)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var gotQuery string
		client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			w.WriteHeader(http.StatusNoContent)
		})
		defer cleanup()

		svc := New(client, "ws1")
		svc.EnableRawTools = true
		svc.EnableRawWrites = true
		svc.RawWriteDocumentedOnly = false
		if _, err := svc.RawAPIRequest(context.Background(), map[string]any{
			"method": "DELETE",
			"path":   "/workspaces/{workspaceId}/things/t1",
			"query":  map[string]any{"flag": []any{"a", "b"}},
		}); err != nil {
			t.Fatalf("raw DELETE rejected: %v", err)
		}
		values, err := url.ParseQuery(gotQuery)
		if err != nil {
			t.Fatalf("parse upstream query %q: %v", gotQuery, err)
		}
		if got := values["flag"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("DELETE flag = %v; want [a b] as repeated keys; raw=%q", got, gotQuery)
		}
	})
}

// TestRawQueryRepeatedKeys unit-tests the rawQuery conversion directly: arrays
// become repeated keys, scalars stay scalar, empty arrays drop the key, and
// insertion order is preserved within a key.
func TestRawQueryRepeatedKeys(t *testing.T) {
	values := rawQuery(map[string]any{
		"status": []any{"active", "archived"},
	})
	if got := values.Encode(); got != "status=active&status=archived" {
		t.Fatalf("rawQuery encode = %q; want status=active&status=archived", got)
	}
}

func TestRawQueryScalarAndArray(t *testing.T) {
	values := rawQuery(map[string]any{
		"page":     float64(2),
		"hydrated": true,
		"status":   []any{"active", "archived"},
	})
	if got := values.Get("page"); got != "2" {
		t.Fatalf("rawQuery page = %q; want 2", got)
	}
	if got := values.Get("hydrated"); got != "true" {
		t.Fatalf("rawQuery hydrated = %q; want true", got)
	}
	if got := values["status"]; len(got) != 2 || got[0] != "active" || got[1] != "archived" {
		t.Fatalf("rawQuery status = %v; want [active archived]", got)
	}
}

func TestRawQueryEmptyAndNil(t *testing.T) {
	if values := rawQuery(nil); len(values) != 0 {
		t.Fatalf("rawQuery(nil) = %v; want empty", values)
	}
	if values := rawQuery(map[string]any{}); len(values) != 0 {
		t.Fatalf("rawQuery(empty) = %v; want empty", values)
	}
	// An empty array contributes no values for its key.
	values := rawQuery(map[string]any{"status": []any{}, "page": float64(1)})
	if _, ok := values["status"]; ok {
		t.Fatalf("rawQuery empty-array kept status key: %v", values)
	}
	if got := values.Get("page"); got != "1" {
		t.Fatalf("rawQuery page = %q; want 1", got)
	}
}
