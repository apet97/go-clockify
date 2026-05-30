package tools

import (
	"maps"
	"strings"
	"time"
)

// ExpenseView is the pass-through expense projection: the raw upstream expense
// object as a generic map.
type ExpenseView map[string]any

// CompactExpenseView is the trimmed expense projection returned by expense
// tools: id, date, normalized amount, category/project/user, and flags.
type CompactExpenseView struct {
	ID           string     `json:"id"`
	Date         string     `json:"date,omitempty"`
	Amount       *MoneyView `json:"amount,omitempty"`
	CategoryID   string     `json:"categoryId,omitempty"`
	CategoryName string     `json:"categoryName,omitempty"`
	ProjectID    string     `json:"projectId,omitempty"`
	ProjectName  string     `json:"projectName,omitempty"`
	UserID       string     `json:"userId,omitempty"`
	Billable     bool       `json:"billable,omitempty"`
	Locked       bool       `json:"locked,omitempty"`
	Notes        string     `json:"notes,omitempty"`
}

// TimeOffPolicyView is the pass-through time-off-policy projection: the raw
// upstream policy object as a generic map.
type TimeOffPolicyView map[string]any

// CompactTimeOffPolicyView is the trimmed time-off-policy projection: id, name,
// time unit, and assignee counts.
type CompactTimeOffPolicyView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Archived       bool   `json:"archived,omitempty"`
	TimeUnit       string `json:"timeUnit,omitempty"`
	UserCount      int    `json:"userCount"`
	UserGroupCount int    `json:"userGroupCount"`
	AssigneeCount  int    `json:"assigneeCount"`
}

// TimeOffRequestView is the pass-through time-off-request projection: the raw
// upstream request object as a generic map.
type TimeOffRequestView map[string]any

// TimeOffBalanceView is the pass-through time-off-balance projection: the raw
// upstream balance object as a generic map.
type TimeOffBalanceView map[string]any

func compactExpenseViewFromRaw(raw map[string]any, workspaceCurrency string) CompactExpenseView {
	currency := firstNonEmptyString(reportCurrency(raw), workspaceCurrency)
	return CompactExpenseView{
		ID:           firstReportString(raw, "id", "_id", "expenseId", "expense_id"),
		Date:         firstReportString(raw, "date"),
		Amount:       moneyFromAny(firstPresent(raw, "amount", "billableAmount", "total"), currency),
		CategoryID:   firstReportString(raw, "categoryId", "category_id"),
		CategoryName: firstReportString(raw, "categoryName", "category_name"),
		ProjectID:    firstReportString(raw, "projectId", "project_id"),
		ProjectName:  firstReportString(raw, "projectName", "project_name"),
		UserID:       firstReportString(raw, "userId", "user_id"),
		Billable:     boolFromAny(firstPresent(raw, "billable", "isBillable")),
		Locked:       boolFromAny(firstPresent(raw, "locked", "isLocked")),
		Notes:        firstReportString(raw, "notes", "note"),
	}
}

func compactExpenseViewsFromRaw(items []map[string]any, workspaceCurrency string) []CompactExpenseView {
	out := make([]CompactExpenseView, 0, len(items))
	for _, item := range items {
		out = append(out, compactExpenseViewFromRaw(item, workspaceCurrency))
	}
	return out
}

