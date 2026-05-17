package tools

import (
	"maps"
	"sort"
	"strings"
)

type InvoiceView map[string]any
type CompactInvoiceView struct {
	ID         string     `json:"id"`
	Number     string     `json:"number,omitempty"`
	Status     string     `json:"status,omitempty"`
	ClientID   string     `json:"clientId,omitempty"`
	ClientName string     `json:"clientName,omitempty"`
	IssuedDate string     `json:"issuedDate,omitempty"`
	DueDate    string     `json:"dueDate,omitempty"`
	Currency   string     `json:"currency,omitempty"`
	Amount     *MoneyView `json:"amount,omitempty"`
}

type InvoiceItemView map[string]any
type InvoicePaymentView map[string]any
type InvoiceSettingsView map[string]any

type InvoiceReportView struct {
	Invoices         []InvoiceView  `json:"invoices"`
	AggregationScope string         `json:"aggregationScope"`
	PageTotalAmount  *MoneyView     `json:"pageTotalAmount,omitempty"`
	PageStatusCounts map[string]int `json:"pageStatusCounts,omitempty"`
	TotalAmount      *MoneyView     `json:"totalAmount,omitempty"`
	StatusCounts     map[string]int `json:"statusCounts,omitempty"`
	Summary          InvoiceSummary `json:"summary"`
	Raw              map[string]any `json:"raw,omitempty"`
}

type InvoiceSummary struct {
	Count                 int            `json:"count"`
	ByStatus              map[string]int `json:"by_status,omitempty"`
	OverdueCount          int            `json:"overdue_count,omitempty"`
	ImportedTimeCount     int            `json:"imported_time_count,omitempty"`
	ImportedExpenseCount  int            `json:"imported_expense_count,omitempty"`
	Amount                *MoneyView     `json:"amount,omitempty"`
	Paid                  *MoneyView     `json:"paid,omitempty"`
	Balance               *MoneyView     `json:"balance,omitempty"`
	Currencies            []string       `json:"currencies,omitempty"`
	Source                string         `json:"source"`
	Reason                string         `json:"reason,omitempty"`
	SuggestedActionsCount int            `json:"suggested_actions_count,omitempty"`
}

func compactInvoiceViewFromRaw(raw map[string]any) CompactInvoiceView {
	currency := reportCurrency(raw)
	return CompactInvoiceView{
		ID:         firstReportString(raw, "id", "_id", "invoiceId", "invoice_id"),
		Number:     firstReportString(raw, "number", "invoiceNumber", "invoice_number"),
		Status:     strings.ToUpper(firstReportString(raw, "status", "invoiceStatus", "invoice_status")),
		ClientID:   firstReportString(raw, "clientId", "client_id"),
		ClientName: firstReportString(raw, "clientName", "client_name"),
		IssuedDate: firstReportString(raw, "issuedDate", "issued_date"),
		DueDate:    firstReportString(raw, "dueDate", "due_date"),
		Currency:   currency,
		Amount:     moneyFromAny(firstPresent(raw, "amount", "totalAmount", "balance"), currency),
	}
}

func compactInvoiceViewsFromRaw(items []map[string]any) []CompactInvoiceView {
	out := make([]CompactInvoiceView, 0, len(items))
	for _, item := range items {
		out = append(out, compactInvoiceViewFromRaw(item))
	}
	return out
}

