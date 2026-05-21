package tools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenWorkspacePlaceholders are raw-API path forms that safeRawPath does
// NOT substitute. Only "{workspaceId}" is valid.
var forbiddenWorkspacePlaceholders = []string{
	"/workspaces/" + "{id}",
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

// walkActiveDocs invokes fn for every tracked active (non-archived,
// non-bannered) Markdown file under docs/. The tracked-file boundary is
// deliberate: local probe/planning folders under docs/ often contain raw
// upstream examples such as /workspaces/{id}; they should not make
// deterministic tests fail until a human stages them as active docs.
func walkActiveDocs(t *testing.T, fn func(path string, content []byte)) {
	t.Helper()
	root := filepath.Join("..", "..")
	walkActiveDocsFromPaths(t, trackedDocsMarkdownFiles(t, root), fn)
}

func walkActiveDocsFromPaths(t *testing.T, paths []string, fn func(path string, content []byte)) {
	t.Helper()
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read doc %s: %v", path, readErr)
		}
		if docsLintIsActive(path, content) {
			fn(path, content)
		}
	}
}

func trackedDocsMarkdownFiles(t *testing.T, repoRoot string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z", "--", "docs")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list tracked docs: %v", err)
	}
	parts := bytes.Split(out, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		path := string(part)
		if !strings.HasSuffix(path, ".md") {
			continue
		}
		paths = append(paths, filepath.Join(repoRoot, filepath.FromSlash(path)))
	}
	return paths
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

func TestActiveDocsLintIgnoresUntrackedScratchMarkdown(t *testing.T) {
	root := t.TempDir()
	active := filepath.Join(root, "docs", "active.md")
	scratch := filepath.Join(root, "docs", "scratch", "plan.md")
	if err := os.MkdirAll(filepath.Dir(scratch), 0o755); err != nil {
		t.Fatalf("mkdir scratch: %v", err)
	}
	if err := os.WriteFile(active, []byte("Use /workspaces/{workspaceId}.\n"), 0o644); err != nil {
		t.Fatalf("write active doc: %v", err)
	}
	if err := os.WriteFile(scratch, []byte("Scratch upstream note: /workspaces/{id}.\n"), 0o644); err != nil {
		t.Fatalf("write scratch doc: %v", err)
	}

	var seen []string
	walkActiveDocsFromPaths(t, []string{active}, func(path string, _ []byte) {
		seen = append(seen, path)
	})
	if len(seen) != 1 || seen[0] != active {
		t.Fatalf("tracked doc walk saw %v, want only %s", seen, active)
	}
}
