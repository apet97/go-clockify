package tools

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestOneUserLiveWorkflow(t *testing.T) {
	if os.Getenv("CLOCKIFY_LIVE_TESTS") != "1" {
		t.Skip("set CLOCKIFY_LIVE_TESTS=1 with CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX to run live Clockify workflow tests")
	}
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	workspaceID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	if apiKey == "" || workspaceID == "" || prefix == "" {
		t.Fatal("CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, and CLOCKIFY_LIVE_PREFIX are required when CLOCKIFY_LIVE_TESTS=1")
	}

	client := clockify.NewClient(apiKey, defaultLiveBaseURL(), 30*time.Second, 2)
	defer client.Close()
	svc := New(client, workspaceID)
	svc.DefaultTimezone = time.UTC
	ctx := context.Background()
	runID := cleanDemoRunID(prefix)
	if runID == "" {
		runID = "live"
	}
	date := "2026-01-02"

	statusOut, err := svc.ClockifyStatus(ctx, nil)
	status := mustToolResult(t, statusOut, err)
	requireID(t, status, "workspaceId")
	requireID(t, status, "userId")

	seedOut, err := svc.ClockifyDemoSeed(ctx, map[string]any{"run_id": runID, "prefix": prefix, "date": date})
	seed := mustToolResult(t, seedOut, err)
	for _, key := range []string{"clientId", "projectId", "taskId", "tagId", "entryId"} {
		requireID(t, seed, key)
	}

	logOut, err := svc.ClockifyLogWork(ctx, map[string]any{
		"start":       date + " 10:30",
		"end":         date + " 11:00",
		"description": prefix + " Live logged work",
		"project_id":  seed.IDs["projectId"],
		"task_id":     seed.IDs["taskId"],
		"tag_ids":     []any{seed.IDs["tagId"]},
		"billable":    true,
	})
	logged := mustToolResult(t, logOut, err)
	requireID(t, logged, "entryId")

	dayOut, err := svc.ClockifyReviewDay(ctx, map[string]any{"date": date, "include_entries": true})
	day := mustToolResult(t, dayOut, err)
	if day.Data == nil {
		t.Fatalf("live day review missing data: %+v", day)
	}

	weekOut, err := svc.ClockifyReviewWeek(ctx, map[string]any{"week_start": "2025-12-29", "include_entries": true})
	week := mustToolResult(t, weekOut, err)
	if week.Data == nil {
		t.Fatalf("live week review missing data: %+v", week)
	}

	cleanupOut, err := svc.ClockifyDemoCleanup(ctx, map[string]any{"run_id": runID, "prefix": prefix, "start": "2026-01-01 00:00", "end": "2026-01-03 00:00"})
	cleanup := mustToolResult(t, cleanupOut, err)
	requireID(t, cleanup, "workspaceId")
}

func defaultLiveBaseURL() string {
	if baseURL := strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL")); baseURL != "" {
		return baseURL
	}
	return "https://api.clockify.me/api/v1"
}
