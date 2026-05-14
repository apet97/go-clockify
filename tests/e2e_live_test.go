//go:build livee2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveOneUserWorkflowMCP(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	prefix := liveWorkflowPrefix()
	runID := strings.ToLower(prefix)
	date := time.Now().UTC().Format("2006-01-02")

	status := h.callOK(ctx, "clockify_status", nil)
	statusIDs := extractIDs(t, status)
	if statusIDs["workspaceId"] == "" || statusIDs["userId"] == "" {
		t.Fatalf("clockify_status missing workspace/user IDs: %#v", statusIDs)
	}

	guide := h.callOK(ctx, "clockify_tools_guide", nil)
	guideData := extractDataMap(t, guide)
	if _, ok := guideData["workflows"]; !ok {
		t.Fatalf("clockify_tools_guide missing workflows: %#v", guideData)
	}

	workPackage := h.callOK(ctx, "clockify_create_work_package", map[string]any{
		"client":  prefix + " workflow client",
		"project": prefix + " workflow project",
		"task":    prefix + " workflow task",
		"tag":     prefix + " workflow tag",
	})
	workPackageIDs := extractIDs(t, workPackage)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId"} {
		if workPackageIDs[key] == "" {
			t.Fatalf("clockify_create_work_package missing %s: %#v", key, workPackageIDs)
		}
	}

	seed := h.callOK(ctx, "clockify_demo_seed", map[string]any{
		"run_id": runID,
		"prefix": prefix,
		"date":   date,
	})
	seedIDs := extractIDs(t, seed)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		if seedIDs[key] == "" {
			t.Fatalf("clockify_demo_seed missing %s: %#v", key, seedIDs)
		}
	}

	logged := h.callOK(ctx, "clockify_log_work", map[string]any{
		"start":       date + " 10:00",
		"end":         date + " 10:20",
		"description": prefix + " workflow log",
		"project_id":  workPackageIDs["projectId"],
		"task_id":     workPackageIDs["taskId"],
		"tag_ids":     []any{workPackageIDs["tagId"]},
		"billable":    true,
	})
	loggedIDs := extractIDs(t, logged)
	if loggedIDs["entryId"] == "" {
		t.Fatalf("clockify_log_work missing entryId: %#v", loggedIDs)
	}

	started := h.callOK(ctx, "clockify_start_work", map[string]any{
		"start":       time.Now().UTC().Add(-4 * time.Minute).Format(time.RFC3339),
		"description": prefix + " workflow started",
		"project_id":  workPackageIDs["projectId"],
		"task_id":     workPackageIDs["taskId"],
		"tag_ids":     []any{workPackageIDs["tagId"]},
	})
	if startedIDs := extractIDs(t, started); startedIDs["entryId"] == "" {
		t.Fatalf("clockify_start_work missing entryId: %#v", startedIDs)
	}

	switched := h.callOK(ctx, "clockify_switch_work", map[string]any{
		"start":       time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
		"description": prefix + " workflow switched",
		"project_id":  workPackageIDs["projectId"],
		"task_id":     workPackageIDs["taskId"],
		"tag_ids":     []any{workPackageIDs["tagId"]},
	})
	if switchedIDs := extractIDs(t, switched); switchedIDs["entryId"] == "" {
		t.Fatalf("clockify_switch_work missing entryId: %#v", switchedIDs)
	}

	stopped := h.callOK(ctx, "clockify_stop_work", nil)
	if stoppedIDs := extractIDs(t, stopped); stoppedIDs["entryId"] == "" {
		t.Fatalf("clockify_stop_work missing entryId: %#v", stoppedIDs)
	}

	fixed := h.callOK(ctx, "clockify_fix_entry", map[string]any{
		"entry_id":    loggedIDs["entryId"],
		"description": prefix + " workflow log fixed",
	})
	fixedIDs := extractIDs(t, fixed)
	if fixedIDs["entryId"] != loggedIDs["entryId"] {
		t.Fatalf("clockify_fix_entry entryId=%q, want %q", fixedIDs["entryId"], loggedIDs["entryId"])
	}

	day := h.callOK(ctx, "clockify_review_day", map[string]any{
		"date":            date,
		"include_entries": true,
	})
	dayData := extractDataMap(t, day)
	if _, ok := dayData["totals"]; !ok {
		t.Fatalf("clockify_review_day missing totals: %#v", dayData)
	}

	week := h.callOK(ctx, "clockify_review_week", map[string]any{
		"week_start":      date,
		"include_entries": true,
	})
	weekData := extractDataMap(t, week)
	if _, ok := weekData["totals"]; !ok {
		t.Fatalf("clockify_review_week missing totals: %#v", weekData)
	}

	noopCleanup := h.callOK(ctx, "clockify_demo_cleanup", map[string]any{
		"run_id": "missing-" + runID,
		"prefix": "missing-" + prefix,
		"start":  date + " 00:00",
		"end":    date + " 23:59",
	})
	cleanupData := extractDataMap(t, noopCleanup)
	if _, ok := cleanupData["deletedCount"]; !ok {
		t.Fatalf("clockify_demo_cleanup missing deletedCount: %#v", cleanupData)
	}
}

func liveWorkflowPrefix() string {
	if prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX")); prefix != "" {
		return prefix
	}
	return fmt.Sprintf("MCP-LIVE-%d", time.Now().UTC().Unix())
}

func extractIDs(t *testing.T, result map[string]any) map[string]string {
	t.Helper()
	envelope := structuredContentMap(t, result)
	raw, ok := envelope["ids"].(map[string]any)
	if !ok {
		t.Fatalf("result missing ids: %#v", envelope)
	}
	ids := make(map[string]string, len(raw))
	for k, v := range raw {
		if s, ok := v.(string); ok {
			ids[k] = s
		}
	}
	return ids
}
