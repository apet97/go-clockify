package tools

import (
	"strings"
	"testing"
)

func FuzzSafeRawPath(f *testing.F) {
	const workspaceID = "65b382b606de527a7ee2b60e"
	for _, seed := range []string{
		"/user",
		"/workspaces/{workspaceId}/projects",
		"workspaces/65b382b606de527a7ee2b60e/time-entries",
		"https://api.clockify.me/api/v1/workspaces/65b382b606de527a7ee2b60e/projects",
		"/workspaces/other/projects",
		"/workspaces/{workspaceId}/../projects",
		"/workspaces/{workspaceId}//projects",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		path, err := safeRawPath(workspaceID, raw)
		if err != nil {
			return
		}
		if strings.Contains(path, "://") || strings.Contains(path, "\\") || strings.Contains(path, "..") || strings.ContainsAny(path, "?#") {
			t.Fatalf("unsafe path accepted: raw=%q path=%q", raw, path)
		}
		if path != "/user" && path != "/workspaces/"+workspaceID && !strings.HasPrefix(path, "/workspaces/"+workspaceID+"/") {
			t.Fatalf("path escaped pinned workspace: raw=%q path=%q", raw, path)
		}
	})
}
