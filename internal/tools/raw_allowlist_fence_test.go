package tools

import (
	"strings"
	"testing"
)

// rawWriteRouteExceptions are the only generated write routes allowed to fall
// outside /workspaces/{workspaceId}/. Both are documented Clockify endpoints.
var rawWriteRouteExceptions = map[string]bool{
	"POST /file/image": true,
	"POST /workspaces": true,
}

func TestGeneratedRawWriteRoutesAreWorkspaceFenced(t *testing.T) {
	if len(documentedWriteRoutes) == 0 {
		t.Fatal("documentedWriteRoutes is empty")
	}
	for route := range documentedWriteRoutes {
		if rawWriteRouteExceptions[route] {
			continue
		}
		parts := strings.SplitN(route, " ", 2)
		if len(parts) != 2 {
			t.Errorf("malformed allowlist route %q", route)
			continue
		}
		path := parts[1]
		if path != "/workspaces/{workspaceId}" && !strings.HasPrefix(path, "/workspaces/{workspaceId}/") {
			t.Errorf("raw write route %q is not fenced inside /workspaces/{workspaceId}/", route)
		}
		if strings.Contains(path, "://") || strings.Contains(path, "..") {
			t.Errorf("raw write route %q contains an illegal path segment", route)
		}
	}
}
