// Command live-clean-prefix deletes objects in the pinned sacrificial Clockify
// workspace whose name — or, for objects with no name, a matching field —
// starts with CLOCKIFY_LIVE_PREFIX.
//
// It is a best-effort sweep: every list and delete is independent, and a
// failure on one object is logged and does not stop the rest. It NEVER touches
// objects without the prefix and refuses to run unless the confirm variable
// matches the workspace.
//
// Sweep order is children before parents: scheduling assignments and time-off
// requests first, projects and clients last. Scheduling assignments carry no
// name of their own, so they are matched by their linked prefixed project —
// by projectId (learned from a pre-pass over prefixed projects) or by the
// embedded projectName/clientName. Time-off requests are listed with a POST
// search body because the Clockify list endpoint is POST-only.
//
// Paginated reads detect an endpoint that ignores pagination: a full page that
// repeats with no new object IDs marks the collection failed instead of
// spinning to the hard page cap.
//
// After the sweep it performs a paginated post-delete rescan of every
// prefixed-object collection and reports that leftover count. User invites are
// outside this prefixed-delete sweep. A machine-readable JSON summary is
// printed as the final stdout line.
//
// Set CLOCKIFY_LIVE_CLEAN_DRY_RUN=1 to list what would be deleted without
// mutating the workspace; a dry run still exits non-zero when any collection
// list fails, so the preview is never silently incomplete.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
)

// methodPost marks a collection whose list endpoint is POST-only (the Clockify
// time-off request search). An empty collection.listMethod means GET.
const methodPost = "POST"

// labelProjects is the projects collection label. Projects are pre-listed so
// scheduling assignments can be matched by their linked projectId, then swept
// near the end once every child collection is clear.
const labelProjects = "projects"

// collection is one prefixed-object sweep target.
//
//   - listMethod == methodPost lists via a POST search body instead of GET.
//   - matchLinkedProject also matches an item whose projectId belongs to a
//     prefixed project (used for scheduling assignments, which have no name).
//   - deleteQuery, when set, is sent as query parameters on the DELETE.
//   - archiveFirst PUTs {"archived":true} before delete (clients and projects
//     that own archived children cannot be deleted while still active).
type collection struct {
	label              string
	path               string
	deletePath         string
	deleteQuery        map[string]string
	listMethod         string
	listKeys           []string
	matchFields        []string
	matchLinkedProject bool
	query              map[string]string
	archiveFirst       bool
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
	os.Exit(run(os.Stdout))
}

func run(stdout io.Writer) int {
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

	s := &sweeper{
		ctx:                context.Background(),
		client:             client,
		workspaceID:        wsID,
		prefix:             prefix,
		dryRun:             dryRun,
		out:                stdout,
		deleted:            map[string]int{},
		failed:             map[string]int{},
		leftovers:          map[string]int{},
		prefixedProjectIDs: map[string]bool{},
	}

	if dryRun {
		s.printf("DRY RUN - prefix %q - no objects will be deleted\n\n", prefix)
	} else {
		s.printf("Sweeping prefix %q\n\n", prefix)
	}

	collections := buildCollections(wsID)

	// Pre-pass: learn which projects carry the prefix so scheduling
	// assignments — which have no name of their own — can be matched by their
	// linked projectId. Projects are deleted last, so this runs before any
	// mutation; a list failure here is best-effort and assignment matching
	// falls back to the embedded projectName/clientName.
	var projectItems []namedObject
	for _, c := range collections {
		if c.label == labelProjects {
			projectItems = s.listPrefixed(c)
			for _, p := range projectItems {
				s.prefixedProjectIDs[p.ID] = true
			}
			break
		}
	}

	for _, c := range collections {
		if c.label == labelProjects {
			// Reuse the pre-pass listing rather than hitting the endpoint twice.
			s.sweepItems(c, projectItems)
			continue
		}
		s.sweep(c)
	}

	if !dryRun {
		s.printf("\nPost-delete rescan:\n")
		for _, c := range collections {
			remaining := s.listPrefixed(c)
			if len(remaining) > 0 {
				s.leftovers[c.label] = len(remaining)
			}
			s.printf("  %s: %d remaining\n", c.label, len(remaining))
		}
	}
	return s.report()
}

