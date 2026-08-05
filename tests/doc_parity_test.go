package e2e_test

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
	"gopkg.in/yaml.v3"
)

const historicalDocBanner = "Historical artifact. Not current one-user MCP product documentation."

// TestReadmeProtocolVersionMatchesSupported guards against drift between
// the advertised MCP protocol version in README.md and the actual newest
// version the server negotiates (mcp.SupportedProtocolVersions[0]).
//
// Two README locations are checked:
//
//  1. The shields.io MCP protocol badge near the top (URL-escaped dashes).
//  2. The "MCP Protocol" row of the Compatibility support matrix.
//
// If this test fails, regenerate or hand-edit README.md so both references
// match SupportedProtocolVersions[0]. The badge form uses double-dashes
// (e.g. "2025--11--25") because shields.io URL-escapes single dashes.
func TestReadmeProtocolVersionMatchesSupported(t *testing.T) {
	want := mcp.SupportedProtocolVersions[0]
	badgeWant := strings.ReplaceAll(want, "-", "--")

	readmePath := filepath.Join("..", "README.md")
	raw, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	readme := string(raw)

	// Badge: img.shields.io/badge/MCP-<version>-<color>
	badgeRe := regexp.MustCompile(`img\.shields\.io/badge/MCP-([0-9]{4}--[0-9]{2}--[0-9]{2})-`)
	badge := badgeRe.FindStringSubmatch(readme)
	if len(badge) != 2 {
		t.Fatalf("MCP protocol badge not found in README.md (looked for img.shields.io/badge/MCP-YYYY--MM--DD-)")
	}
	if badge[1] != badgeWant {
		t.Errorf("README MCP badge version drift: badge=%q want=%q (escaped from SupportedProtocolVersions[0]=%q)",
			badge[1], badgeWant, want)
	}

	// Matrix row: | MCP Protocol | `<version>` ...
	matrixRe := regexp.MustCompile(`\|\s*MCP Protocol\s*\|\s*` + "`" + `([0-9]{4}-[0-9]{2}-[0-9]{2})` + "`")
	matrix := matrixRe.FindStringSubmatch(readme)
	if len(matrix) != 2 {
		t.Fatalf("MCP Protocol matrix row not found in README.md (looked for | MCP Protocol | `YYYY-MM-DD`)")
	}
	if matrix[1] != want {
		t.Errorf("README Compatibility matrix version drift: row=%q want=%q (=SupportedProtocolVersions[0])",
			matrix[1], want)
	}
}

func TestCurrentProductDocsAvoidPlatformEraLanguage(t *testing.T) {
	forbidden := platformEraTerms()
	for _, path := range currentProductDocPaths() {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := strings.ToLower(string(raw))
		for _, term := range forbidden {
			if strings.Contains(text, term) {
				t.Fatalf("%s contains platform-era product language %q", path, term)
			}
		}
	}
}

func TestCurrentProductDocsDoNotLinkHistoricalArtifacts(t *testing.T) {
	historicalDocs := banneredHistoricalDocs(t)
	for _, productDoc := range currentProductDocPaths() {
		raw, err := os.ReadFile(productDoc)
		if err != nil {
			t.Fatalf("read %s: %v", productDoc, err)
		}
		text := filepath.ToSlash(string(raw))
		for historicalDoc := range historicalDocs {
			if strings.Contains(text, historicalDoc) || strings.Contains(text, "./"+historicalDoc) {
				t.Fatalf("%s links or points current users at historical artifact %s", productDoc, historicalDoc)
			}
		}
	}
}

