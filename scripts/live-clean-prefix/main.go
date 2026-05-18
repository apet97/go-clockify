// Command live-clean-prefix deletes objects in the pinned sacrificial Clockify
// workspace whose name starts with CLOCKIFY_LIVE_PREFIX.
//
// It is a best-effort sweep: every list and delete is independent, and a
// failure on one object is logged and does not stop the rest. It NEVER touches
// objects without the prefix and refuses to run unless the confirm variable
// matches the workspace.
//
// After the sweep it performs a paginated post-delete rescan of every
// collection and reports the true leftover count. A machine-readable JSON
// summary is printed as the final stdout line.
//
// Set CLOCKIFY_LIVE_CLEAN_DRY_RUN=1 to list what would be deleted without
// mutating the workspace.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// collection is one prefixed-object sweep target. archiveFirst PUTs
// {"archived":true} before delete (clients and projects that own archived
// children cannot be deleted while still active).
type collection struct {
	label        string
	path         string
	deletePath   string
	listKeys     []string
	matchFields  []string
	query        map[string]string
	archiveFirst bool
}

// cleanSummary is the machine-readable summary printed as the final stdout
// line. Counts are keyed by collection label.
type cleanSummary struct {
	Prefix    string         `json:"prefix"`
	DryRun    bool           `json:"dry_run"`
	Deleted   map[string]int `json:"deleted"`
	Failed    map[string]int `json:"failed"`
	Leftovers map[string]int `json:"leftovers"`
}

func main() {
	os.Exit(run())
}