func invoiceViewFromRaw(raw map[string]any) InvoiceView {
	view := InvoiceView(maps.Clone(raw))
	id := firstReportString(raw, "id", "_id", "invoiceId", "invoice_id")
	status := strings.ToUpper(firstReportString(raw, "status", "invoiceStatus", "invoice_status"))
	currency := reportCurrency(raw)
	view["invoice"] = map[string]any{
		"id":            id,
		"number":        firstReportString(raw, "number", "invoiceNumber", "invoice_number"),
		"status":        status,
		"issued_date":   firstReportString(raw, "issuedDate", "issued_date"),
		"due_date":      firstReportString(raw, "dueDate", "due_date"),
		"days_overdue":  intValue(firstPresent(raw, "daysOverdue", "days_overdue")),
		"bill_from":     firstReportString(raw, "billFrom", "bill_from"),
		"visible_zero":  firstPresent(raw, "visibleZeroFields", "visible_zero_fields"),
		"currency":      currency,
		"imported_time": boolFromAny(firstPresent(raw, "containsImportedTimes", "contains_imported_times")),
		"imported_expense": boolFromAny(firstPresent(raw,
			"containsImportedExpenses", "contains_imported_expenses")),
	}
	view["client"] = map[string]any{
		"id":   firstReportString(raw, "clientId", "client_id"),
		"name": firstReportString(raw, "clientName", "client_name"),
	}
	view["financials"] = map[string]any{
		"subtotal":        moneyFromAny(firstPresent(raw, "subtotal", "subTotal"), currency),
		"discount_amount": moneyFromAny(firstPresent(raw, "discountAmount", "discount_amount"), currency),
		"tax_amount":      moneyFromAny(firstPresent(raw, "taxAmount", "tax_amount"), currency),
		"tax2_amount":     moneyFromAny(firstPresent(raw, "tax2Amount", "tax2_amount"), currency),
		"amount":          moneyFromAny(firstPresent(raw, "amount", "totalAmount"), currency),
		"paid":            moneyFromAny(firstPresent(raw, "paid", "paidAmount"), currency),
		"balance":         moneyFromAny(firstPresent(raw, "balance", "balanceAmount"), currency),
		"source":          "invoice_api",
	}
	view["billing"] = map[string]any{
		"subject":        firstReportString(raw, "subject"),
		"note":           firstReportString(raw, "note"),
		"client_address": firstReportString(raw, "clientAddress", "client_address"),
		"company_id":     firstReportString(raw, "companyId", "company_id"),
		"bill_from":      firstReportString(raw, "billFrom", "bill_from"),
		"visible_zero_fields": firstPresent(raw,
			"visibleZeroFields", "visible_zero_fields"),
	}
	items := invoiceItemViews(mapSlice(raw["items"]), currency)
	if len(items) > 0 {
		view["items"] = items
		view["item_summary"] = invoiceItemSummary(items)
	}
	view["taxes"] = map[string]any{
		"tax":         firstPresent(raw, "tax"),
		"tax2":        firstPresent(raw, "tax2"),
		"tax_percent": firstPresent(raw, "taxPercent", "tax_percent"),
		"tax2_percent": firstPresent(raw,
			"tax2Percent", "tax2_percent"),
		"tax_type": firstReportString(raw, "taxType", "tax_type"),
	}
	view["discount"] = firstPresent(raw, "discount")
	payments := invoicePaymentViewsFromRaw(mapSlice(firstPresent(raw, "payments", "invoicePayments")))
	if len(payments) > 0 {
		view["payments"] = payments
	}
	view["payment_summary"] = invoicePaymentSummary(payments, currency)
	view["suggestedActions"] = invoiceSuggestions(id, status, moneyFromAny(firstPresent(raw, "balance"), currency))
	return view
}

func invoiceViewsFromRaw(items []map[string]any) []InvoiceView {
	out := make([]InvoiceView, 0, len(items))
	for _, item := range items {
		out = append(out, invoiceViewFromRaw(item))
	}
	return out
}