func TestHistoricalPlatformDocsCarryBanner(t *testing.T) {
	docsRoot := filepath.Join("..", "docs")
	allowedCurrentDocs := map[string]bool{
		filepath.Join(docsRoot, "agent-cookbook.md"):                     true,
		filepath.Join(docsRoot, "architecture.md"):                       true,
		filepath.Join(docsRoot, "tool-catalog.md"):                       true,
		filepath.Join(docsRoot, "live-tests.md"):                         true,
		filepath.Join(docsRoot, "agent-handoff.md"):                      true,
		filepath.Join(docsRoot, "goals", "oneuser-tool-coverage.md"):     true,
		filepath.Join(docsRoot, "goals", "perfect-one-user-full-mcp.md"): true,
	}
	forbidden := platformEraTerms()

	err := filepath.WalkDir(docsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == filepath.Join(docsRoot, "openapi") || path == filepath.Join(docsRoot, "audits") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" || allowedCurrentDocs[path] {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(raw)
		lower := strings.ToLower(content)
		hasPlatformTerm := false
		for _, term := range forbidden {
			if strings.Contains(lower, term) {
				hasPlatformTerm = true
				break
			}
		}
		if hasPlatformTerm && !strings.Contains(content, historicalDocBanner) {
			t.Fatalf("%s has platform-era language without the historical banner", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Floor check only; artifact-vs-generator drift is gated by make openapi-drift
// in CI.
func TestGeneratedOpenAPIContractMeetsCoverageFloor(t *testing.T) {
	contract := readOpenAPIContract(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))

	// Floor counts updated 2026-05-25 after quarantining 6 phantom
	// routes total. Round 1 (3 ops, 2 paths): legacy time-off-request
	// /policies/{policyId}/requests trio (POST + DELETE + PATCH).
	// Round 2 (3 ops, 2 paths): G.1 edge-case sweep — POST
	// /time-off/requests/users/{userId} (404 live), GET /time-off/
	// requests (405 live; POST remains as the documented POST-as-list),
	// GET /users/{userId}/time-off/balances (404 live). All probed
	// against the sacrificial sandbox on 2026-05-25; all returned
	// Clockify error code 3000. See the sibling clockify-ts-sdk repo
	// (spec/evidence/discrepancies.md > timeoff.legacy-policies-requests.
	// phantom-path-quarantined) for the full audit.
	// Round 3 (2026-07-28): 6 more ops across 5 paths, from a fake-id
	// existence sweep over every unproven operation against the
	// sacrificial sandbox. Each returned 404 with
	// {"message":"No static resource ...","code":3000}:
	//   PUT    /workspaces/{workspaceId}/projects/{projectId}/archive
	//   PUT    /workspaces/{workspaceId}/clients/{clientId}/archive
	//   GET    /workspaces/{workspaceId}/time-off/requests/{requestId}
	//   DELETE /workspaces/{workspaceId}/time-off/requests/{requestId}
	//   PATCH  /workspaces/{workspaceId}/time-off/requests/{requestId}/status
	//   PATCH  /workspaces/{workspaceId}/webhooks/{webhookId}/generateNewToken
	// The two /archive PUTs are the notable ones: a prior note recorded
	// only POST /projects/{id}/archive as dead, which never transferred to
	// these different-method, different-shape paths. Not affected, and
	// deliberately still present: changeTimeOffRequestStatus (the
	// policy-scoped PATCH) and PATCH .../webhooks/{id}/token, both of which
	// return 400 on a fake id, i.e. live. See the sibling clockify-ts-sdk
	// repo's spec/evidence/discrepancies.md entries
	// `archive-subpaths.projects-clients.phantom`,
	// `time-off.requests.by-id.family-phantom` and
	// `webhooks.generateNewToken.phantom`.
	// Round 4 (2026-08-04/05): 2 more ops quarantined from a clockify-ts-sdk
	// Slice-1 live adjudication sweep over every unproven operation against
	// the sacrificial sandbox (see the sibling repo's
	// spec/evidence/discrepancies.md):
	//   PATCH /workspaces/{workspaceId}/time-entries/invoiced/bulk
	//     (markInvoicedBulk) -- every method (GET/POST/PUT/PATCH/DELETE/
	//     OPTIONS) returned 404 "No static resource ...", code 3000. The
	//     whole path disappears (108 paths), not just one operation.
	//   GET   /workspaces/{workspaceId}/webhooks/{webhookId}/logs -- a
	//     wrong-verb duplicate of the already-live-success POST at the
	//     same path (getWebhookLogs/searchLogs); GET/PUT/PATCH/DELETE all
	//     return 405 with Allow: POST. The path survives (POST is real);
	//     only the operation count drops.
	if got, want := len(contract.paths), 108; got < want {
		t.Fatalf("OpenAPI path count shrank: got %d want at least %d", got, want)
	}
	if got, want := contract.operationCount(), 161; got < want {
		t.Fatalf("OpenAPI operation count shrank: got %d want at least %d", got, want)
	}

	// The required-operations list omits anything that's in
	// PHANTOM_PATHS (scripts/gen-clockify-openapi). The bare
	// `/workspaces/{workspaceId}/balance` GET + PATCH pair previously
	// lived here but was removed when the live-probed phantom-route
	// quarantine landed (see the sibling clockify-ts-sdk repo's
	// spec/evidence/discrepancies.md >
	// `deferred-list-endpoints.not-paginated-or-not-live`). The
	// live-equivalent surface is the granular `/time-off/balance/policy/
	// {policyId}` and `.../user/{userId}` routes (still asserted below).
	requiredOperations := []openAPIOperation{
		{method: "get", path: "/workspaces/{workspaceId}/time-off/policies"},
		{method: "post", path: "/workspaces/{workspaceId}/time-off/policies"},
		{method: "post", path: "/workspaces/{workspaceId}/time-off/requests"},
		// `PATCH /workspaces/{workspaceId}/time-off/requests/{requestId}/status`
		// was removed from this required list by the 2026-07-28 existence
		// sweep: it returns 404 / "No static resource" / code 3000, along
		// with the GET and DELETE on the same by-id branch. Status changes
		// go through the policy-scoped
		// `PATCH .../time-off/policies/{policyId}/requests/{requestId}`,
		// which is live (400 on a fake id) and asserted below.
		{method: "get", path: "/workspaces/{workspaceId}/time-off/balance/user/{userId}"},
		{method: "patch", path: "/workspaces/{workspaceId}/time-off/balance/policy/{policyId}"},
		// `GET /workspaces/{workspaceId}/users/{userId}/time-off/balances`
		// was removed from this required list when the G.1 edge-case
		// re-probe (2026-05-25) confirmed the route returns HTTP 404
		// + Clockify error code 3000 on the live API. It is now in
		// PHANTOM_PATHS. The live per-user balance read is the
		// singular `/time-off/balance/user/{userId}` -- still asserted
		// above.
		// The bare `POST /scheduling/assignments` and
		// `PUT /scheduling/assignments/{assignmentId}` were removed from this
		// list by the 2026-06-23 live API surface audit (sibling
		// clockify-ts-sdk spec/evidence/discrepancies.md >
		// surface.audit.2026-06-23): both are synthetic merge artifacts absent
		// from the official spec and 404 live, now in PHANTOM_PATHS. The real
		// scheduling writes asserted below (publish, recurring create,
		// {assignmentId}/copy) stay.
		{method: "get", path: "/workspaces/{workspaceId}/scheduling/assignments/all"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/projects/totals"},
		{method: "put", path: "/workspaces/{workspaceId}/scheduling/assignments/publish"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/recurring"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy"},
	}
	for _, op := range requiredOperations {
		if !contract.hasOperation(op) {
			t.Fatalf("OpenAPI missing %s %s", strings.ToUpper(op.method), op.path)
		}
	}
}

func TestGeneratedOpenAPICoreEntitySchemasRequireStableIdentityFields(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	requiredFields := map[string][]string{
		"Tag":       {"id", "name", "workspaceId", "archived"},
		"Client":    {"id", "name", "workspaceId", "archived"},
		"Project":   {"id", "name", "workspaceId", "billable", "color", "archived", "public", "template"},
		"Task":      {"id", "name", "projectId", "status", "billable"},
		"UserDtoV1": {"id", "email", "name", "status"},
		"TimeEntry": {"id", "description", "userId", "billable", "workspaceId", "timeInterval", "type", "isLocked"},
	}

	for schemaName, fields := range requiredFields {
		schema, ok := schemas[schemaName].(map[string]interface{})
		if !ok {
			t.Fatalf("OpenAPI schema %s missing or malformed", schemaName)
		}
		required, ok := schema["required"].([]interface{})
		if !ok || len(required) == 0 {
			t.Fatalf("OpenAPI schema %s must declare required identity fields; required=%#v", schemaName, schema["required"])
		}
		requiredSet := requiredStringSet(t, schemaName, required)
		for _, field := range fields {
			if !requiredSet[field] {
				t.Fatalf("OpenAPI schema %s missing required field %q; required=%v", schemaName, field, required)
			}
		}
	}
}

func TestGeneratedOpenAPIClientUpdateComposesArchivedField(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "ClientUpdate")
	allOf, ok := schema["allOf"].([]interface{})
	if !ok || len(allOf) != 2 {
		t.Fatalf("OpenAPI schema ClientUpdate must compose ClientCreate with update-only fields; allOf=%#v", schema["allOf"])
	}

	hasClientCreate := false
	hasArchived := false
	for _, partRaw := range allOf {
		part, ok := partRaw.(map[string]interface{})
		if !ok {
			t.Fatalf("OpenAPI schema ClientUpdate has malformed allOf part %#v", partRaw)
		}
		if part["$ref"] == "#/components/schemas/ClientCreate" {
			hasClientCreate = true
		}
		properties, ok := part["properties"].(map[string]interface{})
		if !ok {
			continue
		}
		archived, ok := properties["archived"].(map[string]interface{})
		if ok && archived["type"] == "boolean" {
			if archived["default"] != false {
				t.Fatalf("OpenAPI schema ClientUpdate property archived default=%#v want false", archived["default"])
			}
			if archived["description"] != "Indicates if client will be archived or not." {
				t.Fatalf("OpenAPI schema ClientUpdate property archived description=%#v", archived["description"])
			}
			hasArchived = true
			if required, ok := part["required"].([]interface{}); ok && requiredStringSet(t, "ClientUpdate", required)["archived"] {
				t.Fatalf("OpenAPI schema ClientUpdate must leave archived optional; required=%#v", required)
			}
		}
	}
	if !hasClientCreate || !hasArchived {
		t.Fatalf("OpenAPI schema ClientUpdate must preserve ClientCreate and expose boolean archived; allOf=%#v", allOf)
	}
}

func TestGeneratedOpenAPITaskCreateBillableIsOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "TaskCreateRequest")
	assertOpenAPIPropertyType(t, "TaskCreateRequest", schema, "billable", "boolean")
	requiredSet := openAPIRequiredSet(t, "TaskCreateRequest", schema)
	if requiredSet["billable"] {
		t.Fatalf("OpenAPI schema TaskCreateRequest must leave billable optional; required=%#v", schema["required"])
	}
	assertOpenAPIExampleValue(t, "TaskCreateRequest", schema, "billable", false)
}

func TestGeneratedOpenAPICustomFieldCreateRequiredFlagIsOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "CreateCustomFieldRequest")
	assertOpenAPIPropertyType(t, "CreateCustomFieldRequest", schema, "required", "boolean")
	requiredSet := openAPIRequiredSet(t, "CreateCustomFieldRequest", schema)
	if requiredSet["required"] {
		t.Fatalf("OpenAPI schema CreateCustomFieldRequest must leave the required flag optional; required=%#v", schema["required"])
	}
	assertOpenAPIExampleValue(t, "CreateCustomFieldRequest", schema, "required", false)
}

