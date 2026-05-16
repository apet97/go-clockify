package tools

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/testclockify"
)

// TestSwitchWorkReportsStatusAndDefaultedStart locks N-5 and N-11: a
// successful switch carries an explicit status:"ok" in its data, and an
// omitted start is echoed back via meta.startWasDefaulted / resolvedStart so
// the model never parses prose to learn the outcome.
func TestSwitchWorkReportsStatusAndDefaultedStart(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)
	svc.DefaultTimezone = time.UTC

	pkgOut, err := svc.ClockifyCreateWorkPackage(context.Background(), map[string]any{"project": "Switch Status Project"})
	pkg := mustToolResult(t, pkgOut, err)

	switchOut, err := svc.ClockifySwitchWork(context.Background(), map[string]any{
		"project_id": pkg.IDs["projectId"],
	})
	switched := mustToolResult(t, switchOut, err)

	data, ok := switched.Data.(map[string]any)
	if !ok {
		t.Fatalf("switch data type = %T, want map[string]any", switched.Data)
	}
	if data["status"] != "ok" {
		t.Fatalf("switch data.status = %v, want \"ok\"", data["status"])
	}
	if switched.Meta["startWasDefaulted"] != true {
		t.Fatalf("expected meta.startWasDefaulted=true when start is omitted: %+v", switched.Meta)
	}
	if resolved, _ := switched.Meta["resolvedStart"].(string); resolved == "" {
		t.Fatalf("expected meta.resolvedStart to carry the resolved timestamp: %+v", switched.Meta)
	}
}