// buildCollections returns the sweep targets in deletion order: children
// before parents. Scheduling assignments and time-off requests lead so they
// are cleared while their linked projects/policies still exist; projects and
// clients trail so archived children are gone before the parent delete.
func buildCollections(wsID string) []collection {
	ws := "/workspaces/" + wsID
	return []collection{
		{
			label: "scheduling assignments",
			path:  ws + "/scheduling/assignments/all",
			// Deletion uses the recurring-assignment route with
			// seriesUpdateOption=ALL so the whole series is removed; the bare
			// /scheduling/assignments/{id} route is not a valid endpoint.
			deletePath:  ws + "/scheduling/assignments/recurring",
			deleteQuery: map[string]string{"seriesUpdateOption": "ALL"},
			// Wide static window so assignments from any past or future test
			// fixture are listed; the endpoint filters by period overlap.
			query:              map[string]string{"start": "2000-01-01T00:00:00Z", "end": "2100-01-01T00:00:00Z"},
			matchFields:        []string{"note", "projectName", "clientName"},
			matchLinkedProject: true,
		},
		{
			label:       "time-off requests",
			path:        ws + "/time-off/requests",
			listMethod:  methodPost,
			listKeys:    []string{"requests"},
			matchFields: []string{"note", "notes", "reason", "description", "name"},
		},
		{label: "time-off policies", path: ws + "/time-off/policies"},
		{label: "expenses", path: ws + "/expenses", listKeys: []string{"expenses", "expenses"}, matchFields: []string{"name", "notes", "note"}},
		{label: "invoices", path: ws + "/invoices", listKeys: []string{"invoices"}, matchFields: []string{"name", "number"}},
		{label: "webhooks", path: ws + "/webhooks", listKeys: []string{"webhooks"}},
		{label: "user-groups", path: ws + "/user-groups"},
		{label: "holidays", path: ws + "/holidays"},
		{label: "tags", path: ws + "/tags"},
		{label: labelProjects, path: ws + "/projects", archiveFirst: true},
		{label: "clients", path: ws + "/clients", archiveFirst: true},
	}
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
	ctx                context.Context
	client             *clockify.Client
	workspaceID        string
	prefix             string
	dryRun             bool
	out                io.Writer
	deleted            map[string]int
	failed             map[string]int
	leftovers          map[string]int
	prefixedProjectIDs map[string]bool
}

type namedObject struct {
	ID   string
	Name string
}

// printf and println are the sweeper's only stdout path. They discard the
// io.Writer error once here — a write failure to the run's captured stdout is
// not independently recoverable — so call sites stay errcheck-clean.
func (s *sweeper) printf(format string, args ...any) {
	_, _ = fmt.Fprintf(s.out, format, args...)
}

func (s *sweeper) println(args ...any) {
	_, _ = fmt.Fprintln(s.out, args...)
}

// listPrefixed pages through a collection endpoint and returns every entry
// that matches the prefix. It stops at the first short page, hard-caps
// iteration, and — so a paging-blind endpoint cannot be processed forever —
// fails the collection when a full page repeats with no new object IDs.
func (s *sweeper) listPrefixed(c collection) []namedObject {
	const pageSize = 200
	const maxPages = 100
	var out []namedObject
	seen := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		items, err := s.listPage(c, page, pageSize)
		if err != nil {
			s.printf("  list %s (page %d) failed: %v\n", c.label, page, s.cleanError(err))
			s.failed[c.label]++
			return out
		}
		newIDs := 0
		for _, item := range items {
			id, _ := item["id"].(string)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			newIDs++
			if !s.itemMatches(c, item) {
				continue
			}
			name := firstString(item, append(defaultMatchFields(c.matchFields), "id")...)
			out = append(out, namedObject{ID: id, Name: name})
		}
		if len(items) < pageSize {
			break
		}
		// A full page that introduced no new object IDs means the endpoint is
		// ignoring pagination and re-serving the same rows. Stop and mark the
		// collection failed rather than looping to maxPages on duplicates.
		if newIDs == 0 {
			s.printf("  list %s (page %d) repeated a full page with no new object IDs; pagination is broken\n", c.label, page)
			s.failed[c.label]++
			return out
		}
	}
	return out
}