func invoiceItemViewFromRaw(raw map[string]any, fallbackCurrency string) InvoiceItemView {
	view := InvoiceItemView(maps.Clone(raw))
	currency := firstNonEmptyString(reportCurrency(raw), fallbackCurrency)
	view["id"] = firstReportString(raw, "id", "_id", "itemId", "item_id")
	view["invoiceItemId"] = firstReportString(raw, "id", "_id", "itemId", "item_id")
	view["order"] = firstPresent(raw, "order", "index", "itemIndex")
	view["type"] = firstReportString(raw, "itemType", "item_type", "type")
	view["import_type"] = firstReportString(raw, "importType", "import_type")
	view["description"] = firstReportString(raw, "description", "name")
	view["quantity"] = firstPresent(raw, "quantity")
	view["unit_price"] = moneyFromAny(firstPresent(raw, "unitPrice", "unit_price"), currency)
	view["amount"] = moneyFromAny(firstPresent(raw, "amount", "total"), currency)
	view["time_entry_ids"] = stringSliceFromAny(firstPresent(raw, "timeEntryIds", "time_entry_ids"))
	view["expense_ids"] = stringSliceFromAny(firstPresent(raw, "expenseIds", "expense_ids"))
	view["taxes"] = map[string]any{
		"apply_taxes": firstReportString(raw, "applyTaxes", "apply_taxes"),
		"tax":         firstPresent(raw, "tax"),
		"tax2":        firstPresent(raw, "tax2"),
	}
	return view
}

func invoiceItemViews(items []map[string]any, fallbackCurrency string) []InvoiceItemView {
	out := make([]InvoiceItemView, 0, len(items))
	for _, item := range items {
		out = append(out, invoiceItemViewFromRaw(item, fallbackCurrency))
	}
	return out
}

func invoicePaymentViewFromRaw(raw map[string]any, fallbackCurrency string) InvoicePaymentView {
	view := InvoicePaymentView(maps.Clone(raw))
	currency := firstNonEmptyString(reportCurrency(raw), fallbackCurrency)
	view["payment"] = map[string]any{
		"id":       firstReportString(raw, "id", "_id", "paymentId", "payment_id"),
		"amount":   moneyFromAny(firstPresent(raw, "amount", "value", "total"), currency),
		"date":     firstReportString(raw, "date", "paymentDate", "payment_date"),
		"note":     firstReportString(raw, "note"),
		"currency": currency,
		"source":   "invoice_payments_api",
	}
	if author, ok := raw["author"].(map[string]any); ok {
		view["author"] = map[string]any{
			"id":    firstReportString(author, "id", "_id", "userId"),
			"name":  firstReportString(author, "name"),
			"email": firstReportString(author, "email"),
			"raw":   maps.Clone(author),
		}
	}
	view["raw"] = maps.Clone(raw)
	return view
}

func invoicePaymentViewsFromRaw(items []map[string]any) []InvoicePaymentView {
	out := make([]InvoicePaymentView, 0, len(items))
	for _, item := range items {
		out = append(out, invoicePaymentViewFromRaw(item, reportCurrency(item)))
	}
	return out
}

func invoicePaymentSummary(payments []InvoicePaymentView, fallbackCurrency string) map[string]any {
	summary := map[string]any{"count": len(payments), "source": "invoice_payments_api"}
	var total *MoneyView
	for _, payment := range payments {
		if block, ok := payment["payment"].(map[string]any); ok {
			total = moneySum(total, moneyFromAny(block["amount"], fallbackCurrency))
		}
	}
	if total != nil {
		summary["amount"] = total
	}
	if len(payments) == 0 {
		summary["source"] = entryFinancialSourceUnavailable
		summary["reason"] = "no invoice payment rows were present"
	}
	return summary
}

func invoiceItemSummary(items []InvoiceItemView) map[string]any {
	var amount *MoneyView
	types := map[string]int{}
	for _, item := range items {
		if money, ok := item["amount"].(*MoneyView); ok {
			amount = moneySum(amount, money)
		}
		if typ := strings.ToUpper(reportValueString(item["type"])); typ != "" {
			types[typ]++
		}
	}
	return map[string]any{"count": len(items), "amount": amount, "types": types}
}

