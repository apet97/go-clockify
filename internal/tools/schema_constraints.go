package tools

import "strings"

// tightenInputSchema mutates a JSON schema tree in place to meet the MCP
// spec requirements for every tool descriptor:
//   - every object schema gets `additionalProperties: false` unless explicitly set
//   - `page` and `page_size` integer properties gain `minimum`/`maximum` bounds
//   - string properties whose description mentions RFC3339 gain
//     `format: "date-time"`, UNLESS the description also documents a
//     flexible parser (e.g. "natural language" or "YYYY-MM-DD"). The
//     validator at internal/jsonschema enforces format: date-time via
//     strict time.Parse(time.RFC3339, ...), so adding the format to a
//     field whose handler accepts wider input would reject valid calls
//     before the handler ever runs.
//   - `color` properties whose description mentions Hex gain the 6-hex pattern
//
// The walker handles nested objects, arrays (via `items`), and anyOf
// subschemas. It never
// overwrites an explicit value — callers can opt out of any single rule
// by setting it themselves.
func tightenInputSchema(schema map[string]any) {
	if schema == nil {
		return
	}
	if typ, _ := schema["type"].(string); typ == "object" {
		if _, set := schema["additionalProperties"]; !set {
			schema["additionalProperties"] = false
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			for name, raw := range props {
				prop, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				applyPropertyConstraints(name, prop)
				tightenInputSchema(prop)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		tightenInputSchema(items)
	}
	if options, ok := schema["anyOf"].([]any); ok {
		for _, option := range options {
			if sub, ok := option.(map[string]any); ok {
				tightenInputSchema(sub)
			}
		}
	}
}

// applyPropertyConstraints adds spec-driven constraints to a single
// property schema based on its name and description. Only untouched keys
// are added — explicit values stay as declared.
func applyPropertyConstraints(name string, prop map[string]any) {
	// Fill a canonical description for any parameter that lacks one. An
	// explicit (non-empty) per-descriptor description is left untouched.
	if canonical, ok := paramDescriptions[name]; ok {
		if existing, _ := prop["description"].(string); strings.TrimSpace(existing) == "" {
			prop["description"] = canonical
		} else if len(strings.TrimSpace(existing)) < 12 {
			prop["description"] = canonical
		}
	}

	switch name {
	case "page":
		if _, set := prop["minimum"]; !set {
			prop["minimum"] = 1
		}
		if _, set := prop["description"]; !set {
			prop["description"] = "Page number, starting at 1."
		}
	case "page_size":
		if _, set := prop["minimum"]; !set {
			prop["minimum"] = 1
		}
		if _, set := prop["maximum"]; !set {
			prop["maximum"] = 200
		}
		prop["description"] = "Items per page. Default 50, maximum 200."
	case "dry_run":
		prop["description"] = "Preview the resolved request without making changes."
	case "color":
		if desc, _ := prop["description"].(string); strings.Contains(strings.ToLower(desc), "hex") {
			if _, set := prop["pattern"]; !set {
				prop["pattern"] = "^#[0-9a-fA-F]{6}$"
			}
		}
	case "currency":
		// Invoice currency is an ISO 4217 three-letter code (USD, EUR, ...).
		// Bounding it centrally rejects malformed values at JSON-schema
		// validation before the handler reaches the live invoice API.
		if typ, _ := prop["type"].(string); typ == "string" {
			if _, set := prop["minLength"]; !set {
				prop["minLength"] = 3
			}
			if _, set := prop["maxLength"]; !set {
				prop["maxLength"] = 3
			}
			if _, set := prop["pattern"]; !set {
				prop["pattern"] = "^[A-Za-z]{3}$"
			}
		}
	}
	// Generic RFC3339 timestamp detection — any string property whose
	// description calls out an RFC3339 timestamp gains format: date-time,
	// unless the description ALSO documents a flexible parser (natural
	// language like "now"/"today" or a YYYY-MM-DD short date). The
	// jsonschema validator enforces format: date-time via strict
	// time.Parse(time.RFC3339, ...) before the handler runs, so adding
	// the format to a flexible-parsing field would reject valid input
	// like start="now" on clockify_add_entry. Handlers using
	// timeparse.ParseDatetime / parseFlexibleDateTime accept the wider
	// surface; the schema must not be tighter than the parser.
	if typ, _ := prop["type"].(string); typ == "string" {
		desc, _ := prop["description"].(string)
		if desc != "" && strings.Contains(desc, "RFC3339") && !descriptionAdvertisesFlexibleTime(desc) {
			if _, set := prop["format"]; !set {
				prop["format"] = "date-time"
			}
		}
	}
	// Generic maxLength bounds on common free-text fields. Centralised
	// here so every descriptor inherits the same ceiling without each
	// handler hand-declaring it. Bounds chosen from observed
	// Clockify-API limits and RFC defaults; an explicit handler-side
	// maxLength always wins.
	//
	// Skipped on purpose:
	//   - project/client/tag lookup identifiers (Clockify accepts UUIDs);
	//   - flexible-time string fields (handled separately above).
	if ceil, ok := freeTextMaxLength[name]; ok {
		if typ, _ := prop["type"].(string); typ == "string" {
			if _, set := prop["maxLength"]; !set {
				prop["maxLength"] = ceil
			}
		}
	}
}

// freeTextMaxLength is the central table of conservative ceilings on
// common free-text property names. The values must NEVER be relaxed in
// place — TestRegistryFreeTextFieldsHaveMaxLength enforces them, and a
// future reviewer who needs a higher ceiling on a specific tool should
// declare maxLength explicitly on that tool's descriptor (which is
// honoured here because applyPropertyConstraints never overwrites an
// existing key).
var freeTextMaxLength = map[string]int{
	"description":          2000, // entry/log descriptions etc.
	"description_contains": 2000, // filter form of description
	"exact_description":    2000, // exact-match variant
	"new_description":      2000, // update form
	"name":                 150,
	"note":                 500,
	"notes":                2000, // expense free-form notes
	"number":               150,  // invoice number
	"path":                 2048, // raw API fallback path
	"address":              256,
	"email":                254, // RFC 5321 max localpart+domain
	"url":                  2048,
	"webhook_url":          2048,
	"redirect_url":         2048,
}

// paramDescriptions is the canonical description for every tool input
// parameter name in the Clockify domain. applyPropertyConstraints fills it
// into any property that has no description of its own, so descriptors
// need not repeat boilerplate and new tools inherit documentation for
// free. An explicit per-descriptor description always wins.
var paramDescriptions = map[string]string{
	// Entity identifiers.
	"project_id":    "Clockify project ID. When a sibling project field is required, this ID-only alias is accepted in place of project.",
	"task_id":       "Clockify task ID. When a sibling task field is required, this ID-only alias is accepted in place of task.",
	"entry_id":      "Clockify time entry ID.",
	"client_id":     "Clockify client ID. When a sibling client field is required, this ID-only alias is accepted in place of client.",
	"invoice_id":    "Clockify invoice ID.",
	"expense_id":    "Clockify expense ID.",
	"category_id":   "Clockify expense category ID. Accepted in place of category when a category reference is required.",
	"field_id":      "Clockify custom field ID.",
	"policy_id":     "Clockify time-off policy ID. Accepted in place of policy when a policy reference is required.",
	"request_id":    "Clockify time-off request ID.",
	"holiday_id":    "Clockify holiday ID.",
	"group_id":      "Clockify user group ID.",
	"tag_id":        "Clockify tag ID. When a sibling tag field is required, this ID-only alias is accepted in place of tag.",
	"user_id":       "Clockify user ID. When a sibling user field is required, this ID-only alias is accepted in place of user.",
	"webhook_id":    "Clockify webhook ID.",
	"approval_id":   "Clockify approval request ID.",
	"assignment_id": "Clockify scheduling assignment ID.",
	"payment_id":    "Clockify invoice payment ID.",
	"currency_id":   "Clockify currency ID.",
	// Name-or-ID references.
	"project": "Project name or ID. If you already have the ID, pass it here or use project_id when exposed.",
	"task":    "Task name or ID. If you already have the ID, pass it here or use task_id when exposed.",
	"tag":     "Tag name or ID. If you already have the ID, pass it here or use tag_id when exposed.",
	// Identifier and value collections.
	"tag_ids":        "Clockify tag IDs.",
	"tags":           "Tag names or IDs.",
	"time_entry_ids": "Time entry IDs.",
	"excluded_ids":   "IDs to exclude from the results.",
	"assignee_ids":   "User IDs to assign.",
	"emails":         "Email addresses.",
	"cc_emails":      "CC email addresses for the invoice.",
	"clients":        "Client IDs to filter by.",
	"users":          "User IDs to filter by.",
	"user_groups":    "User group IDs.",
	"allowed_values": "Allowed values for a dropdown custom field.",
	"memberships":    "Full membership list to set on the project.",
	// Dates and times.
	"start":      flexibleDatetimeDescription,
	"end":        flexibleDatetimeDescription,
	"since":      "Effective date of the rate (YYYY-MM-DD).",
	"start_time": "Start time of day in HH:MM format.",
	// Boolean flags.
	"billable":                 "Whether the work is billable.",
	"archived":                 "Whether the item is archived.",
	"is_public":                "Whether the project is visible to the whole workspace.",
	"is_template":              "Filter to template projects only.",
	"is_active":                "Filter to active items only.",
	"include_entries":          "Include the underlying time entries in the response.",
	"include_non_working_days": "Include non-working days when distributing scheduled hours.",
	"include_roles":            "Include each user's role assignments in the response.",
	"strict_name_search":       "When true, match the name exactly instead of as a substring.",
	"contains_assignee":        "Filter to tasks that have an assignee.",
	"contains_client":          "Filter to projects that have a client.",
	"contains_user":            "Filter to projects assigned to a given user.",
	"contains_group":           "Filter to projects assigned to a given user group.",
	"half_day":                 "Whether the time-off request covers a half day.",
	"repeat":                   "Whether the scheduled assignment repeats.",
	"hydrated":                 "Return the fully hydrated record, including nested related objects.",
	"in_progress":              "Filter to time entries that are currently running.",
	"project_required":         "Return only entries that have a project.",
	"task_required":            "Return only entries that have a task.",
	"get_week_before":          "Also include the week before the requested range.",
	"archive_projects":         "Also archive the client's projects.",
	"mark_tasks_as_done":       "Mark the client's tasks as done.",
	"requires_approval":        "Whether time-off requests under this policy require approval.",
	"auto_approve":             "Whether matching requests are approved automatically.",
	"accrual":                  "Whether the policy accrues balance over time.",
	"negative_balance":         "Whether the policy allows a negative balance.",
	"invoiced":                 "Whether the time entries are marked as invoiced.",
	// Text fields.
	"name":                 "Display name.",
	"note":                 "Optional note.",
	"notes":                "Optional notes.",
	"description":          "Free-text description.",
	"description_contains": "Match items whose description contains this text.",
	"exact_description":    "Match items whose description exactly equals this text.",
	"new_description":      "Replacement description.",
	"email":                "Email address.",
	"address":              "Postal address.",
	"url":                  "Callback or request URL.",
	"color":                "Hex color code, e.g. #4caf50.",
	"number":               "Invoice number.",
	"currency":             "Three-letter currency code, e.g. USD.",
	// Enums and filters.
	"status":                   "Status value.",
	"sort_column":              "Field to sort the results by.",
	"sort_order":               "Sort direction: ASCENDING or DESCENDING.",
	"rate_kind":                "Rate type: hourly or cost.",
	"access":                   "Project access level: PUBLIC or PRIVATE.",
	"client_status":            "Filter projects by their client's status.",
	"user_status":              "Filter projects by member status.",
	"membership_status":        "Filter by membership status.",
	"account_statuses":         "Filter users by account status.",
	"time_unit":                "Time unit for the policy: DAYS or HOURS.",
	"time_entry_group_type":    "How imported time entries are grouped: SINGLE_ITEM, GROUPED, or DETAILED.",
	"custom_field_entity_type": "Entity type the custom field value applies to.",
	"estimate_reset":           "How the estimate resets over time.",
	"expense_limit":            "Clockify expense-limit query filter.",
	"attendance_filter":        "Clockify attendance-report filter object.",
	"detailed_filter":          "Clockify detailed-report filter object.",
	"summary_filter":           "Clockify summary-report filter object.",
	// Numbers.
	"amount":          "Amount in Clockify's expected unit.",
	"quantity":        "Quantity; must be greater than 0.",
	"min_gap_minutes": "Minimum gap, in minutes, to flag between time entries.",
	"days_per_year":   "Days granted per year by the policy.",
	"budget_estimate": "Budget estimate amount.",
	"time_estimate":   "Time estimate, e.g. an ISO-8601 duration.",
	// Miscellaneous.
	"timezone":            "IANA timezone name; defaults to the configured or local timezone.",
	"trigger_source":      "Webhook trigger source IDs.",
	"trigger_source_type": "Webhook trigger source type.",
	"required":            "Whether the custom field is required.",
	// Raw API fallback.
	"body":   "Raw JSON request body forwarded to the Clockify API.",
	"path":   "Clockify API path, e.g. /workspaces/{workspaceId}/projects.",
	"query":  "Query-string parameters as a key/value object.",
	"method": "HTTP method: GET, POST, PUT, PATCH, or DELETE.",
}

// descriptionAdvertisesFlexibleTime reports whether a property's
// description tells callers they can pass non-RFC3339 input. Handlers
// that document such flexibility use timeparse.ParseDatetime or
// parseFlexibleDateTime; the jsonschema validator must skip its
// format: date-time enforcement for these fields so the schema gate
// does not reject valid input the handler would accept.
func descriptionAdvertisesFlexibleTime(desc string) bool {
	lower := strings.ToLower(desc)
	if strings.Contains(lower, "natural language") {
		return true
	}
	// Match the literal token "YYYY-MM-DD" (case-insensitive); handlers
	// like clockify_reports_weekly's week_start parse it via
	// parseFlexibleDateTime.
	if strings.Contains(lower, "yyyy-mm-dd") {
		return true
	}
	return false
}

func requiredSchema(field string) map[string]any {
	return map[string]any{"type": "object", "required": []string{field}, "properties": map[string]any{field: map[string]any{"type": "string"}}}
}
