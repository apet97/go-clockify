package tools

import "strings"

func isDocumentedRawWriteRoute(method, path string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = normalizeRawAllowlistPath(path)
	return documentedWriteRoutes[method+" "+path]
}

func normalizeRawAllowlistPath(path string) string {
	path = strings.TrimSpace(path)
	parts := strings.Split(path, "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "workspaces" && parts[i+1] != "" {
			parts[i+1] = "{workspaceId}"
			break
		}
	}
	return strings.Join(parts, "/")
}