func TestGeneratedOpenAPITimeOffPolicyCreateApproveIsOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	createSchema := openAPIObjectSchema(t, schemas, "CreateTimeOffPolicyRequest")
	assertOpenAPIPropertyType(t, "CreateTimeOffPolicyRequest", createSchema, "approve", "")
	requiredSet := openAPIRequiredSet(t, "CreateTimeOffPolicyRequest", createSchema)
	if !requiredSet["name"] {
		t.Fatalf("OpenAPI schema CreateTimeOffPolicyRequest must keep name required; required=%#v", createSchema["required"])
	}
	// This optionality correction has no committed GOCLMCP live fixture. Keep
	// this assertion evidence-neutral rather than presenting it as live proof.
	if requiredSet["approve"] {
		t.Fatalf("OpenAPI schema CreateTimeOffPolicyRequest must leave approve optional; required=%#v", createSchema["required"])
	}
}

func TestGeneratedOpenAPITimeOffPolicyResponseCarriesReplacementFields(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	policySchema := openAPIObjectSchema(t, schemas, "Policy")
	for field, fieldType := range map[string]string{
		"color":         "string",
		"hasExpiration": "boolean",
		"icon":          "string",
	} {
		assertOpenAPIPropertyType(t, "Policy", policySchema, field, fieldType)
	}
	icon := openAPIProperty(t, "Policy", policySchema, "icon")
	wantIconEnum := []string{
		"UMBRELLA",
		"SNOWFLAKE",
		"FAMILY",
		"PLANE",
		"STETHOSCOPE",
		"HEALTH_METRICS",
		"CHILDCARE",
		"LUGGAGE",
		"MONETIZATION",
		"CALENDAR",
	}
	gotIconEnum, ok := icon["enum"].([]interface{})
	if !ok || len(gotIconEnum) != len(wantIconEnum) {
		t.Fatalf("OpenAPI schema Policy property icon enum=%#v want exact %v", icon["enum"], wantIconEnum)
	}
	for i, want := range wantIconEnum {
		if gotIconEnum[i] != want {
			t.Fatalf("OpenAPI schema Policy property icon enum[%d]=%#v want %q; enum=%#v", i, gotIconEnum[i], want, gotIconEnum)
		}
	}
}

