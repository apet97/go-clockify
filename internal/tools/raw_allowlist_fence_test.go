package tools

import (
	"strings"
	"testing"
)

func TestGeneratedRawWriteRoutesAreWorkspaceFenced(t *testing.T) {
	if len(documentedWriteRoutes) == 0 {
		t.Fatal("documentedWriteRoutes is empty")
	}
	for route := range documentedWriteRoutes {
		parts := strings.SplitN(route, " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed allowlist route %q", route)
			continue
		}
		path := parts[1]
		concrete := strings.ReplaceAll(path, "{workspaceId}", "ws1")
		if _, err := safeRawPath("ws1", concrete); err != nil {
			t.Errorf("raw write route %q fails raw path scope after workspace substitution: %v", route, err)
		}
		if strings.Contains(path, "://") || strings.Contains(path, "..") {
			t.Errorf("raw write route %q contains an illegal path segment", route)
		}
	}
}

func TestDocumentedButRawUnsupportedRoutesAreSeparated(t *testing.T) {
	want := map[string]string{
		"POST /file/image": "global file endpoint",
		"POST /workspaces": "workspace creation",
	}
	for route, reason := range want {
		got, ok := documentedButRawUnsupportedRoutes[route]
		if !ok {
			t.Fatalf("documentedButRawUnsupportedRoutes missing %s", route)
		}
		if !strings.Contains(got, reason) {
			t.Fatalf("%s reason = %q, want to mention %q", route, got, reason)
		}
		if documentedWriteRoutes[route] {
			t.Fatalf("%s must not remain callable through documentedWriteRoutes", route)
		}
	}
}
