package tools

import (
	"context"
	"fmt"

	"goclmcp/internal/dryrun"
	"goclmcp/internal/mcp"
	"goclmcp/internal/resolve"
)

func init() {
	registerTier2Group(Tier2Group{
		Name:        "expenses",
		Description: "Expense tracking — log, categorize, and report expenses",
		Keywords:    []string{"expense", "cost", "receipt", "category", "reimbursement"},
		Builder:     expenseHandlers,
	})
}

func expenseHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List expenses
		{Tool: toolRO("clockify_list_expenses", "List expenses in the workspace with pagination", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listExpenses(ctx, args)
		}},

		// 2. Get expense
		{Tool: toolRO("clockify_get_expense", "Get a single expense by ID", map[string]any{
			"type":       "object",
			"required":   []string{"expense_id"},
			"properties": map[string]any{"expense_id": map[string]any{"type": "string"}},
		}), ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getExpense(ctx, args)
		}},

		// 3. Create expense
		{Tool: toolRW("clockify_create_expense", "Create a new expense", map[string]any{
			"type":     "object",
			"required": []string{"amount", "date"},
			"properties": map[string]any{
				"amount":      map[string]any{"type": "number", "description": "Expense amount"},
				"date":        map[string]any{"type": "string", "description": "Expense date (YYYY-MM-DD)"},
				"category_id": map[string]any{"type": "string", "description": "Expense category ID"},
				"project_id":  map[string]any{"type": "string", "description": "Project ID"},
				"description": map[string]any{"type": "string", "description": "Expense description"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createExpense(ctx, args)
		}},

		// 4. Update expense
		{Tool: toolRW("clockify_update_expense", "Update an existing expense", map[string]any{
			"type":     "object",
			"required": []string{"expense_id"},
			"properties": map[string]any{
				"expense_id":  map[string]any{"type": "string"},
				"amount":      map[string]any{"type": "number"},
				"date":        map[string]any{"type": "string"},
				"category_id": map[string]any{"type": "string"},
				"project_id":  map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateExpense(ctx, args)
		}},

		// 5. Delete expense
		{Tool: toolDestructive("clockify_delete_expense", "Delete an expense by ID", map[string]any{
			"type":     "object",
			"required": []string{"expense_id"},
			"properties": map[string]any{
				"expense_id": map[string]any{"type": "string"},
				"dry_run":    map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteExpense(ctx, args)
		}},

		// 6. List expense categories
		{Tool: toolRO("clockify_list_expense_categories", "List expense categories in the workspace", map[string]any{
			"type": "object",
		}), ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listExpenseCategories(ctx, args)
		}},

		// 7. Create expense category
		{Tool: toolRW("clockify_create_expense_category", "Create a new expense category", map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Category name"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createExpenseCategory(ctx, args)
		}},

		// 8. Update expense category
		{Tool: toolRW("clockify_update_expense_category", "Update an expense category", map[string]any{
			"type":     "object",
			"required": []string{"category_id"},
			"properties": map[string]any{
				"category_id": map[string]any{"type": "string"},
				"name":        map[string]any{"type": "string"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateExpenseCategory(ctx, args)
		}},

		// 9. Delete expense category
		{Tool: toolDestructive("clockify_delete_expense_category", "Delete an expense category", map[string]any{
			"type":     "object",
			"required": []string{"category_id"},
			"properties": map[string]any{
				"category_id": map[string]any{"type": "string"},
				"dry_run":     map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteExpenseCategory(ctx, args)
		}},

		// 10. Expense report
		{Tool: toolRO("clockify_expense_report", "Get expenses filtered by date range with aggregation by category", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start":     map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
				"end":       map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
		}), ReadOnlyHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.expenseReport(ctx, args)
		}},
	}
}

// ---------------------------------------------------------------------------
// Expense handlers
// ---------------------------------------------------------------------------

func (s *Service) listExpenses(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	var items []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/expenses", map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}, &items); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_expenses", items, map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
		"page":        page,
	}), nil
}

func (s *Service) getExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var expense map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/expenses/"+expenseID, nil, &expense); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_expense", expense, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if v, ok := args["amount"]; ok {
		body["amount"] = v
	} else {
		return ResultEnvelope{}, fmt.Errorf("amount is required")
	}
	if v := stringArg(args, "date"); v != "" {
		body["date"] = v
	} else {
		return ResultEnvelope{}, fmt.Errorf("date is required")
	}
	if v := stringArg(args, "category_id"); v != "" {
		body["categoryId"] = v
	}
	if v := stringArg(args, "project_id"); v != "" {
		body["projectId"] = v
	}
	if v := stringArg(args, "description"); v != "" {
		body["description"] = v
	}

	var created map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/expenses", body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_expense", created, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if v, ok := args["amount"]; ok {
		body["amount"] = v
	}
	if v := stringArg(args, "date"); v != "" {
		body["date"] = v
	}
	if v := stringArg(args, "category_id"); v != "" {
		body["categoryId"] = v
	}
	if v := stringArg(args, "project_id"); v != "" {
		body["projectId"] = v
	}
	if v := stringArg(args, "description"); v != "" {
		body["description"] = v
	}

	var updated map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/expenses/"+expenseID, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_expense", updated, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) deleteExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		var expense map[string]any
		if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/expenses/"+expenseID, nil, &expense); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_expense",
			Data:   dryrun.WrapResult(expense, "clockify_delete_expense"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/expenses/"+expenseID); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_expense", map[string]any{
		"deleted":   true,
		"expenseId": expenseID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) listExpenseCategories(ctx context.Context, _ map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var items []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/expenses/categories", nil, &items); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_expense_categories", items, map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
	}), nil
}

func (s *Service) createExpenseCategory(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ResultEnvelope{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{"name": name}
	var created map[string]any
	if err := s.Client.Post(ctx, "/workspaces/"+wsID+"/expenses/categories", body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_expense_category", created, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) updateExpenseCategory(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	body := map[string]any{}
	if v := stringArg(args, "name"); v != "" {
		body["name"] = v
	}

	var updated map[string]any
	if err := s.Client.Put(ctx, "/workspaces/"+wsID+"/expenses/categories/"+catID, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_expense_category", updated, map[string]any{
		"workspaceId": wsID,
		"categoryId":  catID,
	}), nil
}

func (s *Service) deleteExpenseCategory(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_expense_category",
			Data: dryrun.MinimalResult("clockify_delete_expense_category", map[string]any{
				"category_id": catID,
			}),
			Meta: map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, "/workspaces/"+wsID+"/expenses/categories/"+catID); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_delete_expense_category", map[string]any{
		"deleted":    true,
		"categoryId": catID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) expenseReport(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	query := map[string]string{
		"page":      fmt.Sprintf("%d", page),
		"page-size": fmt.Sprintf("%d", pageSize),
	}
	if v := stringArg(args, "start"); v != "" {
		query["start"] = v
	}
	if v := stringArg(args, "end"); v != "" {
		query["end"] = v
	}

	var expenses []map[string]any
	if err := s.Client.Get(ctx, "/workspaces/"+wsID+"/expenses", query, &expenses); err != nil {
		return ResultEnvelope{}, err
	}

	// Aggregate by category.
	var totalAmount float64
	byCategory := map[string]float64{}
	for _, exp := range expenses {
		if amt, ok := exp["amount"].(float64); ok {
			totalAmount += amt
			catID, _ := exp["categoryId"].(string)
			if catID == "" {
				catID = "uncategorized"
			}
			byCategory[catID] += amt
		}
	}

	return ok("clockify_expense_report", map[string]any{
		"expenses":    expenses,
		"totalAmount": totalAmount,
		"byCategory":  byCategory,
	}, map[string]any{
		"workspaceId": wsID,
		"count":       len(expenses),
		"page":        page,
	}), nil
}
