package e2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Contracts here were each re-probed against the sacrificial sandbox on
// 2026-08-07 while auditing the corrected spec against live behaviour. Every
// one of them had drifted away from the wire without any gate noticing,
// because the repo's drift tooling compares operation existence and response
// codes but never parameters or schemas.

type openAPIDoc struct {
	Paths      map[string]map[string]operationDoc `yaml:"paths"`
	Components struct {
		Parameters map[string]parameterDoc `yaml:"parameters"`
		Schemas    map[string]yaml.Node    `yaml:"schemas"`
	} `yaml:"components"`
}

type operationDoc struct {
	Parameters []parameterDoc         `yaml:"parameters"`
	Responses  map[string]responseDoc `yaml:"responses"`
}

type parameterDoc struct {
	Ref    string `yaml:"$ref"`
	Name   string `yaml:"name"`
	In     string `yaml:"in"`
	Schema struct {
		Type    string   `yaml:"type"`
		Format  string   `yaml:"format"`
		Minimum *int     `yaml:"minimum"`
		Enum    []string `yaml:"enum"`
	} `yaml:"schema"`
}

type responseDoc struct {
	Content map[string]struct {
		Schema struct {
			Ref string `yaml:"$ref"`
		} `yaml:"schema"`
	} `yaml:"content"`
}

func readOpenAPIDoc(t *testing.T) openAPIDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	if err != nil {
		t.Fatalf("read OpenAPI: %v", err)
	}
	var doc openAPIDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse OpenAPI: %v", err)
	}
	return doc
}

func (doc openAPIDoc) operation(t *testing.T, path, method string) operationDoc {
	t.Helper()
	item, ok := doc.Paths[path]
	if !ok {
		t.Fatalf("OpenAPI path %s missing", path)
	}
	op, ok := item[method]
	if !ok {
		t.Fatalf("OpenAPI operation %s %s missing", method, path)
	}
	return op
}

// param resolves a component $ref to the parameter it declares, since a
// component keyed `Order` declares parameter `order`.
func (doc openAPIDoc) param(t *testing.T, op operationDoc, name string) parameterDoc {
	t.Helper()
	for _, p := range op.Parameters {
		resolved := p
		if p.Ref != "" {
			resolved = doc.Components.Parameters[filepath.Base(p.Ref)]
		}
		if resolved.Name == name {
			return resolved
		}
	}
	t.Fatalf("parameter %q missing; operation declares %v", name, doc.paramNames(op))
	return parameterDoc{}
}

func (doc openAPIDoc) paramNames(op operationDoc) []string {
	names := make([]string, 0, len(op.Parameters))
	for _, p := range op.Parameters {
		if p.Ref != "" {
			p = doc.Components.Parameters[filepath.Base(p.Ref)]
		}
		names = append(names, p.Name)
	}
	return names
}

func assertJSONResponseSchema(t *testing.T, op operationDoc, status, wantRef string) {
	t.Helper()
	media, ok := op.Responses[status].Content["application/json"]
	if !ok {
		t.Fatalf("response %s declares no application/json body; want %s", status, wantRef)
	}
	if media.Schema.Ref != wantRef {
		t.Fatalf("response %s schema=%q want %q", status, media.Schema.Ref, wantRef)
	}
}

// DELETE returns the deleted entity for clients and tags — unlike expenses,
// which really do answer with an empty body.
func TestOpenAPIDeleteReturnsDeletedEntity(t *testing.T) {
	doc := readOpenAPIDoc(t)
	for path, wantRef := range map[string]string{
		"/workspaces/{workspaceId}/clients/{clientId}": "#/components/schemas/Client",
		"/workspaces/{workspaceId}/tags/{tagId}":       "#/components/schemas/Tag",
	} {
		assertJSONResponseSchema(t, doc.operation(t, path, "delete"), "200", wantRef)
	}
}

