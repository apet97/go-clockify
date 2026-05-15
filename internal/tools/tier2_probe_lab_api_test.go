package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestProbeLabAPIListsDocOnlyAndOpenAPIOperations(t *testing.T) {
	svc := New(nil, "ws1")
	result, err := svc.ListDocumentedAPIOperations(context.Background(), map[string]any{"contains": "invoices"})
	if err != nil {
		t.Fatalf("list documented operations failed: %v", err)
	}
	ops, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("unexpected operation list type: %T", result.Data)
	}
	have := map[string]bool{}
	metadataChecked := false
	for _, op := range ops {
		operation := op["operation"].(string)
		have[operation] = true
		if operation == "POST /workspaces/{workspaceId}/invoices/{invoiceId}/items/import" {
			if op["operation_id"] == "" || op["live_status"] == "" {
				t.Fatalf("documented operation metadata missing canonical id/status: %+v", op)
			}
			if risk, ok := op["risk_class"].([]string); !ok || len(risk) == 0 {
				t.Fatalf("documented operation metadata missing risk class: %+v", op)
			}
			if sources, ok := op["source_files"].([]string); !ok || len(sources) == 0 {
				t.Fatalf("documented operation metadata missing source files: %+v", op)
			}
			metadataChecked = true
		}
	}
	for _, want := range []string{
		"GET /workspaces/{workspaceId}/invoices/settings",
		"POST /workspaces/{workspaceId}/invoices/{invoiceId}/duplicate",
		"POST /workspaces/{workspaceId}/invoices/{invoiceId}/items/import",
		"DELETE /workspaces/{workspaceId}/invoices/{invoiceId}/payments/{paymentId}",
	} {
		if !have[want] {
			t.Fatalf("documented operation list missing %s", want)
		}
	}
	if total, _ := result.Meta["total"].(int); total < 190 {
		t.Fatalf("expected combined probe-lab allowlist to cover at least 190 operations, got meta=%+v", result.Meta)
	}
	if !metadataChecked {
		t.Fatal("did not check generated documented operation metadata")
	}
}

