package tools

import (
	"strings"
	"sync"
)

// rawWriteAllowlist is documentedWriteRoutes parsed once into per-method lists
// of path-segment templates. The generated map (raw_allowlist_gen.go) stays the
// single source of truth; this index just lets the matcher compare each path
// parameter segment-by-segment instead of by literal string.
var (
	rawWriteAllowlistOnce  sync.Once
	rawWriteAllowlistIndex map[string][][]string
)

func rawWriteAllowlist() map[string][][]string {
	rawWriteAllowlistOnce.Do(func() {
		rawWriteAllowlistIndex = make(map[string][][]string)
		for route := range documentedWriteRoutes {
			method, path, ok := strings.Cut(route, " ")
			if !ok {
				continue
			}
			method = strings.ToUpper(strings.TrimSpace(method))
			rawWriteAllowlistIndex[method] = append(rawWriteAllowlistIndex[method], splitRawAllowlistPath(path))
		}
	})
	return rawWriteAllowlistIndex
}

// splitRawAllowlistPath splits a Clockify API path into segments, trimming
// leading and trailing slashes so "/workspaces/ws1/clients" and
// "workspaces/ws1/clients" segment identically.
func splitRawAllowlistPath(path string) []string {
	return strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/")
}

// isDocumentedRawWriteRoute reports whether method+path matches a documented
// Clockify write endpoint. Every "{...}" segment in a documented template
// (e.g. {workspaceId}, {projectId}, {taskId}) matches any non-empty concrete
// segment, so a real request such as DELETE /workspaces/ws1/projects/p1
// matches the template DELETE /workspaces/{workspaceId}/projects/{projectId}.
func isDocumentedRawWriteRoute(method, path string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	request := splitRawAllowlistPath(path)
	for _, template := range rawWriteAllowlist()[method] {
		if rawAllowlistSegmentsMatch(template, request) {
			return true
		}
	}
	return false
}

func rawAllowlistSegmentsMatch(template, request []string) bool {
	if len(template) != len(request) {
		return false
	}
	for i, seg := range template {
		if len(seg) >= 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			if request[i] == "" {
				return false
			}
			continue
		}
		if seg != request[i] {
			return false
		}
	}
	return true
}
