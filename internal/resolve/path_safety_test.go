package resolve

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestPathSafety_ResolveUsesWorkspaceHelper enforces that every file in
// this package routes workspace URLs through the workspacePath helper
// instead of raw "/workspaces/" + workspaceID concatenation. Mirrors the
// internal/tools sibling test TestPathSafety_HandlersValidateIDsBeforeConcat
// but is scoped to this package because internal/paths.Workspace cannot be
// called from here (circular import: paths imports resolve for ValidateID).
//
// The five Resolve*ID helpers historically built workspace paths with raw
// string concatenation. Even though every upstream caller validated the
// workspace ID before calling in, the local layer carried no enforcement,
// so a future helper added with a different (unvalidated) workspace-ID
// source would silently inherit raw concat. The 2026-05-29 adversarial
// audit flagged that as drift bait. The fix is the small workspacePath
// helper in resolve.go that re-validates the workspace ID and
// percent-encodes every sub-segment; this test fails closed on textual
// regression.
//
// Allowed: the single literal "/workspaces/" + url.PathEscape(workspaceID)
// inside workspacePath itself (the only safe origin of that path prefix).
// Anything else is forbidden.
func TestPathSafety_ResolveUsesWorkspaceHelper(t *testing.T) {
	matches, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	// Match raw concat: `"/workspaces/" + <anything>` — that token has no
	// safe usage in this package outside workspacePath itself.
	bad := regexp.MustCompile(`"/workspaces/"\s*\+`)

	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		body := string(raw)
		hits := bad.FindAllStringIndex(body, -1)
		if len(hits) == 0 {
			continue
		}
		// Count the allowed occurrence inside workspacePath itself by
		// walking the function and tolerating exactly one hit when the
		// enclosing line is `b.WriteString("/workspaces/")` or directly
		// adjacent to the helper signature. Today the helper uses
		// b.WriteString("/workspaces/") with no `+`, so this regex
		// should not match at all and any hit is a real bug.
		for _, m := range hits {
			start := m[0]
			snippet := body[max(0, start-40):min(len(body), m[1]+40)]
			t.Errorf("%s contains forbidden raw concat `\"/workspaces/\" +` — route through workspacePath() instead (audit finding 2026-05-29 S6). Context: %q", path, snippet)
		}
	}
}
