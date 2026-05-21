package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestDecodeListSupportsBareAndWrappedClockifyShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		keys []string
		want int
	}{
		{
			name: "bare array",
			raw:  `[{"id":"p1","name":"MCP-LIVE-project"}]`,
			want: 1,
		},
		{
			name: "invoice wrapper",
			raw:  `{"total":1,"invoices":[{"id":"i1","number":"MCP-LIVE-1"}]}`,
			keys: []string{"invoices"},
			want: 1,
		},
		{
			name: "expense nested wrapper",
			raw:  `{"expenses":{"expenses":[{"id":"e1","notes":"MCP-LIVE-expense"}],"count":1}}`,
			keys: []string{"expenses", "expenses"},
			want: 1,
		},
		{
			name: "webhook wrapper",
			raw:  `{"workspaceWebhookCount":1,"webhooks":[{"id":"w1","name":"MCP-LIVE-webhook"}]}`,
			keys: []string{"webhooks"},
			want: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeList(json.RawMessage(tc.raw), tc.keys)
			if err != nil {
				t.Fatalf("decodeList: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestMatchesPrefixUsesCollectionSpecificFields(t *testing.T) {
	item := map[string]any{"id": "i1", "number": "MCP-LIVE-1"}
	if !matchesPrefix(item, "MCP-LIVE", []string{"name", "number"}) {
		t.Fatal("invoice number should match the live cleanup prefix")
	}
	if matchesPrefix(item, "MCP-LIVE", nil) {
		t.Fatal("default name-only match should not match an invoice number")
	}
}

func TestCleanErrorRedactsWorkspaceID(t *testing.T) {
	s := &sweeper{workspaceID: "000000000000000000000001"}
	got := s.cleanError(errors.New("GET /workspaces/000000000000000000000001/invoices failed"))
	if strings.Contains(got, s.workspaceID) {
		t.Fatalf("workspace id was not redacted: %s", got)
	}
	if !strings.Contains(got, "{workspaceId}") {
		t.Fatalf("redacted placeholder missing: %s", got)
	}
}

func TestArchivePayloadIncludesClientName(t *testing.T) {
	got := archivePayload(collection{label: "clients"}, namedObject{ID: "c1", Name: "MCP-LIVE-client"})
	if got["archived"] != true {
		t.Fatalf("archived flag missing: %#v", got)
	}
	if got["name"] != "MCP-LIVE-client" {
		t.Fatalf("client archive payload must preserve name for Clockify PUT validation: %#v", got)
	}

	project := archivePayload(collection{label: "projects"}, namedObject{ID: "p1", Name: "MCP-LIVE-project"})
	if _, ok := project["name"]; ok {
		t.Fatalf("project archive payload should not add a name: %#v", project)
	}
}

// ---------------------------------------------------------------------------
// Full fake-server end-to-end tests
//
// These drive run() against an in-process fake Clockify API so the whole sweep
// — list, archive, delete, post-delete rescan, JSON summary, exit code — is
// exercised without touching live Clockify.
// ---------------------------------------------------------------------------

const testWorkspaceID = "000000000000000000000001"

// allKinds is every collection the sweeper touches, keyed by the fake-server
// route kind. The hyphenated kinds map to space-separated collection labels in
// the JSON summary; see kindToLabel.
var allKinds = []string{
	"scheduling-assignments", "time-off-requests", "time-off-policies",
	"expenses", "invoices", "webhooks", "user-groups", "holidays",
	"tags", "projects", "clients",
}

// kindToLabel maps a fake-server route kind to the collection label used as a
// key in cleanSummary's deleted/failed/leftovers maps.
var kindToLabel = map[string]string{
	"scheduling-assignments": "scheduling assignments",
	"time-off-requests":      "time-off requests",
	"time-off-policies":      "time-off policies",
	"expenses":               "expenses",
	"invoices":               "invoices",
	"webhooks":               "webhooks",
	"user-groups":            "user-groups",
	"holidays":               "holidays",
	"tags":                   "tags",
	"projects":               "projects",
	"clients":                "clients",
}

// prefixField is the object field the sweeper prefix-matches for a kind.
func prefixField(kind string) string {
	switch kind {
	case "scheduling-assignments", "time-off-requests":
		return "note"
	case "invoices":
		return "number"
	default:
		return "name"
	}
}

type fakeClockify struct {
	mu           sync.Mutex
	objects      map[string][]map[string]any
	deletes      map[string]int
	listMethod   map[string]string // kind -> last HTTP method used to list it
	dupPage      string            // kind whose list ignores pagination
	failList     string            // kind whose list returns HTTP 500
	failDelete   string            // kind whose DELETE returns HTTP 500
	keepOnDelete string            // kind whose DELETE returns 200 but keeps the object
}

func newFakeClockify() *fakeClockify {
	return &fakeClockify{
		objects:    map[string][]map[string]any{},
		deletes:    map[string]int{},
		listMethod: map[string]string{},
	}
}

func (f *fakeClockify) add(kind string, objs ...map[string]any) {
	f.objects[kind] = append(f.objects[kind], objs...)
}

// seedFamily adds one prefixed and one non-prefixed object to a kind so a
// sweep can be checked for both deletion and non-prefixed safety.
func (f *fakeClockify) seedFamily(kind, prefix string) {
	field := prefixField(kind)
	f.add(kind,
		map[string]any{"id": kind + "-pfx", field: prefix + " " + kind},
		map[string]any{"id": kind + "-keep", field: "keepme " + kind},
	)
}

func (f *fakeClockify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kind, id := routeKind(r.URL.Path)
	if kind == "" {
		http.NotFound(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		f.listMethod[kind] = r.Method
		f.serveList(w, r, kind)
	case http.MethodPut:
		w.WriteHeader(http.StatusOK) // archive: accept, empty body
	case http.MethodDelete:
		f.serveDelete(w, kind, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (f *fakeClockify) serveList(w http.ResponseWriter, r *http.Request, kind string) {
	if f.failList == kind {
		http.Error(w, "injected list failure", http.StatusInternalServerError)
		return
	}
	page, pageSize := listPageParams(r)
	all := f.objects[kind]
	window := pageSlice(all, page, pageSize)
	if f.dupPage == kind {
		// Ignore pagination entirely: always re-serve the first page.
		window = pageSlice(all, 1, pageSize)
	}
	writeJSON(w, wrapList(kind, window, len(all)))
}

func (f *fakeClockify) serveDelete(w http.ResponseWriter, kind, id string) {
	f.deletes[kind]++
	if f.failDelete == kind {
		http.Error(w, "injected delete failure", http.StatusInternalServerError)
		return
	}
	if f.keepOnDelete != kind {
		objs := f.objects[kind]
		for i, o := range objs {
			if o["id"] == id {
				f.objects[kind] = append(objs[:i:i], objs[i+1:]...)
				break
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

// surviving returns the ids still present in a kind after a sweep.
func (f *fakeClockify) surviving(kind string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, o := range f.objects[kind] {
		if id, _ := o["id"].(string); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// routeKind maps a request path under /workspaces/{ws}/ to a collection kind
// and an optional trailing object id.
func routeKind(path string) (kind, id string) {
	rest, ok := strings.CutPrefix(path, "/workspaces/"+testWorkspaceID+"/")
	if !ok {
		return "", ""
	}
	switch {
	case rest == "scheduling/assignments/all":
		return "scheduling-assignments", ""
	case strings.HasPrefix(rest, "scheduling/assignments/recurring/"):
		return "scheduling-assignments", strings.TrimPrefix(rest, "scheduling/assignments/recurring/")
	case strings.HasPrefix(rest, "scheduling/assignments/"):
		return "scheduling-assignments", strings.TrimPrefix(rest, "scheduling/assignments/")
	case rest == "time-off/requests":
		return "time-off-requests", ""
	case strings.HasPrefix(rest, "time-off/requests/"):
		return "time-off-requests", strings.TrimPrefix(rest, "time-off/requests/")
	case rest == "time-off/policies":
		return "time-off-policies", ""
	case strings.HasPrefix(rest, "time-off/policies/"):
		return "time-off-policies", strings.TrimPrefix(rest, "time-off/policies/")
	}
	seg := strings.SplitN(rest, "/", 2)
	switch seg[0] {
	case "expenses", "invoices", "webhooks", "user-groups", "holidays", "projects", "clients", "tags":
		if len(seg) == 2 {
			return seg[0], seg[1]
		}
		return seg[0], ""
	}
	return "", ""
}

func listPageParams(r *http.Request) (page, pageSize int) {
	page, pageSize = 1, 200
	if r.Method == http.MethodPost {
		var body struct {
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Page > 0 {
			page = body.Page
		}
		if body.PageSize > 0 {
			pageSize = body.PageSize
		}
		return page, pageSize
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && n > 0 {
		page = n
	}
	if n, err := strconv.Atoi(r.URL.Query().Get("page-size")); err == nil && n > 0 {
		pageSize = n
	}
	return page, pageSize
}

func pageSlice(all []map[string]any, page, pageSize int) []map[string]any {
	start := (page - 1) * pageSize
	if start >= len(all) {
		return []map[string]any{}
	}
	end := min(start+pageSize, len(all))
	return all[start:end]
}

// wrapList builds the per-kind list envelope: bare array for most collections,
// and the nested shapes Clockify actually returns for invoices, expenses,
// webhooks, and the POST-only time-off request search.
func wrapList(kind string, items []map[string]any, total int) any {
	switch kind {
	case "invoices":
		return map[string]any{"invoices": items}
	case "webhooks":
		return map[string]any{"webhooks": items}
	case "expenses":
		return map[string]any{"expenses": map[string]any{"expenses": items, "count": total}}
	case "time-off-requests":
		return map[string]any{"count": total, "requests": items}
	default:
		return items
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// startFake mounts a fake Clockify server and points run() at it through the
// confirmed-workspace environment.
func startFake(t *testing.T, fake *fakeClockify, prefix string, dryRun bool) {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("CLOCKIFY_API_KEY", "fake-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", testWorkspaceID)
	t.Setenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM", testWorkspaceID)
	t.Setenv("CLOCKIFY_LIVE_PREFIX", prefix)
	t.Setenv("CLOCKIFY_BASE_URL", server.URL)
	if dryRun {
		t.Setenv("CLOCKIFY_LIVE_CLEAN_DRY_RUN", "1")
	} else {
		t.Setenv("CLOCKIFY_LIVE_CLEAN_DRY_RUN", "")
	}
}

func runClean(t *testing.T) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	code := run(&buf)
	return code, buf.String()
}

// parseSummary decodes the machine-readable JSON summary, which the contract
// requires to be the final stdout line.
func parseSummary(t *testing.T, out string) cleanSummary {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatalf("no stdout produced")
	}
	last := lines[len(lines)-1]
	var summary cleanSummary
	if err := json.Unmarshal([]byte(last), &summary); err != nil {
		t.Fatalf("final stdout line is not valid JSON: %q (%v)", last, err)
	}
	return summary
}

func TestCleanRefusesWorkspaceMismatch(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "fake-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", testWorkspaceID)
	t.Setenv("CLOCKIFY_LIVE_PREFIX", "MCP-TEST")
	t.Setenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM", "a-different-workspace")
	t.Setenv("CLOCKIFY_LIVE_CLEAN_DRY_RUN", "")

	if code, _ := runClean(t); code != 2 {
		t.Fatalf("workspace mismatch exit code = %d, want 2", code)
	}
}

func TestCleanRefusesMissingEnv(t *testing.T) {
	t.Setenv("CLOCKIFY_API_KEY", "fake-key")
	t.Setenv("CLOCKIFY_WORKSPACE_ID", testWorkspaceID)
	t.Setenv("CLOCKIFY_LIVE_PREFIX", "")
	t.Setenv("CLOCKIFY_LIVE_WORKSPACE_CONFIRM", testWorkspaceID)
	t.Setenv("CLOCKIFY_LIVE_CLEAN_DRY_RUN", "")

	if code, _ := runClean(t); code != 2 {
		t.Fatalf("missing prefix exit code = %d, want 2", code)
	}
}

func TestCleanSweepsEveryFamilyAndSparesNonPrefixed(t *testing.T) {
	const prefix = "MCP-TEST-FAMILIES"
	fake := newFakeClockify()
	for _, kind := range allKinds {
		fake.seedFamily(kind, prefix)
	}
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 0 {
		t.Fatalf("clean exit code = %d, want 0\n%s", code, out)
	}
	summary := parseSummary(t, out)
	for _, kind := range allKinds {
		label := kindToLabel[kind]
		if got := summary.Deleted[label]; got != 1 {
			t.Errorf("%s: summary.deleted = %d, want 1", label, got)
		}
		survivors := fake.surviving(kind)
		if len(survivors) != 1 || survivors[0] != kind+"-keep" {
			t.Errorf("%s: survivors = %v, want only the non-prefixed object", kind, survivors)
		}
	}
	if total := sumCounts(summary.Failed); total != 0 {
		t.Errorf("summary.failed total = %d, want 0", total)
	}
	if total := sumCounts(summary.Leftovers); total != 0 {
		t.Errorf("summary.leftovers total = %d, want 0", total)
	}
}

func TestCleanDryRunDeletesNothing(t *testing.T) {
	const prefix = "MCP-TEST-DRY"
	fake := newFakeClockify()
	for _, kind := range allKinds {
		fake.seedFamily(kind, prefix)
	}
	startFake(t, fake, prefix, true)

	code, out := runClean(t)
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, want 0\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if !summary.DryRun {
		t.Error("summary.dry_run = false, want true")
	}
	if total := sumCounts(summary.Deleted); total != 0 {
		t.Errorf("summary.deleted total = %d, want 0 in dry-run", total)
	}
	for _, kind := range allKinds {
		if got := summary.Leftovers[kindToLabel[kind]]; got != 1 {
			t.Errorf("%s: dry-run leftovers = %d, want 1 (the would-delete count)", kind, got)
		}
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.deletes) != 0 {
		t.Fatalf("dry-run issued DELETE calls: %v", fake.deletes)
	}
}

func TestCleanDryRunFailsOnListError(t *testing.T) {
	const prefix = "MCP-TEST-DRYFAIL"
	fake := newFakeClockify()
	fake.seedFamily("projects", prefix)
	fake.failList = "tags"
	startFake(t, fake, prefix, true)

	code, out := runClean(t)
	if code != 1 {
		t.Fatalf("dry-run with a failed list exit code = %d, want 1\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if summary.Failed["tags"] == 0 {
		t.Errorf("expected a recorded tags list failure, got summary.failed=%v", summary.Failed)
	}
}

func TestCleanPaginatesBeyondPageSize(t *testing.T) {
	const prefix = "MCP-TEST-PAGE"
	fake := newFakeClockify()
	const count = 250 // forces a second page (page size is 200)
	for i := range count {
		fake.add("projects", map[string]any{"id": "proj-" + strconv.Itoa(i), "name": prefix + " " + strconv.Itoa(i)})
	}
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 0 {
		t.Fatalf("paginated clean exit code = %d, want 0\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if got := summary.Deleted["projects"]; got != count {
		t.Fatalf("summary.deleted[projects] = %d, want %d", got, count)
	}
	if survivors := fake.surviving("projects"); len(survivors) != 0 {
		t.Fatalf("%d projects survived a full paginated sweep", len(survivors))
	}
}

func TestCleanExitsNonZeroOnLeftovers(t *testing.T) {
	const prefix = "MCP-TEST-LEFT"
	fake := newFakeClockify()
	fake.seedFamily("projects", prefix)
	fake.keepOnDelete = "projects" // server acks the delete but keeps the object
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 1 {
		t.Fatalf("clean with post-rescan leftovers exit code = %d, want 1\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if summary.Leftovers["projects"] == 0 {
		t.Errorf("expected the post-delete rescan to record a projects leftover, got %v", summary.Leftovers)
	}
}

func TestCleanExitsNonZeroOnDeleteFailure(t *testing.T) {
	const prefix = "MCP-TEST-DELFAIL"
	fake := newFakeClockify()
	fake.seedFamily("tags", prefix)
	fake.failDelete = "tags"
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 1 {
		t.Fatalf("clean with a delete failure exit code = %d, want 1\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if summary.Failed["tags"] == 0 {
		t.Errorf("expected a recorded tags delete failure, got %v", summary.Failed)
	}
}

func TestCleanDetectsBrokenPagination(t *testing.T) {
	const prefix = "MCP-TEST-DUP"
	fake := newFakeClockify()
	// Exactly one full page of objects, served on every page request: the
	// sweeper must notice the repeat instead of looping to the page cap.
	for i := range 200 {
		fake.add("tags", map[string]any{"id": "tag-" + strconv.Itoa(i), "name": prefix + " " + strconv.Itoa(i)})
	}
	fake.dupPage = "tags"
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 1 {
		t.Fatalf("broken-pagination clean exit code = %d, want 1\n%s", code, out)
	}
	summary := parseSummary(t, out)
	if summary.Failed["tags"] == 0 {
		t.Errorf("expected the duplicate-page guard to fail the tags collection, got %v", summary.Failed)
	}
}

func TestCleanMatchesSchedulingAssignmentByLinkedProject(t *testing.T) {
	const prefix = "MCP-TEST-ASG"
	fake := newFakeClockify()
	fake.add("projects", map[string]any{"id": "proj-pfx", "name": prefix + " project"})
	// The matching assignment carries a deliberately non-prefixed projectName
	// and a null note, so only the linked projectId can match it.
	fake.add("scheduling-assignments",
		map[string]any{"id": "asg-linked", "projectId": "proj-pfx", "projectName": "Unrelated Display Name", "note": nil},
		map[string]any{"id": "asg-other", "projectId": "proj-other", "projectName": "Other", "note": nil},
	)
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 0 {
		t.Fatalf("assignment clean exit code = %d, want 0\n%s", code, out)
	}
	if survivors := fake.surviving("scheduling-assignments"); len(survivors) != 1 || survivors[0] != "asg-other" {
		t.Fatalf("scheduling-assignment survivors = %v, want only asg-other", survivors)
	}
	if survivors := fake.surviving("projects"); len(survivors) != 0 {
		t.Fatalf("prefixed project survived: %v", survivors)
	}
}

func TestCleanSweepsTimeOffRequestsViaPostSearch(t *testing.T) {
	const prefix = "MCP-TEST-TOR"
	fake := newFakeClockify()
	fake.add("time-off-requests",
		map[string]any{"id": "req-pfx", "note": prefix + " time off"},
		map[string]any{"id": "req-keep", "note": "unrelated note"},
	)
	startFake(t, fake, prefix, false)

	code, out := runClean(t)
	if code != 0 {
		t.Fatalf("time-off request clean exit code = %d, want 0\n%s", code, out)
	}
	fake.mu.Lock()
	method := fake.listMethod["time-off-requests"]
	fake.mu.Unlock()
	if method != http.MethodPost {
		t.Fatalf("time-off requests were listed via %q, want POST search", method)
	}
	if survivors := fake.surviving("time-off-requests"); len(survivors) != 1 || survivors[0] != "req-keep" {
		t.Fatalf("time-off request survivors = %v, want only req-keep", survivors)
	}
}
