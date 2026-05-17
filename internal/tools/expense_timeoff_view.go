package tools

import (
	"maps"
	"strings"
)

type ExpenseView map[string]any
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

type TimeOffPolicyView map[string]any
type CompactTimeOffPolicyView struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Archived       bool   `json:"archived,omitempty"`
	TimeUnit       string `json:"timeUnit,omitempty"`
	UserCount      int    `json:"userCount"`
	UserGroupCount int    `json:"userGroupCount"`
	AssigneeCount  int    `json:"assigneeCount"`
}

type TimeOffRequestView map[string]any
type TimeOffBalanceView map[string]any

func compactExpenseViewFromRaw(raw map[string]any) CompactExpenseView {
	currency := reportCurrency(raw)
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

func compactExpenseViewsFromRaw(items []map[string]any) []CompactExpenseView {
	out := make([]CompactExpenseView, 0, len(items))
	for _, item := range items {
		out = append(out, compactExpenseViewFromRaw(item))
	}
	return out
}

func expenseViewFromRaw(raw map[string]any) ExpenseView {
	view := ExpenseView(maps.Clone(raw))
	currency := reportCurrency(raw)
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
	view["policy"] = map[string]any{
		"id":                firstReportString(raw, "id", "_id", "policyId", "policy_id"),
		"name":              firstReportString(raw, "name"),
		"status":            strings.ToUpper(firstReportString(raw, "status")),
		"time_unit":         firstReportString(raw, "timeUnit", "time_unit"),
		"accrual":           firstPresent(raw, "accrual"),
		"auto_approve":      firstPresent(raw, "autoApprove", "auto_approve"),
		"requires_approval": firstPresent(raw, "requiresApproval", "requires_approval"),
		"negative_balance":  firstPresent(raw, "negativeBalance", "negative_balance"),
	}
	view["balance_rules"] = map[string]any{
		"days_per_year": firstPresent(raw, "daysPerYear", "days_per_year"),
		"hours_per_year": firstPresent(raw,
			"hoursPerYear", "hours_per_year"),
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
	view["user"] = map[string]any{
		"id":    userID,
		"name":  firstReportString(raw, "userName", "user_name"),
		"email": firstReportString(raw, "userEmail", "user_email"),
	}
	view["policy"] = map[string]any{
		"id":   policyID,
		"name": firstReportString(raw, "policyName", "policy_name"),
	}
	view["period"] = firstPresent(raw, "timeOffPeriod", "time_off_period", "period")
	view["day_period"] = firstPresent(raw, "dayPeriod", "day_period", "halfDay", "half_day")
	view["duration"] = map[string]any{
		"days":  firstPresent(raw, "days", "durationDays", "balanceDiff"),
		"hours": firstPresent(raw, "hours", "durationHours"),
	}
	view["actions"] = timeOffActions(status)
	view["suggestedActions"] = timeOffSuggestions(requestID, policyID, status)
	return view
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
