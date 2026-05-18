package tools

import (
	"fmt"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func TestUpdateDemoResourceEmitsListChanged(t *testing.T) {
	var count int
	svc := &Service{
		EmitResourceListChanged: func() { count++ },
	}

	svc.updateDemoResource("run-a", "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("fresh run count=%d want 1", count)
	}

	svc.updateDemoResource("run-a", "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("repeat run count=%d want 1", count)
	}

	svc.updateDemoResource(defaultDemoResourceRunID, "prefix", "seeded", ToolResult{})
	if count != 1 {
		t.Fatalf("default run count=%d want 1", count)
	}
}

func TestDemoResourcesListIsCappedAt50(t *testing.T) {
	svc := New(clockify.NewClient("k", "http://127.0.0.1:1", time.Second, 0), "ws1")
	for i := 0; i < 70; i++ {
		svc.updateDemoResource(fmt.Sprintf("run-%03d", i), "p", "ok", ToolResult{})
	}
	got := len(svc.demoResourcesList())
	if got > maxDemoResourcesListed {
		t.Fatalf("demoResourcesList returned %d resources, want <= %d", got, maxDemoResourcesListed)
	}
	if got != maxDemoResourcesListed {
		t.Fatalf("demoResourcesList returned %d resources, want exactly %d", got, maxDemoResourcesListed)
	}
}
