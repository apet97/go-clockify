package tools

import "testing"

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
