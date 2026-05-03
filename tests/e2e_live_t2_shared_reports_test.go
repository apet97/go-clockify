//go:build livee2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/policy"
)

func TestLiveT2SharedReportsCRUDAndExports(t *testing.T) {
	requireCategory(t, "CLOCKIFY_LIVE_FULL_SURFACE_ENABLED")

	h := setupLiveMCPHarness(t, liveMCPOptions{PolicyMode: policy.Full})
	c := setupLiveCampaign(t, h)
	c.activateTier2("shared_reports")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	now := time.Now().UTC()
	startAt := time.Date(now.Year(), now.Month()-1, now.Day(), 0, 0, 0, 0, time.UTC)
	endAt := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, int(999*time.Millisecond), time.UTC)
	start := startAt.Format("2006-01-02T15:04:05.000Z")
	end := endAt.Format("2006-01-02T15:04:05.000Z")
	summaryFilter := map[string]any{
		"exportType":     "JSON_V1",
		"dateRangeStart": start,
		"dateRangeEnd":   end,
		"sortOrder":      "DESCENDING",
		"summaryFilter": map[string]any{
			"groups":     []any{"PROJECT"},
			"sortColumn": "GROUP",
		},
	}

	created := h.callOK(ctx, "clockify_create_shared_report", map[string]any{
		"name":        c.LivePrefix("shared-summary", 0),
		"report_type": "SUMMARY",
		"filter":      summaryFilter,
	})
	reportID, _ := extractDataMap(t, created)["id"].(string)
	if reportID == "" {
		t.Fatalf("clockify_create_shared_report returned no id: %#v", created)
	}
	reportDeleted := false
	c.RegisterCleanup("shared-report", reportID, func(ctx context.Context) error {
		if reportDeleted {
			return nil
		}
		return h.Service.Client.DeleteReports(ctx, "/workspaces/"+c.WorkspaceID+"/shared-reports/"+reportID)
	})

	updated := h.callOK(ctx, "clockify_update_shared_report", map[string]any{
		"report_id": reportID,
		"name":      c.LivePrefix("shared-renamed", 0),
	})
	if updatedID, _ := extractDataMap(t, updated)["id"].(string); updatedID != reportID {
		t.Fatalf("clockify_update_shared_report id mismatch: got %q want %q", updatedID, reportID)
	}

	got := h.callOK(ctx, "clockify_get_shared_report", map[string]any{"report_id": reportID})
	if filters, ok := extractDataMap(t, got)["filters"].(map[string]any); !ok || filters["type"] != "SUMMARY" {
		t.Fatalf("clockify_get_shared_report did not return SUMMARY filters: %#v", got)
	}

	jsonExport := h.callOK(ctx, "clockify_export_shared_report", map[string]any{
		"report_id": reportID,
		"format":    "json",
	})
	if extractDataMap(t, jsonExport)["filters"] == nil {
		t.Fatalf("JSON export did not include filters: %#v", jsonExport)
	}

	pdfExport := h.callOK(ctx, "clockify_export_shared_report", map[string]any{
		"report_id": reportID,
		"format":    "pdf",
	})
	pdfData := extractDataMap(t, pdfExport)
	if ct, _ := pdfData["contentType"].(string); ct != "application/pdf" {
		t.Fatalf("PDF export content type mismatch: got %q data=%#v", ct, pdfData)
	}
	if bytes, _ := pdfData["bytes"].(float64); bytes <= 0 {
		t.Fatalf("PDF export returned no bytes: %#v", pdfData)
	}

	list := h.callOK(ctx, "clockify_list_shared_reports", map[string]any{"page": 1, "page_size": 100})
	seenTypes := map[string]string{"SUMMARY": reportID}
	for _, row := range extractList(t, list) {
		obj, ok := row.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := obj["type"].(string)
		id, _ := obj["id"].(string)
		if id != "" && (typ == "DETAILED" || typ == "WEEKLY") {
			if _, exists := seenTypes[typ]; !exists {
				seenTypes[typ] = id
			}
		}
	}
	for _, typ := range []string{"SUMMARY", "DETAILED", "WEEKLY"} {
		id := seenTypes[typ]
		if id == "" {
			t.Logf("no %s shared report available to export in sacrificial workspace", typ)
			continue
		}
		result := h.callOK(ctx, "clockify_export_shared_report", map[string]any{
			"report_id": id,
			"format":    "json",
		})
		data := extractDataMap(t, result)
		if data["filters"] == nil {
			t.Fatalf("%s shared report JSON export missing filters: %#v", typ, data)
		}
	}

	_ = h.callOK(ctx, "clockify_delete_shared_report", map[string]any{
		"report_id": reportID,
		"dry_run":   true,
	})
	_ = h.callOK(ctx, "clockify_delete_shared_report", map[string]any{"report_id": reportID})
	reportDeleted = true
}
