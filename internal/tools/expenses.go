package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
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
		}), envelopeSchemaFor[[]CompactExpenseView]("clockify_list_expenses")), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
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
		{Tool: toolRW("clockify_create_expense", "Create an expense. amount defaults to major currency units; pass amount_unit:\"minor\" for cents. Receipt upload is optional; provide all three file_* fields together to attach one.", map[string]any{
			"type":     "object",
			"required": []string{"amount", "date", "category_id"},
			"properties": map[string]any{
				"amount":              map[string]any{"type": "number", "description": "Expense amount. Defaults to major currency units, e.g. 125.00 for $125.00; use amount_unit:\"minor\" for cents/minor units."},
				"amount_unit":         map[string]any{"type": "string", "enum": []string{"major", "minor"}, "description": "Unit for amount. major (default) sends the value as entered; minor divides by 100 before sending to Clockify."},
				"date":                map[string]any{"type": "string", "description": "Expense date as an ISO 8601 / RFC 3339 timestamp, e.g. 2026-05-16T14:30:00Z or 2026-05-16T14:30:00+02:00."},
				"category_id":         map[string]any{"type": "string", "description": "Expense category ID (required)"},
				"user_id":             map[string]any{"type": "string", "description": "User the expense is logged against; defaults to the calling user"},
				"project_id":          map[string]any{"type": "string", "description": "Project ID (optional)"},
				"task_id":             map[string]any{"type": "string", "description": "Task ID (optional)"},
				"notes":               map[string]any{"type": "string", "description": "Free-form notes"},
				"billable":            map[string]any{"type": "boolean", "description": "Whether the expense is billable"},
				"file_name":           map[string]any{"type": "string", "description": "Optional receipt file name. Send file_name, file_content_base64, and file_content_type together to attach a receipt."},
				"file_content_base64": map[string]any{"type": "string", "description": "Optional base64-encoded receipt body. Decoded payload is capped at the multipart file limit (10 MiB)."},
				"file_content_type":   map[string]any{"type": "string", "description": "Optional MIME type for the receipt, e.g. application/pdf or image/png."},
				"dry_run":             map[string]any{"type": "boolean", "description": "Preview the resolved expense payload without creating it."},
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
				"change_fields": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{"USER", "DATE", "PROJECT", "TASK", "CATEGORY", "NOTES", "AMOUNT", "BILLABLE"}}, "description": "Field tokens to update. Each listed token requires the matching argument: USER=user_id, DATE=date, PROJECT=project_id, TASK=task_id, CATEGORY=category_id, NOTES=notes, AMOUNT=amount, BILLABLE=billable. Receipt-file replacement is not supported on update; delete and recreate the expense with file_* fields instead."},
				"amount":        map[string]any{"type": "number", "description": "Expense amount. Defaults to major currency units; use amount_unit:\"minor\" for cents/minor units."},
				"amount_unit":   map[string]any{"type": "string", "enum": []string{"major", "minor"}, "description": "Unit for amount when AMOUNT is included in change_fields."},
				"date":          map[string]any{"type": "string", "description": "Expense date as an ISO 8601 / RFC 3339 timestamp, e.g. 2026-05-16T14:30:00Z or 2026-05-16T14:30:00+02:00."},
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
		{Tool: toolDestructive("clockify_delete_expense", "Permanently delete an expense by ID. Billing impact; destructive; supports dry_run preview.", map[string]any{
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
		{Tool: withOutputSchema(toolRO("clockify_list_expense_categories", "List expense categories in the workspace, paginated via page and page_size.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"page":      map[string]any{"type": "integer", "description": "Page number (default 1)"},
				"page_size": map[string]any{"type": "integer", "description": "Items per page (default 50)"},
			},
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
		{Tool: toolDestructive("clockify_delete_expense_category", "Permanently delete an expense category. Billing impact; destructive; supports dry_run preview.", map[string]any{
			"type":     "object",
			"required": []string{"category_id"},
			"properties": map[string]any{
				"category_id": map[string]any{"type": "string"},
				"dry_run":     map[string]any{"type": "boolean"},
			},
		}), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.deleteExpenseCategory(ctx, args)
		}},
	}
}

// ---------------------------------------------------------------------------
// Expense handlers
// ---------------------------------------------------------------------------

// workspaceDefaultCurrency returns the pinned workspace's default currency
// code (e.g. "USD"). Expenses carry no currency of their own. Returns "" on
// any failure so callers degrade to an unlabelled amount rather than erroring.
func (s *Service) workspaceDefaultCurrency(ctx context.Context, wsID string) string {
	path, err := paths.Workspace(wsID)
	if err != nil {
		return ""
	}
	var ws struct {
		Currencies []struct {
			Code      string `json:"code"`
			IsDefault bool   `json:"isDefault"`
		} `json:"currencies"`
	}
	if err := s.Client.Get(ctx, path, nil, &ws); err != nil {
		return ""
	}
	for _, c := range ws.Currencies {
		if c.IsDefault {
			return c.Code
		}
	}
	if len(ws.Currencies) > 0 {
		return ws.Currencies[0].Code
	}
	return ""
}

func (s *Service) listExpenses(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	page := intArg(args, "page", 1)
	pageSize := intArg(args, "page_size", 50)

	path, err := paths.Workspace(wsID, "expenses")
	if err != nil {
		return ToolResult{}, err
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
		return ToolResult{}, err
	}
	items := envelope.Expenses.Expenses
	// has_more from the known total; the page-full heuristic is only a
	// fallback for when the API does not report a usable count, otherwise an
	// exact final page (count == page*pageSize) would falsely report more.
	hasMore := len(items) == pageSize
	if envelope.Expenses.Count > 0 {
		hasMore = page*pageSize < envelope.Expenses.Count
	}
	workspaceCurrency := s.workspaceDefaultCurrency(ctx, wsID)
	return ok("clockify_list_expenses", compactExpenseViewsFromRaw(items, workspaceCurrency), emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
		"total":       envelope.Expenses.Count,
		"page":        page,
		"pageSize":    pageSize,
		"has_more":    hasMore,
	}, "clockify_record_expense")), nil
}

func (s *Service) getExpense(ctx context.Context, args map[string]any) (ToolResult, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	path, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ToolResult{}, err
	}
	var expense map[string]any
	if err := s.Client.Get(ctx, path, nil, &expense); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_get_expense", expenseViewFromRaw(expense, s.workspaceDefaultCurrency(ctx, wsID)), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) createExpense(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	// Required: amount, date (RFC3339), category_id. user_id defaults
	// to the calling user via /user — the upstream rejects multipart
	// POSTs that omit userId with a 400.
	amount, hasAmount := numberArg(args, "amount")
	if !hasAmount {
		return ToolResult{}, fmt.Errorf("amount is required")
	}
	if amount <= 0 {
		return ToolResult{}, fmt.Errorf("amount must be greater than 0")
	}
	date := stringArg(args, "date")
	if date == "" {
		return ToolResult{}, fmt.Errorf("date is required")
	}
	loc, err := s.locationFromArgs(args)
	if err != nil {
		return ToolResult{}, err
	}
	parsedDate, err := parseFlexibleDateTime(date, loc)
	if err != nil {
		return ToolResult{}, fmt.Errorf("could not parse date %q for date — use YYYY-MM-DD or RFC3339", date)
	}
	date = parsedDate.UTC().Format(time.RFC3339)
	categoryID := stringArg(args, "category_id")
	if categoryID == "" {
		return ToolResult{}, fmt.Errorf("category_id is required")
	}
	userID := stringArg(args, "user_id")
	if userID == "" {
		current, err := s.getCurrentUser(ctx)
		if err != nil {
			return ToolResult{}, fmt.Errorf("resolve user_id from current user: %w", err)
		}
		userID = current.ID
	}

	form := url.Values{}
	form.Set("userId", userID)
	normalizedAmount, err := expenseAmountForClockify(args, amount)
	if err != nil {
		return ToolResult{}, err
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
		return ToolResult{}, err
	}

	files, err := expenseReceiptFiles(args)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		preview := make(map[string]any, len(form)+1)
		for key := range form {
			preview[key] = form.Get(key)
		}
		if len(files) > 0 {
			preview["receiptFiles"] = len(files)
		}
		return ok("clockify_create_expense", dryrunPreviewPayload("clockify_create_expense", preview), map[string]any{"workspaceId": wsID}), nil
	}

	var created map[string]any
	if len(files) > 0 {
		if err := s.Client.PostMultipartWithFiles(ctx, path, form, files, &created); err != nil {
			return ToolResult{}, err
		}
	} else {
		if err := s.Client.PostMultipart(ctx, path, form, &created); err != nil {
			return ToolResult{}, err
		}
	}
	return ok("clockify_create_expense", expenseViewFromRaw(created, s.workspaceDefaultCurrency(ctx, wsID)), map[string]any{"workspaceId": wsID}), nil
}