func run() int {
	apiKey := strings.TrimSpace(os.Getenv("CLOCKIFY_API_KEY"))
	wsID := strings.TrimSpace(os.Getenv("CLOCKIFY_WORKSPACE_ID"))
	prefix := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_PREFIX"))
	confirm := strings.TrimSpace(os.Getenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM"))
	dryRun := isTrue(os.Getenv("CLOCKIFY_LIVE_CLEAN_DRY_RUN"))

	if apiKey == "" || wsID == "" || prefix == "" || confirm == "" {
		fmt.Fprintln(os.Stderr, "live-clean-prefix: set CLOCKIFY_API_KEY, CLOCKIFY_WORKSPACE_ID, CLOCKIFY_LIVE_PREFIX, and CLOCKIFY_LIVE_WORKSPACE_CONFIRM")
		return 2
	}
	if confirm != wsID {
		fmt.Fprintln(os.Stderr, "live-clean-prefix: CLOCKIFY_LIVE_WORKSPACE_CONFIRM does not match CLOCKIFY_WORKSPACE_ID; refusing to run")
		return 2
	}

	baseURL := strings.TrimSpace(os.Getenv("CLOCKIFY_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.clockify.me/api/v1"
	}
	client := clockify.NewClient(apiKey, baseURL, 30*time.Second, 0)
	client.SetUserAgent("clockify-mcp-live-clean-prefix")
	defer client.Close()

	// Order matters: tasks vanish with their projects; archive projects and
	// clients before deleting them.
	assignmentsStart := time.Now().UTC().AddDate(-1, 0, 0).Format(time.RFC3339)
	assignmentsEnd := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)
	collections := []collection{
		{label: "projects", path: "/workspaces/" + wsID + "/projects", archiveFirst: true},
		{label: "clients", path: "/workspaces/" + wsID + "/clients", archiveFirst: true},
		{label: "tags", path: "/workspaces/" + wsID + "/tags"},
		{label: "invoices", path: "/workspaces/" + wsID + "/invoices", listKeys: []string{"invoices"}, matchFields: []string{"name", "number"}},
		{label: "expenses", path: "/workspaces/" + wsID + "/expenses", listKeys: []string{"expenses", "expenses"}, matchFields: []string{"name", "notes", "note"}},
		{label: "holidays", path: "/workspaces/" + wsID + "/holidays"},
		{label: "webhooks", path: "/workspaces/" + wsID + "/webhooks", listKeys: []string{"webhooks"}},
		{label: "user-groups", path: "/workspaces/" + wsID + "/user-groups"},
		{label: "time-off policies", path: "/workspaces/" + wsID + "/time-off/policies"},
		{
			label:      "scheduling assignments",
			path:       "/workspaces/" + wsID + "/scheduling/assignments/all",
			deletePath: "/workspaces/" + wsID + "/scheduling/assignments",
			query:      map[string]string{"start": assignmentsStart, "end": assignmentsEnd},
		},
	}

	s := &sweeper{
		ctx:         context.Background(),
		client:      client,
		workspaceID: wsID,
		prefix:      prefix,
		dryRun:      dryRun,
		deleted:     map[string]int{},
		failed:      map[string]int{},
		leftovers:   map[string]int{},
	}

	if dryRun {
		fmt.Printf("DRY RUN - prefix %q - no objects will be deleted\n\n", prefix)
	} else {
		fmt.Printf("Sweeping prefix %q\n\n", prefix)
	}
	for _, c := range collections {
		s.sweep(c)
	}

	if !dryRun {
		fmt.Printf("\nPost-delete rescan:\n")
		for _, c := range collections {
			remaining := s.listPrefixed(c)
			if len(remaining) > 0 {
				s.leftovers[c.label] = len(remaining)
			}
			fmt.Printf("  %s: %d remaining\n", c.label, len(remaining))
		}
	}
	return s.report()
}

// isTrue parses a permissive boolean env value.
func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

type sweeper struct {
	ctx         context.Context
	client      *clockify.Client
	workspaceID string
	prefix      string
	dryRun      bool
	deleted     map[string]int
	failed      map[string]int
	leftovers   map[string]int
}

type namedObject struct {
	ID   string
	Name string
}

// listPrefixed pages through a collection endpoint and returns every entry
// whose configured match field begins with the prefix. It stops at the first
// short page and hard-caps iteration so an endpoint that ignores paging cannot
// loop forever.
func (s *sweeper) listPrefixed(c collection) []namedObject {
	const pageSize = 200
	const maxPages = 100
	var out []namedObject
	for page := 1; page <= maxPages; page++ {
		var raw json.RawMessage
		query := mergeQuery(c.query, map[string]string{
			"page":      fmt.Sprintf("%d", page),
			"page-size": fmt.Sprintf("%d", pageSize),
		})
		if err := s.client.Get(s.ctx, c.path, query, &raw); err != nil {
			fmt.Printf("  list %s (page %d) failed: %v\n", c.label, page, s.cleanError(err))
			s.failed[c.label]++
			return out
		}
		items, err := decodeList(raw, c.listKeys)
		if err != nil {
			fmt.Printf("  list %s (page %d) returned an unsupported shape: %v\n", c.label, page, err)
			s.failed[c.label]++
			return out
		}
		for _, item := range items {
			id, _ := item["id"].(string)
			name := firstString(item, append(defaultMatchFields(c.matchFields), "id")...)
			if id == "" || !matchesPrefix(item, s.prefix, c.matchFields) {
				continue
			}
			out = append(out, namedObject{ID: id, Name: name})
		}
		if len(items) < pageSize {
			break
		}
	}
	return out
}

// sweep lists the prefixed entries of one collection and deletes them, or, in
// dry-run mode, only reports what it would delete.
func (s *sweeper) sweep(c collection) {
	items := s.listPrefixed(c)
	if len(items) == 0 {
		fmt.Printf("%s: none\n", c.label)
		return
	}
	if s.dryRun {
		fmt.Printf("%s: %d prefixed object(s) would be deleted\n", c.label, len(items))
		for _, it := range items {
			fmt.Printf("  [dry-run] would delete %s (%s)\n", it.Name, it.ID)
		}
		// In dry-run the live inventory IS the leftover set: nothing is removed.
		s.leftovers[c.label] = len(items)
		return
	}
	fmt.Printf("%s: %d prefixed object(s)\n", c.label, len(items))
	for _, it := range items {
		basePath := c.path
		if c.deletePath != "" {
			basePath = c.deletePath
		}
		itemPath := basePath + "/" + it.ID
		if c.archiveFirst {
			var ignore map[string]any
			_ = s.client.Put(s.ctx, itemPath, map[string]any{"archived": true}, &ignore)
		}
		if err := s.client.Delete(s.ctx, itemPath); err != nil {
			fmt.Printf("  DELETE %s %s (%s) failed: %v\n", c.label, it.Name, it.ID, s.cleanError(err))
			s.failed[c.label]++
			continue
		}
		s.deleted[c.label]++
	}
}

// report prints the human-readable and JSON summaries and returns the process
// exit code: 0 on a clean sweep (or any dry run), 1 when delete failures or
// post-rescan leftovers remain.
func (s *sweeper) report() int {
	deleted := sumCounts(s.deleted)
	failed := sumCounts(s.failed)
	leftovers := sumCounts(s.leftovers)

	if s.dryRun {
		fmt.Printf("\nDry run complete. Would delete %d object(s); nothing was changed.\n", leftovers)
	} else {
		fmt.Printf("\nDeleted %d, failed %d. Leftovers: %d\n", deleted, failed, leftovers)
		if failed > 0 || leftovers > 0 {
			fmt.Println("Some prefixed objects remain. Inspect them manually, then re-run.")
		}
	}

	b, err := json.Marshal(cleanSummary{
		Prefix:    s.prefix,
		DryRun:    s.dryRun,
		Deleted:   s.deleted,
		Failed:    s.failed,
		Leftovers: s.leftovers,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "live-clean-prefix: cannot marshal JSON summary: %v\n", err)
		return 1
	}
	fmt.Println(string(b))

	if s.dryRun {
		return 0
	}
	if failed > 0 || leftovers > 0 {
		return 1
	}
	return 0
}

func sumCounts(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

func mergeQuery(base, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func decodeList(raw json.RawMessage, keys []string) ([]map[string]any, error) {
	var direct []map[string]any
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	var cur any = obj
	for _, key := range keys {
		asMap, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("key %q parent is %T, want object", key, cur)
		}
		cur = asMap[key]
	}
	items, ok := cur.([]any)
	if !ok {
		return nil, fmt.Errorf("selected value is %T, want array", cur)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		asMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, asMap)
	}
	return out, nil
}

func defaultMatchFields(fields []string) []string {
	if len(fields) > 0 {
		return fields
	}
	return []string{"name"}
}

func matchesPrefix(item map[string]any, prefix string, fields []string) bool {
	for _, field := range defaultMatchFields(fields) {
		if strings.HasPrefix(stringFromAny(item[field]), prefix) {
			return true
		}
	}
	return false
}

func firstString(item map[string]any, fields ...string) string {
	for _, field := range fields {
		if v := stringFromAny(item[field]); v != "" {
			return v
		}
	}
	return "<unnamed>"
}

func stringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func (s *sweeper) cleanError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if s.workspaceID != "" {
		msg = strings.ReplaceAll(msg, s.workspaceID, "{workspaceId}")
	}
	return msg
}