// `order` binds to a Java int: `abc` returns a conversion error and `0`
// returns "must be greater than or equal to 1".
func TestOpenAPIInvoiceItemOrderIsBoundedInteger(t *testing.T) {
	doc := readOpenAPIDoc(t)
	op := doc.operation(t, "/workspaces/{workspaceId}/invoices/{invoiceId}/items/{order}", "delete")
	order := doc.param(t, op, "order")
	if order.Schema.Type != "integer" || order.Schema.Format != "int32" {
		t.Fatalf("order schema type=%q format=%q want integer/int32", order.Schema.Type, order.Schema.Format)
	}
	if order.Schema.Minimum == nil || *order.Schema.Minimum != 1 {
		t.Fatalf("order minimum=%v want 1", order.Schema.Minimum)
	}
}

// The bare GET renders the report; the saved configuration comes back nested
// under `filters`. Binding SharedReport there shares no top-level key with the
// wire.
func TestOpenAPIBareSharedReportGetReturnsRenderedReport(t *testing.T) {
	doc := readOpenAPIDoc(t)
	op := doc.operation(t, "/shared-reports/{sharedReportId}", "get")
	assertJSONResponseSchema(t, op, "200", "#/components/schemas/SharedReportData")
	for _, name := range []string{"dateRangeStart", "dateRangeEnd", "sortColumn", "sortOrder", "page", "pageSize", "exportType"} {
		doc.param(t, op, name)
	}
}

// Query parameters the corrected spec had dropped, all honoured live.
func TestOpenAPIRestoredQueryParameters(t *testing.T) {
	doc := readOpenAPIDoc(t)
	tags := doc.operation(t, "/workspaces/{workspaceId}/tags", "get")
	doc.param(t, tags, "strict-name-search")
	doc.param(t, tags, "excluded-ids")
	// The server names the accepted set in its own 400: "must be from the
	// following set: ID, NAME".
	if got := doc.param(t, tags, "sort-column").Schema.Enum; len(got) != 2 || got[0] != "ID" || got[1] != "NAME" {
		t.Fatalf("GET /tags sort-column enum=%v want [ID NAME]", got)
	}

	client := doc.operation(t, "/workspaces/{workspaceId}/clients/{clientId}", "put")
	doc.param(t, client, "archive-projects")
	doc.param(t, client, "mark-tasks-as-done")

	// An unknown value returns 500, so the enum is the whole contract.
	filter := doc.param(t, doc.operation(t, "/workspaces/{workspaceId}/shared-reports", "get"), "sharedReportsFilter")
	if len(filter.Schema.Enum) != 4 {
		t.Fatalf("sharedReportsFilter enum=%v want 4 values", filter.Schema.Enum)
	}

	doc.param(t, doc.operation(t, "/workspaces/{workspaceId}/approval-requests", "get"), "types")
}

// Schema-level contracts the wire disagreed with.
func TestOpenAPISchemasMatchObservedWire(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))

	// Creating a request without `note` returns 200.
	create := openAPIObjectSchema(t, schemas, "CreateTimeOffRequest")
	if openAPIRequiredSet(t, "CreateTimeOffRequest", create)["note"] {
		t.Fatalf("CreateTimeOffRequest must not require note; required=%#v", create["required"])
	}

	// `ALL` is a list filter, not a policy state: PATCHing it returns 400
	// "Invalid status".
	status := openAPIProperty(t, "PolicyStatusChangeRequest", openAPIObjectSchema(t, schemas, "PolicyStatusChangeRequest"), "status")
	for _, value := range status["enum"].([]interface{}) {
		if value == "ALL" {
			t.Fatalf("PolicyStatusChangeRequest.status must not accept ALL; enum=%#v", status["enum"])
		}
	}

	// Present (null for non-kiosk entries) on every create/stop response.
	assertOpenAPIPropertyType(t, "TimeEntriesTimeEntry", openAPIObjectSchema(t, schemas, "TimeEntriesTimeEntry"), "kioskId", "string")
}
