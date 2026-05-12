package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/dryrun"
	"github.com/apet97/go-clockify/internal/mcp"
)

type documentedAPIOperation struct {
	Method string
	Path   string
}

func init() {
	registerTier2Group(Tier2Group{
		Name:        "probe_lab_api",
		Description: "Allowlisted OpenAPI/probe-lab routes not yet promoted to dedicated tools",
		Keywords:    []string{"openapi", "probe", "documented", "coverage", "raw", "api"},
		ToolNames: []string{
			"clockify_list_documented_api_operations",
			"clockify_call_documented_read_api",
			"clockify_call_documented_write_api",
			"clockify_call_documented_delete_api",
		},
		Builder: probeLabAPIHandlers,
	})
}

func probeLabAPIHandlers(s *Service) []mcp.ToolDescriptor {
	return []mcp.ToolDescriptor{
		{Tool: toolRO("clockify_list_documented_api_operations", "List the allowlisted method/path operations from the probe-lab OpenAPI and DOC source set.", documentedAPIListSchema()), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.ListDocumentedAPIOperations(ctx, args)
		}},
		{Tool: toolRO("clockify_call_documented_read_api", "Call an allowlisted documented GET endpoint. Use operation from clockify_list_documented_api_operations; path_params fill template variables.", documentedAPICallSchema([]string{http.MethodGet}, false)), ReadOnlyHint: true, IdempotentHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedReadAPI(ctx, args)
		}},
		{Tool: toolRW("clockify_call_documented_write_api", "Call an allowlisted documented POST, PUT, or PATCH endpoint. Supports json_body, form_body, query, raw_response, and dry_run.", documentedAPICallSchema([]string{http.MethodPost, http.MethodPut, http.MethodPatch}, true)), ReadOnlyHint: false, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedWriteAPI(ctx, args)
		}},
		{Tool: toolDestructive("clockify_call_documented_delete_api", "Call an allowlisted documented DELETE endpoint. Supports json_body, query, and dry_run.", documentedAPICallSchema([]string{http.MethodDelete}, true)), ReadOnlyHint: false, DestructiveHint: true, Handler: func(ctx context.Context, args map[string]any) (any, error) {
			return s.CallDocumentedDeleteAPI(ctx, args)
		}},
	}
}

func documentedAPIListSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method":   map[string]any{"type": "string", "enum": []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}},
			"contains": map[string]any{"type": "string", "description": "Case-insensitive substring filter against METHOD path."},
		},
	}
}

func documentedAPICallSchema(methods []string, allowDryRun bool) map[string]any {
	props := map[string]any{
		"operation":    map[string]any{"type": "string", "description": "Exact allowlisted operation string, e.g. GET /workspaces/{workspaceId}/users/{userId}/managers."},
		"method":       map[string]any{"type": "string", "enum": methods, "description": "Alternative to operation; must be paired with path."},
		"path":         map[string]any{"type": "string", "description": "Alternative to operation; documented path template, e.g. /workspaces/{workspaceId}/clients."},
		"workspace_id": map[string]any{"type": "string", "description": "Overrides the configured workspace_id when the path has {workspaceId}."},
		"path_params": map[string]any{
			"type":                 "object",
			"additionalProperties": map[string]any{"type": "string"},
			"description":          "Values for non-workspace path template variables, keyed by placeholder name.",
		},
		"query": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Query parameters. Array values are sent as repeated query keys.",
		},
		"json_body": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "JSON request body sent as-is after method/path allowlist validation.",
		},
		"form_body": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Multipart form fields. Array values are sent as repeated fields.",
		},
		"raw_response": map[string]any{"type": "boolean", "description": "Return base64 body and response headers instead of JSON-decoding."},
	}
	if allowDryRun {
		props["dry_run"] = map[string]any{"type": "boolean", "description": "Preview the resolved request without sending it upstream."}
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"anyOf": []any{
			map[string]any{"required": []string{"operation"}},
			map[string]any{"required": []string{"method", "path"}},
		},
	}
}

