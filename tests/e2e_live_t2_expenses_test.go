//go:build livee2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestLiveOneUserExpensesCRUD covers what is currently exercise-able on the
// expenses domain tools through the MCP path against a real Clockify backend.
// list_expenses / list_expense_categories / expense_report shape
// mismatches were closed in Batch 1, and the createExpense multipart
// fix in Batch 3 unblocks the create-path subtest below. The category
// archive constraint is still pinned because Clockify exposes no
// archive-flag mutation route.
func TestLiveOneUserExpensesCRUD(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	categoryName := c.LivePrefix("exp-cat", 0)
	var categoryID string
	var expenseID string
	var expenseDeleted bool

	t.Run("create_expense_category", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := h.callOK(ctx, "clockify_expenses_categories_create", map[string]any{
			"name": categoryName,
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("clockify_expenses_categories_create returned no id: %#v", data)
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
		result := h.callOK(ctx, "clockify_expenses_categories_update", map[string]any{
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
		errMsg := h.callExpectError(ctx, "clockify_expenses_categories_delete", map[string]any{
			"category_id": categoryID,
		})
		if !strings.Contains(errMsg, "archived") {
			t.Fatalf("expected upstream archive-required error, got: %q", errMsg)
		}
	})

	t.Run("create_expense_returns_id", func(t *testing.T) {
		if categoryID == "" {
			t.Skip("create_expense_category did not produce an id")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// SUMMARY rev 3 #2: the upstream expenses POST is
		// multipart/form-data with required userId, amount, date
		// (RFC3339 yyyy-MM-ddThh:mm:ssZ), and categoryId. user_id
		// defaults to the calling user via /user when omitted. This
		// subtest pins the success path; cleanup runs through the
		// raw client below so a leaked expense doesn't survive the
		// test even if asserts fail.
		date := time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
		result := h.callOK(ctx, "clockify_expenses_create", map[string]any{
			"amount":      1.0,
			"date":        date,
			"category_id": categoryID,
			"notes":       c.LivePrefix("exp", 0),
		})
		data := extractDataMap(t, result)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatalf("clockify_expenses_create returned no id: %#v", data)
		}
		expenseID = id
		c.RegisterCleanup("expense", id, func(ctx context.Context) error {
			if expenseDeleted {
				return nil
			}
			return c.rawDeletePath(ctx, "/expenses/"+id)
		})
	})

	t.Run("get_update_delete_expense_cycle", func(t *testing.T) {
		if expenseID == "" || categoryID == "" {
			t.Skip("create_expense did not produce required ids")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		got := h.callOK(ctx, "clockify_expenses_get", map[string]any{
			"expense_id": expenseID,
		})
		if gotID, _ := extractDataMap(t, got)["id"].(string); gotID != expenseID {
			t.Fatalf("get_expense id mismatch: got %q want %q", gotID, expenseID)
		}

		updated := h.callOK(ctx, "clockify_expenses_update", map[string]any{
			"expense_id":    expenseID,
			"change_fields": []any{"NOTES", "AMOUNT"},
			"notes":         c.LivePrefix("exp-updated", 0),
			"amount":        2.0,
		})
		if updatedID, _ := extractDataMap(t, updated)["id"].(string); updatedID != expenseID {
			t.Fatalf("update_expense id mismatch: got %q want %q", updatedID, expenseID)
		}

		_ = h.callOK(ctx, "clockify_expenses_delete", map[string]any{
			"expense_id": expenseID,
		})
		expenseDeleted = true
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
		errMsg := h.callExpectError(ctx, "clockify_expenses_get", map[string]any{
			"expense_id": "000000000000000000000001",
		})
		if !strings.Contains(errMsg, "doesn't belong to Workspace") &&
			!strings.Contains(strings.ToLower(errMsg), "not found") {
			t.Fatalf("expected workspace-scope or not-found error for bogus expense id, got: %q", errMsg)
		}
	})
}
