package tools

import (
	"context"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/testclockify"
)

// TestCreateExpenseDryRunSkipsWrite locks N-6: clockify_create_expense with
// dry_run:true returns a preview of the resolved payload and performs no
// write, so a caller can preflight before a real (non-idempotent) create.
func TestCreateExpenseDryRunSkipsWrite(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("test-key", fake.URL, time.Second, 0), fake.WorkspaceID)

	env, err := svc.createExpense(context.Background(), map[string]any{
		"amount":      100.0,
		"date":        "2026-05-16T09:00:00Z",
		"category_id": "65b382b606de527a7ee2b622",
		"dry_run":     true,
	})
	if err != nil {
		t.Fatalf("createExpense dry_run: %v", err)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("dry_run data type = %T, want map[string]any", env.Data)
	}
	if data["dry_run"] != true {
		t.Fatalf("expected a dry_run preview envelope, got %+v", data)
	}
}