// expenseReceiptFiles reads the optional receipt-file trio. All three
// fields must be supplied together: a partial trio fails closed so a
// caller never uploads an unnamed or untyped receipt by accident.
// Returns nil (no error) when no file field is present at all.
func expenseReceiptFiles(args map[string]any) ([]clockify.MultipartFile, error) {
	name := strings.TrimSpace(stringArg(args, "file_name"))
	body := strings.TrimSpace(stringArg(args, "file_content_base64"))
	contentType := strings.TrimSpace(stringArg(args, "file_content_type"))
	if name == "" && body == "" && contentType == "" {
		return nil, nil
	}
	// An empty file_content_base64 is treated as a missing field, so an
	// empty receipt body fails the trio check here — before any HTTP
	// call. A non-empty, syntactically valid base64 string always
	// decodes to at least one byte, so there is no separate zero-byte
	// case to guard once this check has passed.
	if name == "" || body == "" || contentType == "" {
		return nil, fmt.Errorf("file_name, file_content_base64, and file_content_type must be provided together")
	}
	data, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("file_content_base64 is not valid base64: %w", err)
	}
	return []clockify.MultipartFile{{
		FieldName:   "file",
		Filename:    name,
		ContentType: contentType,
		Data:        data,
	}}, nil
}

// validUpdateExpenseChangeFields lists the upstream-accepted tokens for
// the multipart `changeFields` field on PUT /expenses/{id}. FILE is
// intentionally absent: this tool does not replace receipts on update.
// clockify_create_expense accepts file_* fields, so the supported path
// for changing a receipt is delete + recreate.
var validUpdateExpenseChangeFields = map[string]bool{
	"USER":     true,
	"DATE":     true,
	"PROJECT":  true,
	"TASK":     true,
	"CATEGORY": true,
	"NOTES":    true,
	"AMOUNT":   true,
	"BILLABLE": true,
}

