//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveT2ExpensesCRUD covers what is currently exercise-able on the
// expenses group through the MCP path against a real Clockify backend.
// The Tier-2 read-only sweep (TestLiveTier2ReadOnlySweep) already
// flagged that list_expenses / list_expense_categories /
// expense_report all fail with shape mismatches — this file picks up
// the still-callable surface (create + update on categories) and
// pins the upstream archive-required constraint that blocks delete.
//
// What this test does NOT exercise (with rationale, so a future fix
// can flip these to real coverage):
//
//   - clockify_create_expense: handler POSTs application/json, but
//     /workspaces/{id}/expenses rejects with
//     `Content-Type 'application/json' is not supported`. The
//     upstream endpoint expects multipart/form-data (likely to
//     support receipt upload). Verified by direct curl probe:
//     `application/json` → 3000 error; multipart with named fields
//     → progresses to "Amount is required". Likely fix: thread a
//     multipart body builder through internal/clockify.Client and
//     have createExpense use it.
//   - clockify_get_expense: an end-to-end test would need a fresh
//     expense to read, which depends on the broken create path.
//     Will land alongside the create fix.
//   - clockify_update_expense / clockify_delete_expense: same
//     dependency.
func TestLiveT2ExpensesCRUD(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)
	c.activateTier2("expenses")

	categoryName := c.LivePrefix("exp-cat", 0)
	var categoryID string

	t.Run("create_expense_category", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_create_expense_category", map[string]any{
			"name": categoryName,
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("create_expense_category returned no id: %#v", data)
		}
		gotName, _ := data["name"].(string)
		if gotName != categoryName {
			t.Fatalf("category name mismatch: got %q, want %q", gotName, categoryName)
		}
		categoryID = id
		t.Logf("created expense category id=%s name=%s", id, gotName)

		// Categories cannot be deleted via API on this Clockify
		// version — the maintainer must archive them manually in the
		// UI. Register a best-effort cleanup that attempts the delete
		// and logs the expected refusal so the orphan is at least
		// audit-trailed in the cleanup log.
		c.RegisterCleanup("expense-category", id, func(ctx context.Context) error {
			err := c.rawDeletePath(ctx, "/expenses/categories/"+id)
			if err != nil && strings.Contains(err.Error(), "archived") {
				// Expected — Clockify requires archival before
				// deletion and the API does not expose a writable
				// archive flag. Treat as best-effort no-op.
				c.t.Logf("expense-category %s left orphaned: archival is UI-only on this Clockify version", id)
				return nil
			}
			return err
		})
	})

	t.Run("update_expense_category", func(t *testing.T) {
		if categoryID == "" {
			t.Skip("create_expense_category did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		updated := categoryName + "-renamed"
		result := h.callOK(ctx, "clockify_update_expense_category", map[string]any{
			"category_id": categoryID,
			"name":        updated,
		})
		data := extractDataMap(t, result)
		gotName, _ := data["name"].(string)
		if gotName != updated {
			t.Fatalf("rename did not stick: got %q, want %q", gotName, updated)
		}
	})

	t.Run("delete_expense_category_blocked_by_archive_constraint", func(t *testing.T) {
		if categoryID == "" {
			t.Skip("create_expense_category did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Upstream rejects deletion of unarchived categories. The
		// archive flag is not writable via PUT on the category
		// resource (verified by direct curl probe), there is no
		// /archive subroute, and POST/PATCH on the category resource
		// are not supported. So the MCP path's delete necessarily
		// surfaces the upstream's "must be archived" error. This
		// assertion pins that contract: a future Clockify version
		// that accepts the delete (or a handler that pre-archives)
		// will flip the assertion and force this annotation to be
		// reviewed.
		errMsg := h.callExpectError(ctx, "clockify_delete_expense_category", map[string]any{
			"category_id": categoryID,
		})
		if !strings.Contains(errMsg, "archived") {
			t.Fatalf("expected upstream archive-required error, got: %q", errMsg)
		}
	})

	t.Run("create_expense_blocked_by_handler_content_type_bug", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// The create-expense handler at
		// internal/tools/tier2_expenses.go:207 POSTs
		// application/json, but the upstream expenses endpoint only
		// accepts multipart/form-data. The handler returns the raw
		// upstream rejection, which we assert here so that any fix
		// (multipart body, different endpoint) flips the test and
		// forces this annotation to be reviewed.
		errMsg := h.callExpectError(ctx, "clockify_create_expense", map[string]any{
			"date":   time.Now().UTC().Format("2006-01-02"),
			"amount": 100.0,
		})
		if !strings.Contains(errMsg, "Content-Type 'application/json' is not supported") {
			t.Fatalf("expected upstream content-type rejection, got: %q", errMsg)
		}
	})

	t.Run("get_expense_rejects_nonexistent_id", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Bogus 24-char hex id — Clockify ids are MongoDB ObjectIds,
		// so a string of all zeros is well-formed but extremely
		// unlikely to match. Upstream surfaces a 400 with
		// "Expense doesn't belong to Workspace" rather than a 404 —
		// the assertion pins that current behaviour. Pure read-only
		// path; no cleanup needed.
		errMsg := h.callExpectError(ctx, "clockify_get_expense", map[string]any{
			"expense_id": "000000000000000000000001",
		})
		if !strings.Contains(errMsg, "doesn't belong to Workspace") &&
			!strings.Contains(strings.ToLower(errMsg), "not found") {
			t.Fatalf("expected workspace-scope or not-found error for bogus expense id, got: %q", errMsg)
		}
	})
}