var documentedAPIOperations = []documentedAPIOperation{
	{Method: http.MethodPost, Path: "/file/image"},
	{Method: http.MethodGet, Path: "/shared-reports/{sharedReportId}"},
	{Method: http.MethodGet, Path: "/user"},
	{Method: http.MethodGet, Path: "/workspaces"},
	{Method: http.MethodPost, Path: "/workspaces"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/addons/{addonId}/webhooks"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/approval-requests"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/approval-requests"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/approval-requests/resubmit-entries-for-approval"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/approval-requests/users/{userId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/approval-requests/users/{userId}/resubmit-entries-for-approval"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/approval-requests/{approvalId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/balance"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/balance"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/clients"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/clients"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/clients/{clientId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/clients/{clientId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/clients/{clientId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/clients/{clientId}/archive"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/clients/{id}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/clients/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/clients/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/cost-rate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/custom-fields"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/custom-fields"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/custom-fields/{customFieldId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/custom-fields/{customFieldId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/expenses"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/expenses"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/expenses/categories"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/expenses/categories"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/expenses/categories/{categoryId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/expenses/categories/{categoryId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/expenses/categories/{categoryId}/status"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/expenses/{expenseId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/expenses/{expenseId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/expenses/{expenseId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/entities/created"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/entities/deleted"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/entities/updated"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/holidays"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/holidays"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/holidays/in-period"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/holidays/{holidayId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/holidays/{holidayId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/hourly-rate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/invoices"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices/info"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/invoices/settings"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/invoices/settings"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/duplicate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/export"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/items"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/items/import"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/items/{order}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/payments"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/payments"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/payments/{paymentId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/invoices/{invoiceId}/status"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/member-profile/{userId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/member-profile/{userId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/member-profile/{userId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/policies"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/policies"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/policies/{policyId}/requests"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/policies/{policyId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/policies/{policyId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/policies/{policyId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/policies/{policyId}/archive"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/projects"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/projects"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/projects/from-template"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/projects/{projectId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/projects/{projectId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/archive"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/cost-rate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/projects/{projectId}/custom-fields"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/projects/{projectId}/custom-fields/{customFieldId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/projects/{projectId}/estimate"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/hourly-rate"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/projects/{projectId}/memberships"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/projects/{projectId}/memberships"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/cost-rate"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{id}/hourly-rate"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/cost-rate"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}/hourly-rate"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/projects/{projectId}/template"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/cost-rate"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/projects/{projectId}/users/{userId}/hourly-rate"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/reports/attendance"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/reports/detailed"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/reports/expenses/detailed"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/reports/summary"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/reports/weekly"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/scheduling/assignments/all"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments/projects/totals"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/scheduling/assignments/projects/totals/{projectId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/scheduling/assignments/publish"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments/recurring"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/scheduling/assignments/recurring/{assignmentId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/scheduling/assignments/series/{assignmentId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments/user-filter/totals"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments/users/totals"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/scheduling/assignments/users/{userId}/totals"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy"},
	// NOTE: /scheduling/capacity is a phantom path — Clockify returns
	// 404 + {"code":3000,"message":"No static resource …"} for it. The
	// real per-user capacity endpoint is
	// /scheduling/assignments/users/{userId}/totals (covered above and
	// by clockify_filter_schedule_capacity). Quarantined in the OpenAPI
	// generator (scripts/gen-clockify-openapi PHANTOM_PATHS); do not
	// re-add to the allowlist without proving the route exists.
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/shared-reports"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/shared-reports"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/shared-reports/{sharedReportId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/shared-reports/{sharedReportId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/tags"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/tags"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/tags/{id}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/tags/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/tags/{id}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/tags/{tagId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/tags/{tagId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/tags/{tagId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-entries"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-entries/invoiced"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-entries/invoiced/bulk"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-entries/status/in-progress"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/time-entries/{id}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-entries/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/time-entries/{id}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/time-entries/{timeEntryId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-entries/{timeEntryId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/time-entries/{timeEntryId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/balance/user/{userId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/balance/policy/{policyId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-off/balance/policy/{policyId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/policies"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-off/policies"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/time-off/policies/{id}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/policies/{id}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-off/policies/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/time-off/policies/{id}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-off/policies/{policyId}/requests"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-off/policies/{policyId}/requests/{requestId}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-off/policies/{policyId}/users/{userId}/requests"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/requests"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-off/requests"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/time-off/requests/users/{userId}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/time-off/requests/{requestId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/time-off/requests/{requestId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/time-off/requests/{requestId}/status"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/user-groups"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/user-groups"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/user-groups/{groupId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/user-groups/{groupId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/user-groups/{groupId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/user-groups/{groupId}/users"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/user-groups/{groupId}/users"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/user-groups/{groupId}/users/{userId}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/user-groups/{id}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/user-groups/{id}"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/user-groups/{userGroupId}/users"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/user-groups/{userGroupId}/users/{userId}"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries/stop"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/user/{userId}/time-entries/{id}/duplicate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/users"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/users"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/users/info"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/users/{userId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/users/{userId}/cost-rate"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/users/{userId}/custom-field/{customFieldId}/value"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/users/{userId}/hourly-rate"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/users/{userId}/managers"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/users/{userId}/time-off/balances"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/users/{userId}/roles"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/users/{userId}/roles"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/webhooks"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/webhooks"},
	{Method: http.MethodDelete, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}"},
	{Method: http.MethodPut, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}/generateNewToken"},
	{Method: http.MethodPost, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}/logs"},
	{Method: http.MethodGet, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}/logs"},
	{Method: http.MethodPatch, Path: "/workspaces/{workspaceId}/webhooks/{webhookId}/token"},
}

func (s *Service) ListDocumentedAPIOperations(_ context.Context, args map[string]any) (ResultEnvelope, error) {
	methodFilter := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	contains := strings.ToLower(strings.TrimSpace(stringArg(args, "contains")))
	operations := make([]map[string]any, 0, len(documentedAPIOperations))
	for _, op := range documentedAPIOperations {
		if methodFilter != "" && op.Method != methodFilter {
			continue
		}
		key := op.key()
		if contains != "" && !strings.Contains(strings.ToLower(key), contains) {
			continue
		}
		operations = append(operations, map[string]any{
			"operation":    key,
			"method":       op.Method,
			"path":         op.Path,
			"reports_host": op.reportsHost(),
		})
	}
	return ok("clockify_list_documented_api_operations", operations, map[string]any{
		"count":      len(operations),
		"total":      len(documentedAPIOperations),
		"source":     "openapi-docs",
		"sourceRows": "realOPENAPI, AIII/openapi.yaml, clockify-api-probe-lab openapi.yaml/fragments, and all *DOC*.md files",
	}), nil
}

func (s *Service) CallDocumentedReadAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_read_api", map[string]bool{http.MethodGet: true})
}

func (s *Service) CallDocumentedWriteAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_write_api", map[string]bool{
		http.MethodPost:  true,
		http.MethodPut:   true,
		http.MethodPatch: true,
	})
}

func (s *Service) CallDocumentedDeleteAPI(ctx context.Context, args map[string]any) (ResultEnvelope, error) {
	return s.callDocumentedAPI(ctx, args, "clockify_call_documented_delete_api", map[string]bool{http.MethodDelete: true})
}

func (s *Service) callDocumentedAPI(ctx context.Context, args map[string]any, toolName string, allowed map[string]bool) (ResultEnvelope, error) {
	requested, err := documentedOperationFromArgs(args)
	if err != nil {
		return ResultEnvelope{}, err
	}
	if !allowed[requested.Method] {
		return ResultEnvelope{}, fmt.Errorf("%s does not allow %s operations", toolName, requested.Method)
	}
	if requested.Method != http.MethodGet && !s.DocumentedAPIWrites && !dryrun.Enabled(args) {
		return ResultEnvelope{}, fmt.Errorf("documented API write/delete calls are disabled; set CLOCKIFY_DOCUMENTED_API_WRITES=1 to enable this raw escape hatch")
	}
	op, found := documentedOperationByKey(requested.key())
	if !found {
		return ResultEnvelope{}, fmt.Errorf("operation %q is not in the documented Clockify allowlist", requested.key())
	}

	pathParams, err := documentedStringMap(args, "path_params")
	if err != nil {
		return ResultEnvelope{}, err
	}
	wsID := strings.TrimSpace(stringArg(args, "workspace_id"))
	resolvedPath, wsID, err := s.resolveDocumentedAPIPath(ctx, op.Path, wsID, pathParams)
	if err != nil {
		return ResultEnvelope{}, err
	}
	query, err := documentedValuesArg(args, "query")
	if err != nil {
		return ResultEnvelope{}, err
	}
	jsonBody, hasJSON := args["json_body"]
	formBody, hasForm, err := documentedFormValuesArg(args, "form_body")
	if err != nil {
		return ResultEnvelope{}, err
	}
	if hasJSON && hasForm {
		return ResultEnvelope{}, fmt.Errorf("json_body and form_body are mutually exclusive")
	}
	if hasJSON && jsonBody == nil {
		hasJSON = false
	}

	meta := map[string]any{
		"source":      "probe-lab-api",
		"operation":   op.key(),
		"method":      op.Method,
		"path":        resolvedPath,
		"reportsHost": op.reportsHost(),
	}
	if wsID != "" {
		meta["workspaceId"] = wsID
	}
	if dryrun.Enabled(args) && op.Method != http.MethodGet {
		preview := map[string]any{
			"method":       op.Method,
			"path":         resolvedPath,
			"query":        query,
			"reports_host": op.reportsHost(),
		}
		if hasJSON {
			preview["json_body"] = jsonBody
		}
		if hasForm {
			preview["form_body"] = formBody
		}
		return ok(toolName, dryrun.Preview(toolName, preview), meta), nil
	}

	if boolArg(args, "raw_response") {
		raw, err := s.Client.RequestRawValues(ctx, op.reportsHost(), op.Method, resolvedPath, query, documentedRequestBody(hasJSON, jsonBody))
		if err != nil {
			return ResultEnvelope{}, err
		}
		return ok(toolName, documentedRawResponse(raw.Header, raw.Body), meta), nil
	}

	var out any
	if hasForm {
		if err := s.Client.RequestMultipartValues(ctx, op.reportsHost(), op.Method, resolvedPath, query, formBody, &out); err != nil {
			return ResultEnvelope{}, err
		}
	} else if err := s.Client.RequestJSONValues(ctx, op.reportsHost(), op.Method, resolvedPath, query, documentedRequestBody(hasJSON, jsonBody), &out); err != nil {
		return ResultEnvelope{}, err
	}
	return ok(toolName, out, meta), nil
}

func documentedRequestBody(hasBody bool, body any) any {
	if !hasBody {
		return nil
	}
	return body
}

func documentedRawResponse(header http.Header, body []byte) map[string]any {
	return map[string]any{
		"contentType": header.Get("Content-Type"),
		"filename":    parseExportFilename(header.Get("Content-Disposition")),
		"bytes":       len(body),
		"body":        base64.StdEncoding.EncodeToString(body),
	}
}

func documentedOperationFromArgs(args map[string]any) (documentedAPIOperation, error) {
	if raw := strings.TrimSpace(stringArg(args, "operation")); raw != "" {
		method, path, ok := strings.Cut(raw, " ")
		if !ok {
			return documentedAPIOperation{}, fmt.Errorf("operation must be formatted as METHOD /path")
		}
		return documentedAPIOperation{
			Method: strings.ToUpper(strings.TrimSpace(method)),
			Path:   normalizeDocumentedAPIPath(strings.TrimSpace(path)),
		}, nil
	}
	method := strings.ToUpper(strings.TrimSpace(stringArg(args, "method")))
	path := normalizeDocumentedAPIPath(strings.TrimSpace(stringArg(args, "path")))
	if method == "" || path == "" {
		return documentedAPIOperation{}, fmt.Errorf("operation is required, or method and path must both be supplied")
	}
	return documentedAPIOperation{Method: method, Path: path}, nil
}

func normalizeDocumentedAPIPath(path string) string {
	if strings.HasPrefix(path, "/api/v1/") {
		path = strings.TrimPrefix(path, "/api/v1")
	} else if strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return canonicalDocumentedAPIPath(path)
}

func canonicalDocumentedAPIPath(path string) string {
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		previous := ""
		if i > 0 {
			previous = segments[i-1]
		}
		switch name {
		case "ws", "wsId", "workspace", "workspace_id", "workspaceId":
			name = "workspaceId"
		case "uid", "user", "user_id", "userId":
			name = "userId"
		default:
			name = canonicalDocumentedParam(previous, name)
		}
		segments[i] = "{" + name + "}"
	}
	return strings.Join(segments, "/")
}

func canonicalDocumentedParam(segment, fallback string) string {
	switch segment {
	case "addons":
		return "addonId"
	case "approval-requests":
		return "approvalId"
	case "assignments":
		return "assignmentId"
	case "categories":
		return "categoryId"
	case "clients":
		return "clientId"
	case "custom-fields":
		return "customFieldId"
	case "expenses":
		return "expenseId"
	case "files":
		return "fileId"
	case "holidays":
		return "holidayId"
	case "invoices":
		return "invoiceId"
	case "items":
		return "order"
	case "payments":
		return "paymentId"
	case "policies":
		return "policyId"
	case "projects":
		return "projectId"
	case "requests":
		return "requestId"
	case "shared-reports":
		return "sharedReportId"
	case "tags":
		return "tagId"
	case "tasks":
		return "taskId"
	case "time-entries":
		return "timeEntryId"
	case "user":
		return "userId"
	case "user-groups":
		return "groupId"
	case "users":
		return "userId"
	case "webhooks":
		return "webhookId"
	default:
		return fallback
	}
}

func documentedOperationByKey(key string) (documentedAPIOperation, bool) {
	for _, op := range documentedAPIOperations {
		if op.key() == key {
			return documentedAPIOperation{Method: op.Method, Path: normalizeDocumentedAPIPath(op.Path)}, true
		}
	}
	return documentedAPIOperation{}, false
}

func (op documentedAPIOperation) key() string {
	return op.Method + " " + normalizeDocumentedAPIPath(op.Path)
}

func (op documentedAPIOperation) reportsHost() bool {
	return strings.Contains(op.Path, "/reports/") || strings.Contains(op.Path, "/shared-reports")
}

func (s *Service) resolveDocumentedAPIPath(ctx context.Context, template, workspaceID string, pathParams map[string]string) (string, string, error) {
	out := template
	for {
		start := strings.Index(out, "{")
		if start < 0 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end < 0 {
			return "", workspaceID, fmt.Errorf("malformed documented path template %q", template)
		}
		end += start
		name := out[start+1 : end]
		value := ""
		if name == "workspaceId" {
			value = strings.TrimSpace(workspaceID)
			if value == "" {
				var err error
				value, err = s.ResolveWorkspaceID(ctx)
				if err != nil {
					return "", workspaceID, err
				}
				workspaceID = value
			}
		} else {
			value = strings.TrimSpace(pathParams[name])
		}
		if value == "" {
			return "", workspaceID, fmt.Errorf("path_params.%s is required for %s", name, template)
		}
		out = out[:start] + url.PathEscape(value) + out[end+1:]
	}
	return out, workspaceID, nil
}

func documentedStringMap(args map[string]any, key string) (map[string]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return map[string]string{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		text, err := documentedScalar(v)
		if err != nil {
			return nil, fmt.Errorf("%s.%s: %w", key, k, err)
		}
		out[k] = text
	}
	return out, nil
}

func documentedValuesArg(args map[string]any, key string) (url.Values, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return url.Values{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an object", key)
	}
	return documentedValuesFromMap(key, m)
}

func documentedFormValuesArg(args map[string]any, key string) (url.Values, bool, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("%s must be an object", key)
	}
	values, err := documentedValuesFromMap(key, m)
	if err != nil {
		return nil, true, err
	}
	return values, true, nil
}

func documentedValuesFromMap(parent string, m map[string]any) (url.Values, error) {
	values := url.Values{}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		switch typed := v.(type) {
		case []any:
			for i, item := range typed {
				text, err := documentedScalar(item)
				if err != nil {
					return nil, fmt.Errorf("%s.%s[%d]: %w", parent, k, i, err)
				}
				values.Add(k, text)
			}
		case []string:
			for _, item := range typed {
				values.Add(k, item)
			}
		default:
			text, err := documentedScalar(v)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", parent, k, err)
			}
			values.Add(k, text)
		}
	}
	return values, nil
}

func documentedScalar(v any) (string, error) {
	switch typed := v.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	default:
		return "", fmt.Errorf("must be string, number, or boolean")
	}
}