func expenseViewFromRaw(raw map[string]any, workspaceCurrency string) ExpenseView {
	view := ExpenseView(maps.Clone(raw))
	currency := firstNonEmptyString(reportCurrency(raw), workspaceCurrency)
	view["amount"] = moneyFromAny(firstPresent(raw, "amount", "billableAmount", "total"), currency)
	view["currency"] = currency
	view["status"] = strings.ToUpper(firstReportString(raw, "status", "approvalStatus", "invoicingState"))
	view["has_file"] = hasAnyKey(raw, "file", "fileId", "file_id", "receipt", "receiptId")
	category := map[string]any{
		"id":             firstReportString(raw, "categoryId", "category_id"),
		"price_in_cents": firstPresent(raw, "categoryPriceInCents", "priceInCents", "price_in_cents"),
		"archived":       firstPresent(raw, "categoryArchived", "archived"),
		"has_unit_price": firstPresent(raw, "categoryHasUnitPrice", "hasUnitPrice", "has_unit_price"),
	}
	if name := firstReportString(raw, "categoryName", "category_name"); name != "" {
		category["name"] = name
	}
	if unit := firstReportString(raw, "categoryUnit", "unit"); unit != "" {
		category["unit"] = unit
	}
	view["category"] = category
	view["receipt"] = map[string]any{
		"has_file":  hasAnyKey(raw, "file", "fileId", "file_id", "receipt", "receiptId"),
		"file_id":   firstReportString(raw, "fileId", "file_id", "receiptId", "receipt_id"),
		"file_name": firstReportString(raw, "fileName", "file_name", "receiptName", "receipt_name"),
		"file_url":  firstReportString(raw, "fileUrl", "file_url", "receiptUrl", "receipt_url"),
	}
	view["tax"] = map[string]any{
		"reimbursable": firstPresent(raw, "reimbursable", "isReimbursable"),
		"tax":          firstPresent(raw, "tax"),
		"tax2":         firstPresent(raw, "tax2"),
		"tax_amount":   moneyFromAny(firstPresent(raw, "taxAmount", "tax_amount"), currency),
	}
	view["entities"] = map[string]any{
		"user": map[string]any{
			"id":   firstReportString(raw, "userId", "user_id"),
			"name": firstReportString(raw, "userName", "user_name"),
		},
		"client": map[string]any{
			"id":   firstReportString(raw, "clientId", "client_id"),
			"name": firstReportString(raw, "clientName", "client_name"),
		},
		"project": map[string]any{
			"id":   firstReportString(raw, "projectId", "project_id"),
			"name": firstReportString(raw, "projectName", "project_name"),
		},
		"task": map[string]any{
			"id":   firstReportString(raw, "taskId", "task_id"),
			"name": firstReportString(raw, "taskName", "task_name"),
		},
	}
	view["approval"] = map[string]any{
		"request_id":      firstReportString(raw, "approvalRequestId", "approval_request_id"),
		"state":           strings.ToUpper(firstReportString(raw, "approvalState", "approvalStatus", "state", "status")),
		"detailed_status": firstPresent(raw, "detailedApprovalStatus", "detailed_approval_status"),
		"source":          "expense_api",
	}
	view["invoicing"] = map[string]any{
		"state":          strings.ToUpper(firstReportString(raw, "invoicingState", "invoicing_state")),
		"invoice_id":     firstReportString(raw, "invoiceId", "invoice_id"),
		"invoice_number": firstReportString(raw, "invoiceNumber", "invoice_number"),
		"source":         "expense_api",
	}
	return view
}

func compactTimeOffPolicyViewFromRaw(raw map[string]any) CompactTimeOffPolicyView {
	userCount := arrayLikeLen(firstPresent(raw, "userIds", "user_ids", "assigneeIds", "assignee_ids"))
	groupCount := arrayLikeLen(firstPresent(raw, "userGroupIds", "user_group_ids", "groupIds", "group_ids"))
	return CompactTimeOffPolicyView{
		ID:             firstReportString(raw, "id", "_id", "policyId", "policy_id"),
		Name:           firstReportString(raw, "name"),
		Archived:       boolFromAny(firstPresent(raw, "archived", "isArchived")),
		TimeUnit:       firstReportString(raw, "timeUnit", "time_unit"),
		UserCount:      userCount,
		UserGroupCount: groupCount,
		AssigneeCount:  userCount + groupCount,
	}
}

func compactTimeOffPolicyViewsFromRaw(items []map[string]any) []CompactTimeOffPolicyView {
	out := make([]CompactTimeOffPolicyView, 0, len(items))
	for _, item := range items {
		out = append(out, compactTimeOffPolicyViewFromRaw(item))
	}
	return out
}