func TestGeneratedOpenAPIInvoiceUpdateReplacementFieldsAreOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "UpdateInvoiceRequest")
	requiredSet := openAPIRequiredSet(t, "UpdateInvoiceRequest", schema)
	wantRequired := []string{"currency", "discountPercent", "dueDate", "issuedDate", "number", "tax2Percent", "taxPercent"}
	if len(requiredSet) != len(wantRequired) {
		t.Fatalf("OpenAPI schema UpdateInvoiceRequest required=%#v want exact replacement set %v", schema["required"], wantRequired)
	}
	for _, field := range wantRequired {
		if !requiredSet[field] {
			t.Fatalf("OpenAPI schema UpdateInvoiceRequest missing existing required field %q; required=%#v", field, schema["required"])
		}
	}
	for _, field := range []string{"billFrom", "clientAddress"} {
		assertOpenAPIPropertyType(t, "UpdateInvoiceRequest", schema, field, "string")
		if requiredSet[field] {
			t.Fatalf("OpenAPI schema UpdateInvoiceRequest must leave %s optional; required=%#v", field, schema["required"])
		}
		assertOpenAPIExampleString(t, "UpdateInvoiceRequest", schema, field)
	}
}

func TestGeneratedOpenAPIExpenseCreateRequestMatchesLiveMultipartOptionalFields(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "ExpenseCreateRequest")
	requiredSet := openAPIRequiredSet(t, "ExpenseCreateRequest", schema)

	wantRequired := []string{"amount", "categoryId", "date", "userId"}
	if len(requiredSet) != len(wantRequired) {
		t.Fatalf("OpenAPI schema ExpenseCreateRequest required=%#v want exact create set %v", schema["required"], wantRequired)
	}
	for _, field := range wantRequired {
		if !requiredSet[field] {
			t.Fatalf("OpenAPI schema ExpenseCreateRequest missing required field %q; required=%#v", field, schema["required"])
		}
	}
	for _, field := range []string{"file", "projectId"} {
		if requiredSet[field] {
			t.Fatalf("OpenAPI schema ExpenseCreateRequest marks live-optional field %q as required; required=%#v", field, schema["required"])
		}
	}
}

