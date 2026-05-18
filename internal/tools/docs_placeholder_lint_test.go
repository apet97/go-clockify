package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenWorkspacePlaceholders are raw-API path forms that safeRawPath does
// NOT substitute. Only "{workspaceId}" is valid.
var forbiddenWorkspacePlaceholders = []string{
	"/workspaces/{id}",
	"/workspaces/{workspace_id}",
	"/workspaces/<workspaceId>",
	"/workspaces/<workspace-id>",
	"/workspaces/{CLOCKIFY_WORKSPACE_ID}",
}

// docsLintIsActive reports whether a doc file should be linted. Archived and
// OpenAPI-source material is exempt: it preserves historical/raw API text.
func docsLintIsActive(path string, content []byte) bool {
	if strings.Contains(path, "/archive/") || strings.Contains(path, "/openapi/") {
		return false
	}
	head := content
	if len(head) > 600 {
		head = head[:600]
	}
	return !strings.Contains(string(head), "Historical artifact")
}

// walkActiveDocs invokes fn for every active (non-archived, non-bannered)
// Markdown file under docs/.
func walkActiveDocs(t *testing.T, fn func(path string, content []byte)) {
	t.Helper()
	root := filepath.Join("..", "..", "docs")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if docsLintIsActive(path, content) {
			fn(path, content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}
}

func TestActiveDocsUseOnlyTheWorkspaceIdPlaceholder(t *testing.T) {
	walkActiveDocs(t, func(path string, content []byte) {
		text := string(content)
		for _, bad := range forbiddenWorkspacePlaceholders {
			if strings.Contains(text, bad) {
				t.Errorf("%s contains forbidden raw-path placeholder %q; use /workspaces/{workspaceId}", path, bad)
			}
		}
	})
}
