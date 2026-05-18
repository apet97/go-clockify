// Command live-clean-prefix deletes objects in the pinned sacrificial Clockify
// workspace whose name starts with CLOCKIFY_LIVE_PREFIX. It is a best-effort
// sweep: every list and delete is independent, and a failure on one object is
// logged and does not stop the rest. It NEVER touches objects without the
// prefix and refuses to run unless the confirm variable matches the workspace.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

func main() {
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	wsID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	confirm := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM"))

	if apiKey == "" || wsID == "" || prefix == "" || confirm == "" {
		fmt.Fprintln(os.Stderr, "live-clean-prefix: set CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, CLOCKIFY_LIVE_PREFIX, and CLOCKIFY_LIVE_WORKSPACE_CONFIRM")
		os.Exit(2)
	}
	if confirm != wsID {
		fmt.Fprintln(os.Stderr, "live-clean-prefix: CLOCKIFY_LIVE_WORKSPACE_CONFIRM does not match CLOCKIFY_WORKSPACE_ID; refusing to run")
		os.Exit(2)
	}

	baseURL := strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.clockify.me/api/v1"
	}
	client := clockify.NewClient(apiKey, baseURL, 30*time.Second, 0)
	client.SetUserAgent("clockify-mcp-live-clean-prefix")
	defer client.Close()

	ctx := context.Background()
	swept := newSweeper(ctx, client, wsID, prefix)

	// Order matters: tasks vanish with their projects; archive projects and
	// clients before deleting them.
	swept.sweepProjectsAndTasks()
	swept.sweepSimple("clients", "/workspaces/"+wsID+"/clients", true)
	swept.sweepSimple("tags", "/workspaces/"+wsID+"/tags", false)
	swept.sweepSimple("invoices", "/workspaces/"+wsID+"/invoices", false)
	swept.sweepSimple("expenses", "/workspaces/"+wsID+"/expenses", false)
	swept.sweepSimple("holidays", "/workspaces/"+wsID+"/holidays", false)
	swept.sweepSimple("webhooks", "/workspaces/"+wsID+"/webhooks", false)
	swept.sweepSimple("user-groups", "/workspaces/"+wsID+"/user-groups", false)
	swept.sweepSimple("time-off policies", "/workspaces/"+wsID+"/time-off/policies", false)
	swept.sweepSimple("scheduling assignments", "/workspaces/"+wsID+"/scheduling/assignments", false)

	fmt.Printf("\nLeftovers: %d (deleted %d, failed %d)\n", swept.failed, swept.deleted, swept.failed)
	if swept.failed > 0 {
		fmt.Println("Some objects could not be deleted. Inspect them manually, then re-run.")
		os.Exit(1)
	}
}

type sweeper struct {
	ctx     context.Context
	client  *clockify.Client
	wsID    string
	prefix  string
	deleted int
	failed  int
}

func newSweeper(ctx context.Context, c *clockify.Client, wsID, prefix string) *sweeper {
	return &sweeper{ctx: ctx, client: c, wsID: wsID, prefix: prefix}
}

// listPrefixed lists a collection endpoint and returns the id/name pairs of
// entries whose "name" begins with the prefix.
func (s *sweeper) listPrefixed(path string) []struct{ ID, Name string } {
	var raw []map[string]any
	if err := s.client.Get(s.ctx, path, map[string]string{"page-size": "200"}, &raw); err != nil {
		fmt.Printf("  list %s failed: %v\n", path, err)
		return nil
	}
	var out []struct{ ID, Name string }
	for _, item := range raw {
		id, _ := item["id"].(string)
		name, _ := item["name"].(string)
		if id == "" || !strings.HasPrefix(name, s.prefix) {
			continue
		}
		out = append(out, struct{ ID, Name string }{id, name})
	}
	return out
}

func (s *sweeper) delete(path string) bool {
	if err := s.client.Delete(s.ctx, path); err != nil {
		fmt.Printf("  DELETE %s failed: %v\n", path, err)
		s.failed++
		return false
	}
	s.deleted++
	return true
}

// sweepSimple deletes prefixed entries of a flat collection. When archiveFirst
// is true it PUTs {"archived":true} before deleting (clients with archived
// projects, etc.).
func (s *sweeper) sweepSimple(label, listPath string, archiveFirst bool) {
	items := s.listPrefixed(listPath)
	if len(items) == 0 {
		fmt.Printf("%s: none\n", label)
		return
	}
	fmt.Printf("%s: %d prefixed object(s)\n", label, len(items))
	for _, it := range items {
		itemPath := listPath + "/" + it.ID
		if archiveFirst {
			var ignore map[string]any
			_ = s.client.Put(s.ctx, itemPath, map[string]any{"archived": true}, &ignore)
		}
		s.delete(itemPath)
	}
}

// sweepProjectsAndTasks archives then deletes prefixed projects. Tasks are
// removed implicitly when their project is deleted.
func (s *sweeper) sweepProjectsAndTasks() {
	listPath := "/workspaces/" + s.wsID + "/projects"
	items := s.listPrefixed(listPath)
	if len(items) == 0 {
		fmt.Println("projects: none")
		return
	}
	fmt.Printf("projects: %d prefixed object(s)\n", len(items))
	for _, it := range items {
		itemPath := listPath + "/" + it.ID
		var ignore map[string]any
		_ = s.client.Put(s.ctx, itemPath, map[string]any{"archived": true}, &ignore)
		s.delete(itemPath)
	}
}