func parseUpdateExpenseChangeFields(args map[string]any) ([]string, error) {
	raw, hasChange := args["change_fields"]
	if !hasChange {
		return nil, fmt.Errorf("change_fields is required and must list at least one of USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE")
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
		return nil, fmt.Errorf("change_fields is required and must list at least one of USER, DATE, PROJECT, TASK, CATEGORY, NOTES, AMOUNT, BILLABLE")
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
			if _, ok := numberArg(args, "amount"); !ok {
				return fmt.Errorf("change_fields includes AMOUNT but amount is required")
			}
		case "BILLABLE":
			if _, ok := args["billable"].(bool); !ok {
				return fmt.Errorf("change_fields includes BILLABLE but billable is required")
			}
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

func (s *Service) updateExpense(ctx context.Context, args map[string]any) (ToolResult, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ToolResult{}, err
	}
	changeFields, err := parseUpdateExpenseChangeFields(args)
	if err != nil {
		return ToolResult{}, err
	}
	if err := validateUpdateExpenseChangedValues(changeFields, args); err != nil {
		return ToolResult{}, err
	}
	normalizedDate := ""
	if date := stringArg(args, "date"); date != "" {
		loc, err := s.locationFromArgs(args)
		if err != nil {
			return ToolResult{}, err
		}
		parsed, err := parseFlexibleDateTime(date, loc)
		if err != nil {
			return ToolResult{}, fmt.Errorf("could not parse date %q for date — use YYYY-MM-DD or RFC3339", date)
		}
		normalizedDate = parsed.UTC().Format(time.RFC3339)
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	path, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ToolResult{}, err
	}

	var existing map[string]any
	if err := s.Client.Get(ctx, path, nil, &existing); err != nil {
		return ToolResult{}, err
	}

	form := url.Values{}
	for _, token := range changeFields {
		form.Add("changeFields", token)
	}
	if v, ok := numberArg(args, "amount"); ok {
		if v <= 0 {
			return ToolResult{}, fmt.Errorf("amount must be greater than 0")
		}
		normalized, err := expenseAmountForClockify(args, v)
		if err != nil {
			return ToolResult{}, err
		}
		form.Set("amount", strconv.FormatFloat(normalized, 'f', -1, 64))
	} else if v, ok := reportNumber(firstPresent(existing, "amount")); ok {
		form.Set("amount", strconv.FormatFloat(v, 'f', -1, 64))
	} else if amount, ok := existing["amount"].(map[string]any); ok {
		if v, ok := reportNumber(firstPresent(amount, "amount", "value", "total")); ok {
			form.Set("amount", strconv.FormatFloat(v, 'f', -1, 64))
		}
	}
	if normalizedDate != "" {
		form.Set("date", normalizedDate)
	} else if v, _ := existing["date"].(string); v != "" {
		form.Set("date", v)
	}
	if v := stringArg(args, "category_id"); v != "" {
		form.Set("categoryId", v)
	} else if v := firstReportString(existing, "categoryId", "category_id"); v != "" {
		form.Set("categoryId", v)
	} else if category, ok := existing["category"].(map[string]any); ok {
		if v := firstReportString(category, "id", "_id", "categoryId", "category_id"); v != "" {
			form.Set("categoryId", v)
		}
	}
	if v := stringArg(args, "project_id"); v != "" {
		form.Set("projectId", v)
	} else if v := firstReportString(existing, "projectId", "project_id"); v != "" {
		form.Set("projectId", v)
	}
	if v := stringArg(args, "task_id"); v != "" {
		form.Set("taskId", v)
	} else if v := firstReportString(existing, "taskId", "task_id"); v != "" {
		form.Set("taskId", v)
	}
	userID := stringArg(args, "user_id")
	if userID == "" {
		userID = firstReportString(existing, "userId", "user_id")
		if userID == "" {
			current, err := s.getCurrentUser(ctx)
			if err != nil {
				return ToolResult{}, fmt.Errorf("resolve user_id from current user: %w", err)
			}
			userID = current.ID
		}
	}
	if userID != "" {
		form.Set("userId", userID)
	}
	if v, ok := args["notes"].(string); ok {
		form.Set("notes", v)
	} else if v := firstReportString(existing, "notes", "note"); v != "" {
		form.Set("notes", v)
	}
	if v, ok := args["billable"].(bool); ok {
		form.Set("billable", strconv.FormatBool(v))
	} else if v, ok := existing["billable"].(bool); ok {
		form.Set("billable", strconv.FormatBool(v))
	}

	var updated map[string]any
	if err := s.Client.PutMultipart(ctx, path, form, &updated); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_update_expense", expenseViewFromRaw(updated, s.workspaceDefaultCurrency(ctx, wsID)), map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) deleteExpense(ctx context.Context, args map[string]any) (ToolResult, error) {
	expenseID := stringArg(args, "expense_id")
	if err := resolve.ValidateID(expenseID, "expense_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}
	expensePath, err := paths.Workspace(wsID, "expenses", expenseID)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		var expense map[string]any
		if err := s.Client.Get(ctx, expensePath, nil, &expense); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_expense",
			Data:   dryrun.WrapResult(expense, "clockify_delete_expense"),
			Meta:   map[string]any{"workspaceId": wsID},
		}, nil
	}

	if err := s.Client.Delete(ctx, expensePath); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_delete_expense", map[string]any{
		"deleted":   true,
		"expenseId": expenseID,
	}, map[string]any{"workspaceId": wsID}), nil
}

