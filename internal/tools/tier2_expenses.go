package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
	"github.com/apet97/go-clockify/internal/paths"
	"github.com/apet97/go-clockify/internal/resolve"
)

func expenseHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		// 1. List expenses
		{Tool: withOutputSchema(toolRO("clockify_list_expenses", "List expenses in the workspace with pagination and optional date range", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
				"start":     map[string]any{"type": "string", "description": "Start date (YYYY-MM-DD or RFC3339)"},
				"end":       map[string]any{"type": "string", "description": "End date (YYYY-MM-DD or RFC3339)"},
			},
		}), envelopeSchemaFor[[]ExpenseView]("clockify_list_expenses")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listExpenses(ctx, args)
		}},

		// 2. Get expense
		{Tool: withOutputSchema(toolRO("clockify_get_expense", "Get a single expense by ID", map[string]any{
			"type":       "object",
			"required":   []string{"expense_id"},
			"properties": map[string]any{"expense_id": map[string]any{"type": "string"}},
		}), envelopeSchemaFor[ExpenseView]("clockify_get_expense")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.getExpense(ctx, args)
		}},

		// 3. Create expense
		{Tool: toolRW("clockify_create_expense", "Create a new expense (multipart form). amount is interpreted as major currency units by default, e.g. 125.00 for $125.00; pass amount_unit:\"minor\" when supplying cents.", map[string]any{
			"type":     "object",
			"required": []string{"amount", "date", "category_id"},
			"properties": map[string]any{
				"amount":      map[string]any{"type": "number", "description": "Expense amount. Defaults to major currency units, e.g. 125.00 for $125.00; use amount_unit:\"minor\" for cents/minor units."},
				"amount_unit": map[string]any{"type": "string", "enum": []string{"major", "minor"}, "description": "Unit for amount. major (default) sends the value as entered; minor divides by 100 before sending to Clockify."},
				"date":        map[string]any{"type": "string", "description": "Expense date (RFC3339 yyyy-MM-ddThh:mm:ssZ)"},
				"category_id": map[string]any{"type": "string", "description": "Expense category ID (required)"},
				"user_id":     map[string]any{"type": "string", "description": "User the expense is logged against; defaults to the calling user"},
				"project_id":  map[string]any{"type": "string", "description": "Project ID (optional)"},
				"task_id":     map[string]any{"type": "string", "description": "Task ID (optional)"},
				"notes":       map[string]any{"type": "string", "description": "Free-form notes"},
				"billable":    map[string]any{"type": "boolean", "description": "Whether the expense is billable"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createExpense(ctx, args)
		}},

		// 4. Update expense
		{Tool: toolRW("clockify_update_expense", "Update an existing expense (multipart form). change_fields enumerates which fields the upstream should apply; every listed token must include its matching argument.", map[string]any{
			"type":     "object",
			"required": []string{"expense_id", "change_fields"},
			"properties": map[string]any{
				"expense_id":    map[string]any{"type": "string"},
				"change_fields": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"USER", "DATE", "PROJECT", "TASK", "CATEGORY", "NOTES", "AMOUNT", "BILLABLE", "FILE"}}, "description": "Field tokens to update. Each listed token requires the matching argument: USER=user_id, DATE=date, PROJECT=project_id, TASK=task_id, CATEGORY=category_id, NOTES=notes, AMOUNT=amount, BILLABLE=billable. FILE is not supported."},
				"amount":        map[string]any{"type": "number", "description": "Expense amount. Defaults to major currency units; use amount_unit:\"minor\" for cents/minor units."},
				"amount_unit":   map[string]any{"type": "string", "enum": []string{"major", "minor"}, "description": "Unit for amount when AMOUNT is included in change_fields."},
				"date":          map[string]any{"type": "string", "description": "RFC3339 yyyy-MM-ddThh:mm:ssZ"},
				"category_id":   map[string]any{"type": "string"},
				"project_id":    map[string]any{"type": "string"},
				"task_id":       map[string]any{"type": "string"},
				"user_id":       map[string]any{"type": "string", "description": "Reassign the expense to this user"},
				"notes":         map[string]any{"type": "string"},
				"billable":      map[string]any{"type": "boolean"},
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
		{Tool: withOutputSchema(toolRO("clockify_list_expense_categories", "List expense categories in the workspace", map[string]any{
			"type": "object",
		}), envelopeOpenMapSlice("clockify_list_expense_categories")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.listExpenseCategories(ctx, args)
		}},

		// 7. Create expense category
		{Tool: toolRW("clockify_create_expense_category", "Create a new expense category, optionally including upstream unit-price fields.", map[string]any{
			"type":     "object",
			"required": []string{"name"},
			"properties": map[string]any{
				"name":           map[string]any{"type": "string", "description": "Category name"},
				"has_unit_price": map[string]any{"type": "boolean", "description": "Whether this category has a unit price."},
				"price_in_cents": map[string]any{"type": "integer", "minimum": 0, "description": "Unit price in minor currency units/cents."},
				"unit":           map[string]any{"type": "string", "description": "Unit label for unit-priced categories."},
				"dry_run":        map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.createExpenseCategory(ctx, args)
		}},

		// 8. Update expense category
		{Tool: toolRW("clockify_update_expense_category", "Update an expense category, including upstream unit-price fields.", map[string]any{
			"type":     "object",
			"required": []string{"category_id"},
			"properties": map[string]any{
				"category_id":    map[string]any{"type": "string"},
				"name":           map[string]any{"type": "string"},
				"has_unit_price": map[string]any{"type": "boolean", "description": "Whether this category has a unit price."},
				"price_in_cents": map[string]any{"type": "integer", "minimum": 0, "description": "Unit price in minor currency units/cents."},
				"unit":           map[string]any{"type": "string", "description": "Unit label for unit-priced categories."},
				"archived":       map[string]any{"type": "boolean", "description": "Archive state to apply via the category status endpoint."},
				"dry_run":        map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.updateExpenseCategory(ctx, args)
		}},

		// 9. Archive / unarchive expense category
		{Tool: toolRW("clockify_archive_expense_category", "Archive or unarchive an expense category via PATCH /expenses/categories/{categoryId}/status.", map[string]any{
			"type":     "object",
			"required": []string{"category_id"},
			"properties": map[string]any{
				"category_id": map[string]any{"type": "string"},
				"archived":    map[string]any{"type": "boolean", "description": "Archive flag. Defaults to true."},
				"dry_run":     map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.archiveExpenseCategory(ctx, args)
		}},

		// 10. Delete expense category
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

		// 11. Expense report
		{Tool: withOutputSchema(toolRO("clockify_expense_report", "Generate a Clockify expense detailed report via the Reports API", expenseReportInputSchema()), envelopeOpenMap("clockify_expense_report")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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

	path, err := paths.Workspace(wsID, "expenses")
	if err != nil {
		return ResultEnvelope{}, err
	}
	// Upstream wraps the list in a doubly-nested envelope:
	// {expenses: {expenses: [...], count: N}, dailyTotals: [...], weeklyTotals: [...]}.
	// Verified live 2026-05-02 via clockify-api-probe-lab.
	var envelope struct {
		Expenses struct {
			Expenses []map[string]any `json:"expenses"`
			Count    int              `json:"count"`
		} `json:"expenses"`
	}
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
	if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	items := envelope.Expenses.Expenses
	return ok("clockify_list_expenses", expenseViewsFromRaw(items), map[string]any{
		"workspaceId": wsID,
		"count":       envelope.Expenses.Count,
		"page":        page,
		"pageSize":    pageSize,
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

	path, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var expense map[string]any
	if err := s.Client.Get(ctx, path, nil, &expense); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_get_expense", expenseViewFromRaw(expense), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}

	// Required: amount, date (RFC3339), category_id. user_id defaults
	// to the calling user via /user — the upstream rejects multipart
	// POSTs that omit userId with a 400.
	amount, hasAmount := args["amount"].(float64)
	if !hasAmount {
		return ResultEnvelope{}, fmt.Errorf("amount is required")
	}
	date := stringArg(args, "date")
	if date == "" {
		return ResultEnvelope{}, fmt.Errorf("date is required")
	}
	categoryID := stringArg(args, "category_id")
	if categoryID == "" {
		return ResultEnvelope{}, fmt.Errorf("category_id is required")
	}
	userID := stringArg(args, "user_id")
	if userID == "" {
		current, err := s.getCurrentUser(ctx)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("resolve user_id from current user: %w", err)
		}
		userID = current.ID
	}

	form := url.Values{}
	form.Set("userId", userID)
	normalizedAmount, err := expenseAmountForClockify(args, amount)
	if err != nil {
		return ResultEnvelope{}, err
	}
	form.Set("amount", strconv.FormatFloat(normalizedAmount, 'f', -1, 64))
	form.Set("date", date)
	form.Set("categoryId", categoryID)
	if v := stringArg(args, "project_id"); v != "" {
		form.Set("projectId", v)
	}
	if v := stringArg(args, "task_id"); v != "" {
		form.Set("taskId", v)
	}
	if v := stringArg(args, "notes"); v != "" {
		form.Set("notes", v)
	}
	if v, ok := args["billable"].(bool); ok {
		form.Set("billable", strconv.FormatBool(v))
	}

	path, err := paths.Workspace(wsID, "expenses")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.PostMultipart(ctx, path, form, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_expense", expenseViewFromRaw(created), map[string]any{"workspaceId": wsID}), nil
}

// validUpdateExpenseChangeFields lists the upstream-accepted tokens for
// the multipart `changeFields` field on PUT /expenses/{id}. Anything
// outside this set is rejected by the upstream with code 3000.
var validUpdateExpenseChangeFields = map[string]bool{
	"USER":     true,
	"DATE":     true,
	"PROJECT":  true,
	"TASK":     true,
	"CATEGORY": true,
	"NOTES":    true,
	"AMOUNT":   true,
	"BILLABLE": true,
	"FILE":     true,
}

func parseUpdateExpenseChangeFields(args map[string]any) ([]string, error) {
	raw, hasChange := args["change_fields"]
	if !hasChange {
		return nil, fmt.Errorf("change_fields is required and must list at least one of USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE, FILE")
	}
	var values []any
	switch v := raw.(type) {
	case []any:
		values = v
	case []string:
		values = make([]any, 0, len(v))
		for _, item := range v {
			values = append(values, item)
		}
	default:
		return nil, fmt.Errorf("change_fields must be an array of strings; got %T", raw)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("change_fields is required and must list at least one of USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE, FILE")
	}

	changeFields := make([]string, 0, len(values))
	for _, raw := range values {
		token, isStr := raw.(string)
		if !isStr {
			return nil, fmt.Errorf("change_fields must be strings; got %T", raw)
		}
		token = strings.ToUpper(strings.TrimSpace(token))
		if !validUpdateExpenseChangeFields[token] {
			return nil, fmt.Errorf("change_fields contains unsupported token %q", token)
		}
		changeFields = append(changeFields, token)
	}
	return changeFields, nil
}

func validateUpdateExpenseChangedValues(changeFields []string, args map[string]any) error {
	for _, token := range changeFields {
		switch token {
		case "USER":
			if stringArg(args, "user_id") == "" {
				return fmt.Errorf("change_fields includes USER but user_id is required")
			}
		case "DATE":
			if stringArg(args, "date") == "" {
				return fmt.Errorf("change_fields includes DATE but date is required")
			}
		case "PROJECT":
			if stringArg(args, "project_id") == "" {
				return fmt.Errorf("change_fields includes PROJECT but project_id is required")
			}
		case "TASK":
			if stringArg(args, "task_id") == "" {
				return fmt.Errorf("change_fields includes TASK but task_id is required")
			}
		case "CATEGORY":
			if stringArg(args, "category_id") == "" {
				return fmt.Errorf("change_fields includes CATEGORY but category_id is required")
			}
		case "NOTES":
			if _, ok := args["notes"].(string); !ok {
				return fmt.Errorf("change_fields includes NOTES but notes is required")
			}
		case "AMOUNT":
			if _, ok := args["amount"].(float64); !ok {
				return fmt.Errorf("change_fields includes AMOUNT but amount is required")
			}
		case "BILLABLE":
			if _, ok := args["billable"].(bool); !ok {
				return fmt.Errorf("change_fields includes BILLABLE but billable is required")
			}
		case "FILE":
			return fmt.Errorf("change_fields includes FILE, but file updates are not supported")
		}
	}
	return nil
}

func expenseAmountForClockify(args map[string]any, amount float64) (float64, error) {
	switch strings.ToLower(strings.TrimSpace(stringArg(args, "amount_unit"))) {
	case "", "major":
		return amount, nil
	case "minor":
		return amount / 100, nil
	default:
		return 0, fmt.Errorf("amount_unit must be major or minor")
	}
}

func (s *Service) updateExpense(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ResultEnvelope{}, err
	}
	changeFields, err := parseUpdateExpenseChangeFields(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := validateUpdateExpenseChangedValues(changeFields, args); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	var existing map[string]any
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return ResultEnvelope{}, err
	}

	form := url.Values{}
	for _, token := range changeFields {
		form.Add("changeFields", token)
	}
	if v, ok := args["amount"].(float64); ok {
		normalized, err := expenseAmountForClockify(args, v)
		if err != nil {
			return ResultEnvelope{}, err
		}
		form.Set("amount", strconv.FormatFloat(normalized, 'f', -1, 64))
	} else if v, ok := existing["amount"].(float64); ok {
		form.Set("amount", strconv.FormatFloat(v, 'f', -1, 64))
	}
	if v := stringArg(args, "date"); v != "" {
		form.Set("date", v)
	} else if v, _ := existing["date"].(string); v != "" {
		form.Set("date", v)
	}
	if v := stringArg(args, "category_id"); v != "" {
		form.Set("categoryId", v)
	} else if v, _ := existing["categoryId"].(string); v != "" {
		form.Set("categoryId", v)
	}
	if v := stringArg(args, "project_id"); v != "" {
		form.Set("projectId", v)
	}
	if v := stringArg(args, "task_id"); v != "" {
		form.Set("taskId", v)
	}
	userID := stringArg(args, "user_id")
	if userID == "" {
		current, err := s.getCurrentUser(ctx)
		if err != nil {
			return ResultEnvelope{}, fmt.Errorf("resolve user_id from current user: %w", err)
		}
		userID = current.ID
	}
	if userID != "" {
		form.Set("userId", userID)
	}
	if v, ok := args["notes"].(string); ok {
		form.Set("notes", v)
	}
	if v, ok := args["billable"].(bool); ok {
		form.Set("billable", strconv.FormatBool(v))
	} else if v, ok := existing["billable"].(bool); ok {
		form.Set("billable", strconv.FormatBool(v))
	}

	var updated map[string]any
	if err := s.Client.PutMultipart(ctx, path, form, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_expense", expenseViewFromRaw(updated), map[string]any{"workspaceId": wsID}), nil
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
	expensePath, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ResultEnvelope{}, err
	}

	if dryrun.Enabled(args) {
		var expense map[string]any
		if err := s.Client.Get(ctx, expensePath, nil, &expense); err != nil {
			return ResultEnvelope{}, err
		}
		return ResultEnvelope{
			OK:     true,
			Action: "clockify_delete_expense",
			Data:   dryrun.WrapResult(expense, "clockify_delete_expense"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, expensePath); err != nil {
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

	path, err := paths.Workspace(wsID, "expenses", "categories")
	if err != nil {
		return ResultEnvelope{}, err
	}
	// Upstream returns {count: N, categories: [...]}. Probe evidence:
	// clockify-api-probe-lab/findings/expenses.md (rev 2 2026-05-02).
	var envelope struct {
		Count      int              `json:"count"`
		Categories []map[string]any `json:"categories"`
	}
	if err := s.Client.Get(ctx, path, nil, &envelope); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_list_expense_categories", envelope.Categories, map[string]any{
		"workspaceId": wsID,
		"count":       envelope.Count,
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

	body := expenseCategoryBody(args)
	body["name"] = name
	if dryrun.Enabled(args) {
		return ok("clockify_create_expense_category", dryrun.Preview("clockify_create_expense_category", body), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "expenses", "categories")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_create_expense_category", created, map[string]any{"workspaceId": wsID}), nil
}

func expenseCategoryBody(args map[string]any) map[string]any {
	body := map[string]any{}
	if v := stringArg(args, "name"); v != "" {
		body["name"] = v
	}
	if v, ok := args["has_unit_price"].(bool); ok {
		body["hasUnitPrice"] = v
	}
	if v, ok := args["price_in_cents"].(float64); ok {
		body["priceInCents"] = int(v)
	} else if v, ok := args["price_in_cents"].(int); ok {
		body["priceInCents"] = v
	}
	if v := stringArg(args, "unit"); v != "" {
		body["unit"] = v
	}
	return body
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

	if archived, ok := args["archived"].(bool); ok {
		args["archived"] = archived
		return s.archiveExpenseCategory(ctx, args)
	}

	body := expenseCategoryBody(args)
	if len(body) == 0 {
		return ResultEnvelope{}, fmt.Errorf("at least one of name, has_unit_price, price_in_cents, unit, or archived is required")
	}
	if dryrun.Enabled(args) {
		return ok("clockify_update_expense_category", dryrun.Preview("clockify_update_expense_category", map[string]any{
			"category_id": catID,
			"body":        body,
		}), map[string]any{"workspaceId": wsID, "categoryId": catID}), nil
	}

	path, err := paths.Workspace(wsID, "expenses", "categories", catID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_update_expense_category", updated, map[string]any{
		"workspaceId": wsID,
		"categoryId":  catID,
	}), nil
}

func (s *Service) archiveExpenseCategory(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ResultEnvelope{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ResultEnvelope{}, err
	}
	archived := true
	if v, ok := args["archived"].(bool); ok {
		archived = v
	}
	body := map[string]any{"archived": archived}
	if dryrun.Enabled(args) {
		return ok("clockify_archive_expense_category", dryrun.Preview("clockify_archive_expense_category", map[string]any{
			"category_id": catID,
			"body":        body,
		}), map[string]any{"workspaceId": wsID, "categoryId": catID}), nil
	}
	path, err := paths.Workspace(wsID, "expenses", "categories", catID, "status")
	if err != nil {
		return ResultEnvelope{}, err
	}
	var updated map[string]any
	if err := s.Client.Patch(ctx, path, body, &updated); err != nil {
		return ResultEnvelope{}, err
	}
	return ok("clockify_archive_expense_category", updated, map[string]any{
		"workspaceId": wsID,
		"categoryId":  catID,
		"archived":    archived,
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

	path, err := paths.Workspace(wsID, "expenses", "categories", catID)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
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
	body, err := buildExpenseReportBody(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	path, err := paths.Workspace(wsID, "reports", "expenses", "detailed")
	if err != nil {
		return ResultEnvelope{}, err
	}
	data, binary, err := s.postReportsAPI(ctx, path, body)
	if err != nil {
		return ResultEnvelope{}, err
	}
	meta := map[string]any{
		"workspaceId": wsID,
		"source":      "reports-api",
		"exportType":  body["exportType"],
	}
	if binary {
		meta["binary"] = true
	} else {
		appendExpenseReportViews(data)
	}
	return ok("clockify_expense_report", data, meta), nil
}