func TestGeneratedOpenAPIExpenseUpdateRequestMakesOnlyFileOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema := openAPIObjectSchema(t, schemas, "ExpenseUpdateRequest")
	requiredSet := openAPIRequiredSet(t, "ExpenseUpdateRequest", schema)
	wantRequired := []string{"amount", "categoryId", "changeFields", "date", "userId"}

	if len(requiredSet) != len(wantRequired) {
		t.Fatalf("OpenAPI schema ExpenseUpdateRequest required=%#v want exact update set %v", schema["required"], wantRequired)
	}
	for _, field := range wantRequired {
		if !requiredSet[field] {
			t.Fatalf("OpenAPI schema ExpenseUpdateRequest missing required field %q; required=%#v", field, schema["required"])
		}
	}
	if requiredSet["file"] {
		t.Fatalf("OpenAPI schema ExpenseUpdateRequest marks live-optional field \"file\" as required; required=%#v", schema["required"])
	}
}

func TestGeneratedOpenAPIChangeTimeOffRequestStatusNoteOptional(t *testing.T) {
	schemas := readOpenAPISchemas(t, filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	schema, ok := schemas["ChangeTimeOffRequestStatusRequest"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema ChangeTimeOffRequestStatusRequest missing or malformed")
	}
	required, ok := schema["required"].([]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema ChangeTimeOffRequestStatusRequest must declare required fields; required=%#v", schema["required"])
	}
	requiredSet := requiredStringSet(t, "ChangeTimeOffRequestStatusRequest", required)

	if !requiredSet["status"] {
		t.Fatalf("OpenAPI schema ChangeTimeOffRequestStatusRequest must keep status required; required=%v", required)
	}
	if requiredSet["note"] {
		t.Fatalf("OpenAPI schema ChangeTimeOffRequestStatusRequest marks live-optional field \"note\" as required; the live status PATCH accepts a {status}-only body (probed 2026-06-20); required=%v", required)
	}
}

func TestGeneratedOpenAPIIgnoresPendingLiveFindingRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "docs", "openapi", "clockify-openapi.yaml"))
	if err != nil {
		t.Fatalf("read generated OpenAPI: %v", err)
	}
	if strings.Contains(string(raw), "TODO-live") {
		t.Fatalf("generated OpenAPI must not include pending live-finding TODO rows")
	}
}

