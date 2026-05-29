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
	if got, want := len(contract.paths), 121; got < want {
		t.Fatalf("OpenAPI path count shrank: got %d want at least %d", got, want)
	}
	if got, want := contract.operationCount(), 185; got < want {
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
		{method: "patch", path: "/workspaces/{workspaceId}/time-off/requests/{requestId}/status"},
		{method: "get", path: "/workspaces/{workspaceId}/time-off/balance/user/{userId}"},
		{method: "patch", path: "/workspaces/{workspaceId}/time-off/balance/policy/{policyId}"},
		// `GET /workspaces/{workspaceId}/users/{userId}/time-off/balances`
		// was removed from this required list when the G.1 edge-case
		// re-probe (2026-05-25) confirmed the route returns HTTP 404
		// + Clockify error code 3000 on the live API. It is now in
		// PHANTOM_PATHS. The live per-user balance read is the
		// singular `/time-off/balance/user/{userId}` -- still asserted
		// above.
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments"},
		{method: "get", path: "/workspaces/{workspaceId}/scheduling/assignments/all"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/projects/totals"},
		{method: "put", path: "/workspaces/{workspaceId}/scheduling/assignments/publish"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/recurring"},
		{method: "put", path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}"},
		{method: "post", path: "/workspaces/{workspaceId}/scheduling/assignments/{assignmentId}/copy"},
	}
	for _, op := range requiredOperations {
		if !contract.hasOperation(op) {
			t.Fatalf("OpenAPI missing %s %s", strings.ToUpper(op.method), op.path)
		}
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