func (s *Service) listExpenseCategories(ctx context.Context, args map[string]any) (ToolResult, error) {
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	all, err := s.fetchAllExpenseCategories(ctx, wsID)
	if err != nil {
		return ToolResult{}, err
	}
	page, pageSize := paginationFromArgs(args)
	start := (page - 1) * pageSize
	end := min(start+pageSize, len(all))
	items := []map[string]any{}
	if start < len(all) {
		items = all[start:end]
	}
	return ok("clockify_list_expense_categories", items, emptyListMeta(map[string]any{
		"workspaceId": wsID,
		"count":       len(items),
		"total":       len(all),
		"page":        page,
		"pageSize":    pageSize,
		"has_more":    end < len(all),
	}, "clockify_expenses_categories_create")), nil
}

func (s *Service) createExpenseCategory(ctx context.Context, args map[string]any) (ToolResult, error) {
	name := stringArg(args, "name")
	if name == "" {
		return ToolResult{}, fmt.Errorf("name is required")
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	body := expenseCategoryBody(args)
	body["name"] = name
	if dryrun.Enabled(args) {
		return ok("clockify_create_expense_category", dryrun.Preview("clockify_create_expense_category", body), map[string]any{"workspaceId": wsID}), nil
	}
	path, err := paths.Workspace(wsID, "expenses", "categories")
	if err != nil {
		return ToolResult{}, err
	}
	var created map[string]any
	if err := s.Client.Post(ctx, path, body, &created); err != nil {
		return ToolResult{}, err
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
	if v, ok := numberArg(args, "price_in_cents"); ok {
		body["priceInCents"] = int64(v)
	}
	if v := stringArg(args, "unit"); v != "" {
		body["unit"] = v
	}
	return body
}

func (s *Service) updateExpenseCategory(ctx context.Context, args map[string]any) (ToolResult, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if archived, ok := args["archived"].(bool); ok {
		args["archived"] = archived
		return s.archiveExpenseCategory(ctx, args)
	}

	body := expenseCategoryBody(args)
	if len(body) == 0 {
		return ToolResult{}, fmt.Errorf("at least one of name, has_unit_price, price_in_cents, unit, or archived is required")
	}
	existing, err := s.findExpenseCategory(ctx, wsID, catID)
	if err != nil {
		return ToolResult{}, err
	}
	body = expenseCategoryUpdateBody(existing, args)
	if dryrun.Enabled(args) {
		return ok("clockify_update_expense_category", dryrun.Preview("clockify_update_expense_category", map[string]any{
			"category_id": catID,
			"body":        body,
		}), map[string]any{"workspaceId": wsID, "categoryId": catID}), nil
	}

	path, err := paths.Workspace(wsID, "expenses", "categories", catID)
	if err != nil {
		return ToolResult{}, err
	}
	var updated map[string]any
	if err := s.Client.Put(ctx, path, body, &updated); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_update_expense_category", updated, map[string]any{
		"workspaceId": wsID,
		"categoryId":  catID,
	}), nil
}

