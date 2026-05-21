package tools

import (
	"strings"
	"testing"
)

func FuzzSafeRawPath(f *testing.F) {
	const workspaceID = "000000000000000000000001"
	for _, seed := range []string{
		"/user",
		"/workspaces/{workspaceId}/projects",
		"workspaces/000000000000000000000001/time-entries",
		"https://api.clockify.me/api/v1/workspaces/000000000000000000000001/projects",
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