// listPage fetches one page of a collection. POST collections send a JSON
// search body (page/pageSize); GET collections send page/page-size as query
// parameters. The decoded items are the raw Clockify objects.
func (s *sweeper) listPage(c collection, page, pageSize int) ([]map[string]any, error) {
	var raw json.RawMessage
	if c.listMethod == methodPost {
		body := map[string]any{"page": page, "pageSize": pageSize}
		if err := s.client.Post(s.ctx, c.path, body, &raw); err != nil {
			return nil, err
		}
	} else {
		query := mergeQuery(c.query, map[string]string{
			"page":      fmt.Sprintf("%d", page),
			"page-size": fmt.Sprintf("%d", pageSize),
		})
		if err := s.client.Get(s.ctx, c.path, query, &raw); err != nil {
			return nil, err
		}
	}
	return decodeList(raw, c.listKeys)
}

// itemMatches reports whether a raw object belongs to the current prefix run.
// A name-field prefix match always counts; a scheduling assignment also counts
// when its projectId is a prefixed project discovered by the pre-pass.
func (s *sweeper) itemMatches(c collection, item map[string]any) bool {
	if matchesPrefix(item, s.prefix, c.matchFields) {
		return true
	}
	if c.matchLinkedProject {
		if pid := stringFromAny(item["projectId"]); pid != "" && s.prefixedProjectIDs[pid] {
			return true
		}
	}
	return false
}

// sweep lists then deletes one collection.
func (s *sweeper) sweep(c collection) {
	s.sweepItems(c, s.listPrefixed(c))
}

// sweepItems deletes the supplied prefixed entries of one collection, or, in
// dry-run mode, only reports what it would delete. It is split from sweep so
// the projects collection can reuse the pre-pass listing.
func (s *sweeper) sweepItems(c collection, items []namedObject) {
	if len(items) == 0 {
		s.printf("%s: none\n", c.label)
		return
	}
	if s.dryRun {
		s.printf("%s: %d prefixed object(s) would be deleted\n", c.label, len(items))
		for _, it := range items {
			s.printf("  [dry-run] would delete %s (%s)\n", it.Name, it.ID)
		}
		// In dry-run the live inventory IS the leftover set: nothing is removed.
		s.leftovers[c.label] = len(items)
		return
	}
	s.printf("%s: %d prefixed object(s)\n", c.label, len(items))
	for _, it := range items {
		basePath := c.path
		if c.deletePath != "" {
			basePath = c.deletePath
		}
		itemPath := basePath + "/" + it.ID
		if c.archiveFirst {
			var ignore map[string]any
			_ = s.client.Put(s.ctx, itemPath, archivePayload(c, it), &ignore)
		}
		if err := s.deleteItem(c, itemPath); err != nil {
			s.printf("  DELETE %s %s (%s) failed: %v\n", c.label, it.Name, it.ID, s.cleanError(err))
			s.failed[c.label]++
			continue
		}
		s.deleted[c.label]++
	}
}

// deleteItem issues the DELETE for one object, sending c.deleteQuery as query
// parameters when the collection requires them (scheduling assignments need
// seriesUpdateOption).
func (s *sweeper) deleteItem(c collection, itemPath string) error {
	if len(c.deleteQuery) > 0 {
		return s.client.DeleteWithQuery(s.ctx, itemPath, c.deleteQuery)
	}
	return s.client.Delete(s.ctx, itemPath)
}

func archivePayload(c collection, it namedObject) map[string]any {
	payload := map[string]any{"archived": true}
	if c.label == "clients" && it.Name != "" && it.Name != "<unnamed>" {
		payload["name"] = it.Name
	}
	return payload
}

// report prints the human-readable and JSON summaries and returns the process
// exit code. A real run exits 1 on delete failures or post-rescan leftovers.
// A dry run exits 1 only when a collection list failed — the preview would
// otherwise be silently incomplete — and 0 when every list succeeded.
func (s *sweeper) report() int {
	deleted := sumCounts(s.deleted)
	failed := sumCounts(s.failed)
	leftovers := sumCounts(s.leftovers)

	if s.dryRun {
		s.printf("\nDry run complete. Would delete %d object(s); nothing was changed.\n", leftovers)
		if failed > 0 {
			s.printf("%d collection list(s) failed; this preview is incomplete.\n", failed)
		}
	} else {
		s.printf("\nDeleted %d, failed %d. Leftovers: %d\n", deleted, failed, leftovers)
		if failed > 0 || leftovers > 0 {
			s.println("Some prefixed objects remain. Inspect them manually, then re-run.")
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
	s.println(string(b))

	if s.dryRun {
		if failed > 0 {
			return 1
		}
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
	maps.Copy(out, base)
	maps.Copy(out, extra)
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