func (s *Service) findExpenseCategory(ctx context.Context, wsID, catID string) (map[string]any, error) {
	categories, err := s.fetchAllExpenseCategories(ctx, wsID)
	if err != nil {
		return nil, err
	}
	for _, category := range categories {
		if firstReportString(category, "id", "_id") == catID {
			return category, nil
		}
	}
	return nil, fmt.Errorf("expense category %q not found", catID)
}

// fetchAllExpenseCategories walks every server page of
// GET /expenses/categories. Clockify caps this endpoint's page size, so a
// single un-paginated GET silently drops categories past the first page.
// It de-duplicates by id, so the loop terminates even if the endpoint
// ignores `page` and keeps returning the first page.
func (s *Service) fetchAllExpenseCategories(ctx context.Context, wsID string) ([]map[string]any, error) {
	path, err := paths.Workspace(wsID, "expenses", "categories")
	if err != nil {
		return nil, err
	}
	const pageSize = 50
	seen := make(map[string]bool)
	all := []map[string]any{}
	for page := 1; page <= 200; page++ {
		var envelope struct {
			Categories []map[string]any `json:"categories"`
		}
		query := map[string]string{
			"page":      strconv.Itoa(page),
			"page-size": strconv.Itoa(pageSize),
		}
		if err := s.Client.Get(ctx, path, query, &envelope); err != nil {
			return nil, err
		}
		added := 0
		for _, category := range envelope.Categories {
			id := firstReportString(category, "id", "_id")
			if id != "" && seen[id] {
				continue
			}
			if id != "" {
				seen[id] = true
			}
			all = append(all, category)
			added++
		}
		if added < pageSize {
			break
		}
	}
	return all, nil
}