func invoiceSettingsViewFromRaw(raw map[string]any) InvoiceSettingsView {
	view := InvoiceSettingsView(maps.Clone(raw))
	view["labels"] = firstPresent(raw, "labels")
	view["defaults"] = firstPresent(raw, "defaults")
	view["export_fields"] = firstPresent(raw, "exportFields", "export_fields")
	view["settings_summary"] = map[string]any{
		"bill_from_label": firstNestedString(raw, "labels", "billFrom"),
		"bill_to_label":   firstNestedString(raw, "labels", "billTo"),
		"subject_default": firstNestedString(raw, "defaults", "subject"),
		"notes_default":   firstNestedString(raw, "defaults", "notes"),
		"tax_type":        firstNestedString(raw, "defaults", "taxType"),
		"source":          "invoice_settings_api",
	}
	view["raw"] = maps.Clone(raw)
	return view
}

func invoiceSummaryFromViews(views []InvoiceView) InvoiceSummary {
	summary := InvoiceSummary{Count: len(views), ByStatus: map[string]int{}, Source: "invoice_api"}
	currencies := map[string]struct{}{}
	for _, view := range views {
		status := ""
		if block, ok := view["invoice"].(map[string]any); ok {
			status = strings.ToUpper(reportValueString(block["status"]))
			if days, ok := reportInt(block["days_overdue"]); ok && days > 0 {
				summary.OverdueCount++
			}
			if boolFromAny(block["imported_time"]) {
				summary.ImportedTimeCount++
			}
			if boolFromAny(block["imported_expense"]) {
				summary.ImportedExpenseCount++
			}
			if currency := reportValueString(block["currency"]); currency != "" {
				currencies[currency] = struct{}{}
			}
		}
		if status != "" {
			summary.ByStatus[status]++
		}
		if fin, ok := view["financials"].(map[string]any); ok {
			summary.Amount = moneySum(summary.Amount, moneyFromAny(fin["amount"], ""))
			summary.Paid = moneySum(summary.Paid, moneyFromAny(fin["paid"], ""))
			summary.Balance = moneySum(summary.Balance, moneyFromAny(fin["balance"], ""))
		}
		if suggestions, ok := view["suggestedActions"].([]ToolSuggestion); ok {
			summary.SuggestedActionsCount += len(suggestions)
		}
	}
	if len(summary.ByStatus) == 0 {
		summary.ByStatus = nil
	}
	for currency := range currencies {
		summary.Currencies = append(summary.Currencies, currency)
	}
	sort.Strings(summary.Currencies)
	return summary
}

func invoiceSuggestions(invoiceID, status string, balance *MoneyView) []ToolSuggestion {
	if invoiceID == "" {
		return nil
	}
	var out []ToolSuggestion
	switch status {
	case "UNSENT":
		out = append(out, ToolSuggestion{Tool: "clockify_invoices_send", Reason: "Send this unsent invoice to the client.", Arguments: map[string]any{"invoice_id": invoiceID}})
	case "SENT", "OVERDUE", "PARTIALLY_PAID":
		if balance == nil || balance.AmountCents != 0 {
			out = append(out, ToolSuggestion{Tool: "clockify_invoices_mark_paid", Reason: "Mark this invoice paid after confirming payment.", Arguments: map[string]any{"invoice_id": invoiceID, "dry_run": true}})
		}
	}
	out = append(out, ToolSuggestion{Tool: "clockify_invoices_get", Reason: "Inspect invoice line items and raw billing fields.", Arguments: map[string]any{"invoice_id": invoiceID}})
	return out
}

func intValue(v any) int {
	n, _ := reportInt(v)
	return n
}

func firstNestedString(raw map[string]any, key, nestedKey string) string {
	if nested, ok := raw[key].(map[string]any); ok {
		return firstReportString(nested, nestedKey)
	}
	return ""
}

func stringSliceFromAny(value any) []string {
	switch items := value.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s := reportValueString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := reportValueString(value); s != "" {
			return []string{s}
		}
	}
	return nil
}