func TestProbeLabAPINormalizesDocumentedPathAliases(t *testing.T) {
	cases := map[string]string{
		"/api/v1/workspaces/{wsId}/projects/{projectId}/tasks/{id}":           "/workspaces/{workspaceId}/projects/{projectId}/tasks/{taskId}",
		"/v1/workspaces/{workspaceId}/time-off/policies/{id}":                 "/workspaces/{workspaceId}/time-off/policies/{policyId}",
		"/workspaces/{workspaceId}/user/{userId}/time-entries/{id}/duplicate": "/workspaces/{workspaceId}/user/{userId}/time-entries/{timeEntryId}/duplicate",
		"/workspaces/{workspaceId}/expenses/{expenseId}/files/{id}":           "/workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}",
		"/workspaces/{workspaceId}/webhooks/{id}/logs":                        "/workspaces/{workspaceId}/webhooks/{webhookId}/logs",
	}
	for input, want := range cases {
		if got := normalizeDocumentedAPIPath(input); got != want {
			t.Fatalf("normalizeDocumentedAPIPath(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestProbeLabAPIAllowlistCoversOpenAPIDriftPatch(t *testing.T) {
	for _, operation := range []string{
		"PUT /workspaces/{workspaceId}",
		"GET /workspaces/{workspaceId}/expenses/{expenseId}/files/{fileId}",
		"GET /workspaces/{workspaceId}/time-off/balance/user/{userId}",
		"GET /workspaces/{workspaceId}/time-off/requests",
		"GET /workspaces/{workspaceId}/user-groups/{groupId}",
		"GET /workspaces/{workspaceId}/user-groups/{groupId}/users",
		"GET /workspaces/{workspaceId}/users/{userId}/time-off/balances",
		"GET /workspaces/{workspaceId}/webhooks/{webhookId}/logs",
		"PATCH /workspaces/{workspaceId}/webhooks/{webhookId}/token",
		"POST /workspaces/{workspaceId}/policies/{policyId}/requests",
		"DELETE /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}",
		"PATCH /workspaces/{workspaceId}/policies/{policyId}/requests/{requestId}",
		"PUT /workspaces/{workspaceId}/time-off/policies/{policyId}",
	} {
		if _, ok := documentedOperationByKey(operation); !ok {
			t.Fatalf("documented allowlist missing %s", operation)
		}
	}
}

func TestProbeLabAPIReadCallResolvesPathAndRepeatedQuery(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/users/u1/managers" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query()["status"]; !reflect.DeepEqual(got, []string{"ACTIVE", "PENDING"}) {
			t.Fatalf("status query values = %v", got)
		}
		respondJSON(t, w, []map[string]any{{"id": "m1"}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CallDocumentedReadAPI(context.Background(), map[string]any{
		"operation": "GET /workspaces/{workspaceId}/users/{userId}/managers",
		"path_params": map[string]any{
			"userId": "u1",
		},
		"query": map[string]any{
			"status": []any{"ACTIVE", "PENDING"},
		},
	})
	if err != nil {
		t.Fatalf("documented read call failed: %v", err)
	}
	if result.Meta["path"] != "/workspaces/ws1/users/u1/managers" {
		t.Fatalf("unexpected resolved path meta: %+v", result.Meta)
	}
}

func TestProbeLabAPIWriteCallSendsReportsJSONBody(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/workspaces/ws1/reports/summary" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		respondJSON(t, w, map[string]any{"totals": []any{}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DocumentedAPIWrites = true
	result, err := svc.CallDocumentedWriteAPI(context.Background(), map[string]any{
		"operation": "POST /workspaces/{workspaceId}/reports/summary",
		"json_body": map[string]any{
			"dateRangeStart": "2026-05-01T00:00:00.000",
			"dateRangeEnd":   "2026-05-07T23:59:59.999",
			"summaryFilter": map[string]any{
				"groups": []any{"DATE"},
			},
		},
	})
	if err != nil {
		t.Fatalf("documented write call failed: %v", err)
	}
	if result.Meta["reportsHost"] != true {
		t.Fatalf("reports operation should use reports host meta: %+v", result.Meta)
	}
	filter, _ := gotBody["summaryFilter"].(map[string]any)
	if got := filter["groups"]; !reflect.DeepEqual(got, []any{"DATE"}) {
		t.Fatalf("summaryFilter.groups body = %#v", got)
	}
}

func TestProbeLabAPIDeleteCallCanSendJSONBody(t *testing.T) {
	var gotBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/workspaces/ws1/users/u1/roles" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode delete body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	svc := New(client, "ws1")
	svc.DocumentedAPIWrites = true
	if _, err := svc.CallDocumentedDeleteAPI(context.Background(), map[string]any{
		"operation":   "DELETE /workspaces/{workspaceId}/users/{userId}/roles",
		"path_params": map[string]any{"userId": "u1"},
		"json_body":   map[string]any{"role": "MANAGER", "entityId": "p1"},
	}); err != nil {
		t.Fatalf("documented delete call failed: %v", err)
	}
	if gotBody["role"] != "MANAGER" || gotBody["entityId"] != "p1" {
		t.Fatalf("unexpected delete body: %#v", gotBody)
	}
}

func TestProbeLabAPIRejectsWrongToolClassAndUndocumentedPath(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.CallDocumentedReadAPI(context.Background(), map[string]any{
		"operation": "POST /workspaces/{workspaceId}/clients",
	}); err == nil || !strings.Contains(err.Error(), "does not allow POST") {
		t.Fatalf("expected read tool to reject POST, got %v", err)
	}
	if _, err := svc.CallDocumentedWriteAPI(context.Background(), map[string]any{
		"method": "POST",
		"path":   "/workspaces/{workspaceId}/not-a-real-route",
	}); err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("expected undocumented path rejection, got %v", err)
	}
}

func TestProbeLabAPIWriteDryRunDoesNotMutate(t *testing.T) {
	var called bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		respondJSON(t, w, map[string]any{})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.CallDocumentedWriteAPI(context.Background(), map[string]any{
		"operation": "PUT /workspaces/{workspaceId}/cost-rate",
		"json_body": map[string]any{
			"amount":     10000,
			"currencyId": "cur1",
		},
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("documented write dry-run failed: %v", err)
	}
	if called {
		t.Fatal("dry-run must not call upstream")
	}
	if result.Action != "clockify_call_documented_write_api" {
		t.Fatalf("unexpected action: %s", result.Action)
	}
}

func TestCallDocumentedWriteAPIBlockedWhenRawWritesDisabled(t *testing.T) {
	svc := New(nil, "ws1")
	_, err := svc.CallDocumentedWriteAPI(context.Background(), map[string]any{
		"operation": "POST /workspaces/{workspaceId}/clients",
		"json_body": map[string]any{"name": "Blocked"},
	})
	if err == nil || !strings.Contains(err.Error(), "CLOCKIFY_ENABLE_RAW_WRITES") || !strings.Contains(err.Error(), "clockify_clients_create") {
		t.Fatalf("err = %v, want documented API writes disabled error", err)
	}
}

func TestCallDocumentedReadAPIRejectsUnsafeRawPathEscapes(t *testing.T) {
	svc := New(nil, "ws1")
	cases := []struct {
		name string
		path string
	}{
		{"absolute_url", "https://evil.example/workspaces/{workspaceId}/clients"},
		{"encoded_host", "/%2f%2fevil.example/workspaces/{workspaceId}/clients"},
		{"dotdot", "/workspaces/{workspaceId}/../users"},
		{"backslash", `\workspaces\{workspaceId}\clients`},
		{"encoded_dotdot", "/workspaces/{workspaceId}/%2e%2e/users"},
		{"duplicate_slash", "//workspaces/{workspaceId}/clients"},
		{"query_confusion", "/workspaces/{workspaceId}?path=/workspaces/ws2/clients"},
		{"file_image", "/file/image"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.CallDocumentedReadAPI(context.Background(), map[string]any{
				"method": http.MethodGet,
				"path":   c.path,
			})
			if err == nil || !strings.Contains(err.Error(), "raw documented API path") {
				t.Fatalf("err = %v, want raw path rejection", err)
			}
		})
	}
}

func TestCallDocumentedReadAPIRejectsWorkspaceEnumerationAndOverride(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.CallDocumentedReadAPI(context.Background(), map[string]any{
		"operation": "GET /workspaces",
	}); err == nil || !strings.Contains(err.Error(), "pinned workspace") {
		t.Fatalf("GET /workspaces err = %v, want pinned workspace rejection", err)
	}
	if _, err := svc.CallDocumentedReadAPI(context.Background(), map[string]any{
		"operation":    "GET /workspaces/{workspaceId}",
		"workspace_id": "ws2",
	}); err == nil || !strings.Contains(err.Error(), "pinned workspace") {
		t.Fatalf("workspace override err = %v, want pinned workspace rejection", err)
	}
}

func TestCallDocumentedReadAPIAllowsPinnedWorkspaceAndUserPaths(t *testing.T) {
	svc := New(nil, "ws1")
	for _, path := range []string{
		"/user",
		"/workspaces/ws1",
		"/workspaces/ws1/webhooks",
	} {
		if err := svc.validateDocumentedAPIResolvedPath(path, "ws1"); err != nil {
			t.Fatalf("validate %s: %v", path, err)
		}
	}
}

func TestUploadImageSendsMultipartFile(t *testing.T) {
	var gotContentType string
	var gotBody string
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/file/image" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		gotBody = string(body)
		respondJSON(t, w, map[string]any{"id": "file-1"})
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UploadImage(context.Background(), map[string]any{
		"filename":     "avatar.png",
		"content_type": "image/png",
		"data_base64":  base64.StdEncoding.EncodeToString([]byte("png")),
	})
	if err != nil {
		t.Fatalf("UploadImage: %v", err)
	}
	if result.Action != "clockify_upload_image" {
		t.Fatalf("action = %s", result.Action)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Fatalf("Content-Type = %q", gotContentType)
	}
	for _, want := range []string{`name="file"; filename="avatar.png"`, "Content-Type: image/png", "png"} {
		if !strings.Contains(gotBody, want) {
			t.Fatalf("body missing %q: %s", want, gotBody)
		}
	}
}