func expenseCategoryUpdateBody(existing map[string]any, args map[string]any) map[string]any {
	body := expenseCategoryBody(args)
	if _, set := body["name"]; !set {
		if name := firstReportString(existing, "name"); name != "" {
			body["name"] = name
		}
	}
	return body
}

func (s *Service) archiveExpenseCategory(ctx context.Context, args map[string]any) (ToolResult, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
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
		return ToolResult{}, err
	}
	var updated map[string]any
	if err := s.Client.Patch(ctx, path, body, &updated); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_archive_expense_category", updated, map[string]any{
		"workspaceId": wsID,
		"categoryId":  catID,
		"archived":    archived,
	}), nil
}

func (s *Service) deleteExpenseCategory(ctx context.Context, args map[string]any) (ToolResult, error) {
	catID := stringArg(args, "category_id")
	if err := resolve.ValidateID(catID, "category_id"); err != nil {
		return ToolResult{}, err
	}
	wsID, err := s.ResolveWorkspaceID(ctx)
	if err != nil {
		return ToolResult{}, err
	}

	if dryrun.Enabled(args) {
		return ToolResult{
			OK:     true,
			Action: "clockify_delete_expense_category",
			Data: dryrun.MinimalResult("clockify_delete_expense_category", map[string]any{
				"category_id": catID,
				"preflight":   "archive_then_delete",
			}),
			Meta: map[string]any{"workspaceId": wsID},
		}, nil
	}

	statusPath, err := paths.Workspace(wsID, "expenses", "categories", catID, "status")
	if err != nil {
		return ToolResult{}, err
	}
	var archived map[string]any
	if err := s.Client.Patch(ctx, statusPath, map[string]any{"archived": true}, &archived); err != nil {
		return ToolResult{}, fmt.Errorf("archive expense category before delete failed: %w", err)
	}

	path, err := paths.Workspace(wsID, "expenses", "categories", catID)
	if err != nil {
		return ToolResult{}, err
	}
	if err := s.Client.Delete(ctx, path); err != nil {
		return ToolResult{}, err
	}
	return ok("clockify_delete_expense_category", map[string]any{
		"deleted":    true,
		"categoryId": catID,
		"archived":   true,
	}, map[string]any{"workspaceId": wsID}), nil
}