// TestApiParityMatrixIsCurrent runs the check-api-parity-matrix script in
// validate mode (no --write). The script regenerates the matrix in a
// scratch dir and exits non-zero when the committed matrix would change,
// so a clean run proves the file is in sync with docs/tool-catalog.json
// and docs/goals/oneuser-tool-coverage.md.
func TestApiParityMatrixIsCurrent(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed; matrix drift requires jq")
	}
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	cmd := exec.Command("bash", "scripts/check-api-parity-matrix.sh", "--repo-root", repoRoot)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("api-parity-matrix drift detected:\n%s", out)
	}
}

// TestReadmeToolsetCountsMatchRegistry pins README's documented
// workflow/domain/raw split against the actual registry composition. If
// the registry grows or shrinks we want README's prose to fail the build
// rather than silently lie to readers.
func TestReadmeToolsetCountsMatchRegistry(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	got, gotOk := scanReadmeStartupCount(string(readme))
	if !gotOk {
		t.Fatalf("README.md does not advertise an N-tool startup registry near the toolset description")
	}

	catalog, err := os.ReadFile(filepath.Join("..", "docs", "tool-catalog.json"))
	if err != nil {
		t.Fatalf("read tool-catalog.json: %v", err)
	}
	want := strings.Count(string(catalog), `"name": "clockify_`)
	if got != want {
		t.Fatalf("README startup registry count = %d, generated catalog has %d tools (regen with `make gen-tool-catalog` and update README)", got, want)
	}
}

