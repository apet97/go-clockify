package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPathSafety_HandlersValidateIDsBeforeConcat enforces the convention
// that every internal/tools/ handler file which concatenates a
// non-workspace ID into a Clockify URL path also has a validation gate
// in the same file — either a direct resolve.ValidateID call or a
// resolve.Resolve*ID resolver that performs ValidateID internally
// (resolveByNameOrID delegates to ValidateID before any name lookup,
// see internal/resolve/resolve.go).
//
// Why: clockify.Client.doOnce concatenates baseURL + path without
// per-segment escaping. Every ID-bearing path segment must be
// validated by the caller; a drift at this layer can let "/" or "?"
// smuggle a path-traversal or query-injection. Audit finding 3.
//
// Workspace IDs are exempt because they are validated at config load
// (Wave B) when sourced from CLOCKIFY_WORKSPACE_ID, and the
// auto-detect path additionally validates in GetWorkspace (Wave F).
// Non-workspace IDs (project, client, tag, user, invoice, …) come
// from MCP arguments and need per-handler validation.
//
// This is a coarse static gate — it does not prove validation runs
// before the concat, only that the file is aware of validation at
// all. The goal is to fail-closed on textual drift, catching the
// most common regression: a new handler or refactor that introduces
// a non-workspace ID concat without any validation pathway.
//
// When adding a legitimate exception (file with a hard-coded ID for
// a smoke test, or other ID source already validated at a layer this
// regex doesn't see), add it to pathSafetyAllowedNoValidate with a
// short justification.
func TestPathSafety_HandlersValidateIDsBeforeConcat(t *testing.T) {
	// Match concatenations like `+ projectID`, `+ invoiceID`, etc.
	// Workspace IDs (wsID, workspaceID) are matched separately so the
	// test can ignore files that only concat workspace IDs.
	idConcatRE := regexp.MustCompile(`\+\s*(\w+)(ID|Id)\b`)
	validateRE := regexp.MustCompile(`\b(resolve\.ValidateID|resolve\.Resolve\w+ID)\(`)

	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if pathSafetyAllowedNoValidate[filepath.Base(path)] {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(raw)
		hits := idConcatRE.FindAllStringSubmatch(body, -1)
		if len(hits) == 0 {
			continue
		}
		nonWorkspace := false
		for _, m := range hits {
			prefix := strings.ToLower(m[1])
			if prefix == "ws" || prefix == "workspace" {
				continue
			}
			nonWorkspace = true
			break
		}
		if !nonWorkspace {
			continue
		}
		if !validateRE.MatchString(body) {
			t.Errorf("%s concatenates a non-workspace ID into a path but neither calls resolve.ValidateID nor resolve.Resolve*ID — every ID-bearing path segment must be validated before reaching clockify.Client (audit finding 3). If this file is genuinely an exception, add it to pathSafetyAllowedNoValidate with a justification.", path)
		}
	}
}

// pathSafetyAllowedNoValidate lists handler files that legitimately
// concatenate an ID into a path without their own ValidateID call —
// either because the ID is a hard-coded literal, comes from an upstream
// service that has already validated it, or the file is a generator.
// Add an entry only with a written justification.
var pathSafetyAllowedNoValidate = map[string]bool{
	// No exceptions today. Keep this map minimal — every entry is a
	// gap in the static gate.
}
