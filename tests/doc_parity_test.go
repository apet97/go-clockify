package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
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
			if path == filepath.Join(docsRoot, "openapi") {
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

func currentProductDocPaths() []string {
	return []string{
		filepath.Join("..", "README.md"),
		filepath.Join("..", "docs", "agent-cookbook.md"),
		filepath.Join("..", "docs", "tool-catalog.md"),
		filepath.Join("..", "docs", "live-tests.md"),
		filepath.Join("..", "docs", "goals", "oneuser-tool-coverage.md"),
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