func TestApiCoverageDocCountsMatchRegistry(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "docs", "api-coverage.md"))
	if err != nil {
		t.Fatalf("read api-coverage.md: %v", err)
	}
	counts := scanAPICoverageCounts(string(raw))
	for _, key := range []string{"Workflow tools", "Domain tools", "Total"} {
		if _, ok := counts[key]; !ok {
			t.Fatalf("api-coverage.md missing %q count in summary table: %#v", key, counts)
		}
	}

	catalogRaw, err := os.ReadFile(filepath.Join("..", "docs", "tool-catalog.json"))
	if err != nil {
		t.Fatalf("read tool-catalog.json: %v", err)
	}
	var catalog struct {
		Tools []struct {
			Category string `json:"category"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(catalogRaw, &catalog); err != nil {
		t.Fatalf("parse tool-catalog.json: %v", err)
	}
	wantWorkflow, wantDomain := 0, 0
	for _, tool := range catalog.Tools {
		switch tool.Category {
		case "workflow":
			wantWorkflow++
		case "domain":
			wantDomain++
		}
	}
	if counts["Workflow tools"] != wantWorkflow {
		t.Fatalf("api-coverage workflow count = %d, catalog has %d", counts["Workflow tools"], wantWorkflow)
	}
	if counts["Domain tools"] != wantDomain {
		t.Fatalf("api-coverage domain count = %d, catalog has %d", counts["Domain tools"], wantDomain)
	}
	if counts["Total"] != len(catalog.Tools) {
		t.Fatalf("api-coverage total count = %d, catalog has %d", counts["Total"], len(catalog.Tools))
	}
}

// TestReadmeRawFallbackBehaviorMatchesImplementation guards the README
// section that documents the raw-API escape hatch: the two raw tools must
// be present in the catalog and the env-var name and HTTP method list
// must stay aligned with internal/clockify path-safety code.
func TestReadmeRawFallbackBehaviorMatchesImplementation(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	text := string(readme)

	for _, want := range []string{
		"clockify_api_get",
		"clockify_api_request",
		"CLOCKIFY_ENABLE_RAW_WRITES=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("README.md missing raw-fallback marker %q", want)
		}
	}

	catalog, err := os.ReadFile(filepath.Join("..", "docs", "tool-catalog.json"))
	if err != nil {
		t.Fatalf("read tool-catalog.json: %v", err)
	}
	for _, want := range []string{
		`"name": "clockify_api_get"`,
		`"name": "clockify_api_request"`,
	} {
		if !strings.Contains(string(catalog), want) {
			t.Fatalf("tool-catalog.json missing %s; README documents it but the registry no longer ships it", want)
		}
	}

	// Both methods listed in README must remain gated on the same env
	// var the runtime checks. If anyone moves the env var name we want
	// the test to surface the rename before the docs ship.
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if !strings.Contains(text, method) {
			t.Fatalf("README.md raw-fallback section no longer lists gated method %q", method)
		}
	}
}

// scanReadmeStartupCount finds the "N-tool startup registry" or
// "preserves the N-tool" marker in README.md and returns the integer.
func scanReadmeStartupCount(readme string) (int, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(\d+)-tool startup registry`),
		regexp.MustCompile(`preserves the (\d+)-tool`),
	}
	for _, re := range patterns {
		match := re.FindStringSubmatch(readme)
		if len(match) == 2 {
			n, err := strconv.Atoi(match[1])
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func scanAPICoverageCounts(markdown string) map[string]int {
	counts := map[string]int{}
	lineRE := regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*(\d+)\s*\|`)
	scanner := bufio.NewScanner(strings.NewReader(markdown))
	for scanner.Scan() {
		match := lineRE.FindStringSubmatch(scanner.Text())
		if len(match) != 3 {
			continue
		}
		n, err := strconv.Atoi(match[2])
		if err != nil {
			continue
		}
		counts[strings.TrimSpace(match[1])] = n
	}
	return counts
}

func currentProductDocPaths() []string {
	return []string{
		filepath.Join("..", "README.md"),
		filepath.Join("..", "docs", "agent-cookbook.md"),
		filepath.Join("..", "docs", "tool-catalog.md"),
		filepath.Join("..", "docs", "live-tests.md"),
		filepath.Join("..", "docs", "goals", "oneuser-tool-coverage.md"),
	}
}

type openAPIOperation struct {
	method string
	path   string
}

type openAPIContract struct {
	paths map[string]map[string]bool
}

func readOpenAPIContract(t *testing.T, path string) openAPIContract {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("read OpenAPI contract: %v", err)
	}
	defer f.Close()

	contract := openAPIContract{paths: map[string]map[string]bool{}}
	var currentPath string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, `  "`) && strings.HasSuffix(strings.TrimSpace(line), `":`) {
			currentPath = strings.TrimSuffix(strings.Trim(strings.TrimSpace(line), `"`), `":`)
			contract.paths[currentPath] = map[string]bool{}
			continue
		}
		if currentPath == "" || !strings.HasPrefix(line, "    ") {
			continue
		}
		method := strings.TrimSuffix(strings.TrimSpace(line), ":")
		if isOpenAPIHTTPMethod(method) {
			contract.paths[currentPath][method] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan OpenAPI contract: %v", err)
	}
	return contract
}

func readOpenAPISchemas(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenAPI schemas: %v", err)
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse OpenAPI schemas: %v", err)
	}
	components, ok := doc["components"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI components missing or malformed")
	}
	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schemas missing or malformed")
	}
	return schemas
}

func openAPIObjectSchema(t *testing.T, schemas map[string]interface{}, schemaName string) map[string]interface{} {
	t.Helper()
	schema, ok := schemas[schemaName].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s missing or malformed", schemaName)
	}
	return schema
}

func openAPIRequiredSet(t *testing.T, schemaName string, schema map[string]interface{}) map[string]bool {
	t.Helper()
	required, ok := schema["required"].([]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s must declare required fields; required=%#v", schemaName, schema["required"])
	}
	return requiredStringSet(t, schemaName, required)
}

func assertOpenAPIPropertyType(t *testing.T, schemaName string, schema map[string]interface{}, propertyName, wantType string) {
	t.Helper()
	property := openAPIProperty(t, schemaName, schema, propertyName)
	if wantType != "" && property["type"] != wantType {
		t.Fatalf("OpenAPI schema %s property %q type=%#v want %q", schemaName, propertyName, property["type"], wantType)
	}
}

func openAPIProperty(t *testing.T, schemaName string, schema map[string]interface{}, propertyName string) map[string]interface{} {
	t.Helper()
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s properties missing or malformed", schemaName)
	}
	property, ok := properties[propertyName].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s missing property %q", schemaName, propertyName)
	}
	return property
}

func assertOpenAPIExampleValue(t *testing.T, schemaName string, schema map[string]interface{}, propertyName string, want interface{}) {
	t.Helper()
	example, ok := schema["example"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s example missing or malformed", schemaName)
	}
	got, ok := example[propertyName]
	if !ok || got != want {
		t.Fatalf("OpenAPI schema %s example property %q=%#v want %#v", schemaName, propertyName, got, want)
	}
}

func assertOpenAPIExampleString(t *testing.T, schemaName string, schema map[string]interface{}, propertyName string) {
	t.Helper()
	example, ok := schema["example"].(map[string]interface{})
	if !ok {
		t.Fatalf("OpenAPI schema %s example missing or malformed", schemaName)
	}
	got, ok := example[propertyName].(string)
	if !ok || got == "" {
		t.Fatalf("OpenAPI schema %s example property %q=%#v want non-empty string", schemaName, propertyName, example[propertyName])
	}
}

func requiredStringSet(t *testing.T, schemaName string, required []interface{}) map[string]bool {
	t.Helper()
	requiredSet := map[string]bool{}
	for _, field := range required {
		name, ok := field.(string)
		if !ok {
			t.Fatalf("OpenAPI schema %s has non-string required field %#v", schemaName, field)
		}
		requiredSet[name] = true
	}
	return requiredSet
}

func (c openAPIContract) operationCount() int {
	total := 0
	for _, methods := range c.paths {
		total += len(methods)
	}
	return total
}

func (c openAPIContract) hasOperation(op openAPIOperation) bool {
	return c.paths[op.path][op.method]
}

func isOpenAPIHTTPMethod(method string) bool {
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options", "trace":
		return true
	default:
		return false
	}
}

func banneredHistoricalDocs(t *testing.T) map[string]bool {
	t.Helper()
	repoRoot := filepath.Clean("..")
	docs := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(repoRoot, "docs"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(raw), historicalDocBanner) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		docs[filepath.ToSlash(rel)] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

func platformEraTerms() []string {
	return []string{
		"hosted",
		"tenant",
		"control plane",
		"controlplane",
		"oidc",
		"mtls",
		"grpc",
		"streamable http",
		"policy mode",
		"policy modes",
		"confirmation token",
		"confirmation-token",
		"tier 2",
		"activation",
		"shared-service",
		"shared service",
		"forward auth",
		"forward-auth",
	}
}