func arrayLikeLen(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []string:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func timeOffPolicyViewFromRaw(raw map[string]any) TimeOffPolicyView {
	view := TimeOffPolicyView(maps.Clone(raw))
	view["balance_rules"] = map[string]any{
		"days_per_year":  firstPresent(raw, "daysPerYear", "days_per_year"),
		"hours_per_year": firstPresent(raw, "hoursPerYear", "hours_per_year"),
	}
	return view
}

func timeOffRequestViewFromRaw(raw map[string]any) TimeOffRequestView {
	status := timeOffRequestStatus(raw)
	requestID := firstReportString(raw, "id", "_id", "requestId", "request_id")
	policyID := firstReportString(raw, "policyId", "policy_id")
	userID := firstReportString(raw, "userId", "user_id")
	view := TimeOffRequestView{
		"id":        requestID,
		"requestId": requestID,
		"status":    status,
		"note":      firstReportString(raw, "note", "reason"),
		"policyId":  policyID,
		"userId":    userID,
	}
	view["audit"] = map[string]any{
		"created_at": firstReportString(raw, "createdAt", "created_at"),
		"updated_at": firstReportString(raw, "updatedAt", "updated_at"),
		"updated_by": firstReportString(raw, "updatedBy", "updated_by"),
		"source":     "time_off_api",
	}
	// The time-off approve/deny PATCH responses omit user and policy names;
	// emit only the fields the upstream actually returned so the receipt
	// carries no empty-string noise.
	user := map[string]any{"id": userID}
	if name := firstReportString(raw, "userName", "user_name"); name != "" {
		user["name"] = name
	}
	if email := firstReportString(raw, "userEmail", "user_email"); email != "" {
		user["email"] = email
	}
	view["user"] = user
	policy := map[string]any{"id": policyID}
	if name := firstReportString(raw, "policyName", "policy_name"); name != "" {
		policy["name"] = name
	}
	view["policy"] = policy
	view["period"] = firstPresent(raw, "timeOffPeriod", "time_off_period", "period")
	view["day_period"] = firstPresent(raw, "dayPeriod", "day_period", "halfDay", "half_day")
	view["duration"] = map[string]any{
		"days":  firstPresent(raw, "days", "durationDays", "balanceDiff"),
		"hours": firstPresent(raw, "hours", "durationHours"),
	}
	if stale := timeOffRequestStale(status, raw); stale != nil {
		view["stale"] = stale
	}
	view["actions"] = timeOffActions(status)
	view["suggestedActions"] = timeOffSuggestions(requestID, policyID, status)
	return view
}

// timeOffRequestStale flags a PENDING time-off request whose period has
// already elapsed: it can no longer be acted on meaningfully, so an approver
// should see it as cleanup, not a live request.
func timeOffRequestStale(status string, raw map[string]any) map[string]any {
	if status != "PENDING" {
		return nil
	}
	period, ok := firstPresent(raw, "timeOffPeriod", "time_off_period", "period").(map[string]any)
	if !ok {
		return nil
	}
	inner, ok := period["period"].(map[string]any)
	if !ok {
		inner = period
	}
	endRaw := firstReportString(inner, "end", "endDate", "end_date")
	if endRaw == "" {
		return nil
	}
	end, err := parseFlexibleDateTime(endRaw, time.UTC)
	if err != nil || !end.Before(time.Now().UTC()) {
		return nil
	}
	return map[string]any{
		"reason":    "pending_period_in_past",
		"periodEnd": endRaw,
		"hint":      "This PENDING request's period has already elapsed; deny or delete it.",
	}
}

func timeOffRequestStatus(raw map[string]any) string {
	if status, ok := firstPresent(raw, "status", "state").(map[string]any); ok {
		return strings.ToUpper(firstReportString(status, "statusType", "status_type", "type", "state", "name"))
	}
	return strings.ToUpper(firstReportString(raw, "status", "state", "statusType", "status_type"))
}

func timeOffRequestViewsFromRaw(items []map[string]any) []TimeOffRequestView {
	out := make([]TimeOffRequestView, 0, len(items))
	for _, item := range items {
		out = append(out, timeOffRequestViewFromRaw(item))
	}
	return out
}

func timeOffBalanceViewFromRaw(raw map[string]any) TimeOffBalanceView {
	view := TimeOffBalanceView{}
	view["balance"] = map[string]any{
		"available": firstPresent(raw, "available", "availableBalance", "balance"),
		"used":      firstPresent(raw, "used", "usedBalance"),
		"accrued":   firstPresent(raw, "accrued", "accruedBalance"),
		"pending":   firstPresent(raw, "pending", "pendingBalance"),
		"remaining": firstPresent(raw, "remaining", "remainingBalance"),
		"unit":      firstReportString(raw, "timeUnit", "unit", "policyTimeUnit", "policy_time_unit"),
	}
	view["user"] = map[string]any{
		"id":   firstReportString(raw, "userId", "user_id"),
		"name": firstReportString(raw, "userName", "user_name"),
	}
	view["policy"] = map[string]any{
		"id":   firstReportString(raw, "policyId", "policy_id"),
		"name": firstReportString(raw, "policyName", "policy_name"),
	}
	return view
}

func timeOffActions(status string) []string {
	switch status {
	case "PENDING":
		return []string{"approve", "deny"}
	case "REJECTED", "DENIED":
		return []string{"review"}
	default:
		return nil
	}
}

func timeOffSuggestions(requestID, policyID, status string) []ToolSuggestion {
	if requestID == "" || policyID == "" || status != "PENDING" {
		return nil
	}
	return []ToolSuggestion{
		{Tool: "clockify_time_off_approve", Reason: "Approve this pending time-off request.", Arguments: map[string]any{"policy_id": policyID, "request_id": requestID}},
		{Tool: "clockify_time_off_deny", Reason: "Deny this pending time-off request with an optional note.", Arguments: map[string]any{"policy_id": policyID, "request_id": requestID}},
	}
}

func hasAnyKey(raw map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := raw[key]; ok && value != nil && reportValueString(value) != "" {
			return true
		}
	}
	return false
}
