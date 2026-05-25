package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/mcp"
)

// testTagID is a 24-char hex value that the resolver treats as a
// Clockify ObjectID — resolution short-circuits to the GET-by-ID path
// without a list lookup.
const testTagID = "6b00f6bc2568d3d293061e3b"

func TestCreateTagNameLengthLimit(t *testing.T) {
	svc := New(nil, "ws1")
	longName := strings.Repeat("a", 101)
	if _, err := svc.CreateTag(context.Background(), map[string]any{"name": longName}); err == nil {
		t.Fatal("expected error for name > 100 chars")
	} else if !strings.Contains(err.Error(), "100") {
		t.Fatalf("expected error to mention 100, got: %v", err)
	}
}

func TestTagListSchemaProperties(t *testing.T) {
	svc := New(clockify.NewClient("k", "https://api.clockify.me/api/v1", 5*time.Second, 0), "ws1")
	for _, d := range mustRegistry(t, svc) {
		if d.Tool.Name != "clockify_tags_list" {
			continue
		}
		props, ok := d.Tool.InputSchema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("clockify_tags_list properties = %T", d.Tool.InputSchema["properties"])
		}
		for _, prop := range []string{"page", "page_size", "name", "strict_name_search", "excluded_ids", "archived", "sort_column", "sort_order"} {
			if _, ok := props[prop]; !ok {
				t.Fatalf("clockify_tags_list missing property %s", prop)
			}
		}
		return
	}
	t.Fatal("missing descriptor for clockify_tags_list")
}

func TestListTagsFiltersForwarded(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/tags" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{
			"name":               "billable",
			"archived":           "true",
			"strict-name-search": "true",
			"sort-column":        "NAME",
			"sort-order":         "ASCENDING",
			"page":               "2",
			"page-size":          "50",
		}
		for key, value := range want {
			if got := q.Get(key); got != value {
				t.Fatalf("query %s = %q, want %q (raw=%s)", key, got, value, r.URL.RawQuery)
			}
		}
		if got := q["excluded-ids"]; len(got) != 2 || got[0] != "tag-a" || got[1] != "tag-b" {
			t.Fatalf("query excluded-ids = %#v, want [tag-a tag-b] (raw=%s)", got, r.URL.RawQuery)
		}
		respondJSON(t, w, []clockify.Tag{{ID: testTagID, Name: "billable", Archived: true}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.ListTags(context.Background(), map[string]any{
		"name":               "billable",
		"archived":           true,
		"strict_name_search": true,
		"excluded_ids":       []string{"tag-a", "tag-b"},
		"sort_column":        "NAME",
		"sort_order":         "ASCENDING",
		"page":               2,
		"page_size":          50,
	}); err != nil {
		t.Fatalf("ListTags: %v", err)
	}
}

func TestTagsListWrapperFiltersForwarded(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/workspaces/ws1/tags" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{
			"name":               "billable",
			"archived":           "true",
			"strict-name-search": "true",
			"sort-column":        "NAME",
			"sort-order":         "ASCENDING",
			"page":               "2",
			"page-size":          "50",
		}
		for key, value := range want {
			if got := q.Get(key); got != value {
				t.Fatalf("query %s = %q, want %q (raw=%s)", key, got, value, r.URL.RawQuery)
			}
		}
		if got := q["excluded-ids"]; len(got) != 2 || got[0] != "tag-a" || got[1] != "tag-b" {
			t.Fatalf("query excluded-ids = %#v, want [tag-a tag-b] (raw=%s)", got, r.URL.RawQuery)
		}
		respondJSON(t, w, []clockify.Tag{{ID: testTagID, Name: "billable", Archived: true}})
	})
	defer cleanup()

	svc := New(client, "ws1")
	if _, err := svc.TagsList(context.Background(), map[string]any{
		"name":               "billable",
		"archived":           true,
		"strict_name_search": true,
		"excluded_ids":       []string{"tag-a", "tag-b"},
		"sort_column":        "NAME",
		"sort_order":         "ASCENDING",
		"page":               2,
		"page_size":          50,
	}); err != nil {
		t.Fatalf("TagsList: %v", err)
	}
}

func TestGetTagByID(t *testing.T) {
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/workspaces/ws1/tags/"+testTagID && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "billable", Archived: false})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.GetTag(context.Background(), map[string]any{
		"tag": testTagID,
	})
	if err != nil {
		t.Fatalf("get tag failed: %v", err)
	}
	tag, ok := result.Data.(clockify.Tag)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if tag.ID != testTagID || tag.Name != "billable" {
		t.Fatalf("unexpected tag: %+v", tag)
	}
	if result.Meta["workspaceId"] != "ws1" || result.Meta["tagId"] != testTagID {
		t.Fatalf("unexpected meta: %+v", result.Meta)
	}
}

func TestGetTagRequiresTag(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.GetTag(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when tag is missing")
	} else if !strings.Contains(err.Error(), "tag") {
		t.Fatalf("expected error to mention tag, got %v", err)
	}
}

func TestUpdateTagFetchThenMerge(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "old-name", Archived: false})
		case r.URL.Path == tagPath && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "new-name", Archived: false})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateTag(context.Background(), map[string]any{
		"tag":  testTagID,
		"name": "new-name",
	})
	if err != nil {
		t.Fatalf("update tag failed: %v", err)
	}
	if putBody["name"] != "new-name" {
		t.Fatalf("PUT body must carry new name, got %v", putBody["name"])
	}
	changed, _ := result.Meta["changedFields"].([]string)
	if len(changed) != 1 || changed[0] != "name" {
		t.Fatalf("expected changedFields=[name], got %v", changed)
	}
}

func TestUpdateTagPreservesArchivedFalse(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "existing", Archived: false})
		case r.URL.Path == tagPath && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "existing", Archived: false})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	// no changes requested — PUT must still carry name and archived:false
	if _, err := svc.UpdateTag(context.Background(), map[string]any{
		"tag": testTagID,
	}); err != nil {
		t.Fatalf("update tag failed: %v", err)
	}
	if putBody["name"] != "existing" {
		t.Fatalf("PUT body must preserve existing name, got %v", putBody["name"])
	}
	// archived:false must be sent explicitly (not omitted)
	archived, hasArchived := putBody["archived"]
	if !hasArchived {
		t.Fatal("PUT body must include archived field even when false")
	}
	if archived != false {
		t.Fatalf("archived must be false, got %v", archived)
	}
}

func TestUpdateTagArchivedToggle(t *testing.T) {
	var putBody map[string]any
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "my-tag", Archived: false})
		case r.URL.Path == tagPath && r.Method == http.MethodPut:
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode put body: %v", err)
			}
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "my-tag", Archived: true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateTag(context.Background(), map[string]any{
		"tag":      testTagID,
		"archived": true,
	})
	if err != nil {
		t.Fatalf("update tag with archived toggle failed: %v", err)
	}
	if putBody["archived"] != true {
		t.Fatalf("PUT body must carry archived:true, got %v", putBody["archived"])
	}
	changed, _ := result.Meta["changedFields"].([]string)
	if len(changed) != 1 || changed[0] != "archived" {
		t.Fatalf("expected changedFields=[archived], got %v", changed)
	}
}

func TestUpdateTagDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "existing"})
		case r.URL.Path == tagPath && r.Method == http.MethodPut:
			mutated = true
			respondJSON(t, w, map[string]any{})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.UpdateTag(context.Background(), map[string]any{
		"tag":     testTagID,
		"name":    "new-name",
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("update tag dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue PUT")
	}
	if result.Action != "clockify_tags_update" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestUpdateTagRequiresTag(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.UpdateTag(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when tag is missing")
	}
}

func TestDeleteTagDeletesDirectly(t *testing.T) {
	var deleted bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "old-tag", Archived: false})
		case r.URL.Path == tagPath && r.Method == http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteTag(context.Background(), map[string]any{
		"tag": testTagID,
	})
	if err != nil {
		t.Fatalf("delete tag failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected DELETE call")
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data type: %T", result.Data)
	}
	if data["deleted"] != true || data["tagId"] != testTagID {
		t.Fatalf("unexpected response data: %+v", data)
	}
}

func TestDeleteTagDryRunDoesNotMutate(t *testing.T) {
	var mutated bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "old-tag"})
		case r.URL.Path == tagPath && r.Method == http.MethodDelete:
			mutated = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	result, err := svc.DeleteTag(context.Background(), map[string]any{
		"tag":     testTagID,
		"dry_run": true,
	})
	if err != nil {
		t.Fatalf("delete tag dry-run failed: %v", err)
	}
	if mutated {
		t.Fatal("dry-run must not issue DELETE")
	}
	if result.Action != "clockify_tags_delete" {
		t.Fatalf("unexpected action: %q", result.Action)
	}
}

func TestTagsDeleteDryRunStandardizedResultHasEmptyChanged(t *testing.T) {
	var deleted bool
	client, cleanup := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		tagPath := "/workspaces/ws1/tags/" + testTagID
		switch {
		case r.URL.Path == tagPath && r.Method == http.MethodGet:
			respondJSON(t, w, clockify.Tag{ID: testTagID, Name: "old-tag"})
		case r.URL.Path == tagPath && r.Method == http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer cleanup()

	svc := New(client, "ws1")
	deleteTool := findToolDescriptor(t, svc, "clockify_tags_delete")
	dry, err := deleteTool.Handler(context.Background(), map[string]any{"tag": testTagID, "dry_run": true})
	if err != nil {
		t.Fatalf("dry-run handler: %v", err)
	}
	if deleted {
		t.Fatal("dry-run must not issue DELETE")
	}
	dryResult, ok := dry.(ToolResult)
	if !ok {
		t.Fatalf("dry-run result type = %T, want ToolResult", dry)
	}
	if len(dryResult.Changed.Deleted) != 0 || len(dryResult.Changed.Created) != 0 || len(dryResult.Changed.Updated) != 0 {
		t.Fatalf("dry-run changed must be empty, got %#v", dryResult.Changed)
	}

	real, err := deleteTool.Handler(context.Background(), map[string]any{"tag": testTagID})
	if err != nil {
		t.Fatalf("real delete handler: %v", err)
	}
	realResult, ok := real.(ToolResult)
	if !ok {
		t.Fatalf("real delete result type = %T, want ToolResult", real)
	}
	if len(realResult.Changed.Deleted) != 1 || realResult.Changed.Deleted[0].ID != testTagID {
		t.Fatalf("real delete should report changed.deleted receipt, got %#v", realResult.Changed)
	}
}

func TestDeleteTagRequiresTag(t *testing.T) {
	svc := New(nil, "ws1")
	if _, err := svc.DeleteTag(context.Background(), map[string]any{}); err == nil {
		t.Fatal("expected error when tag is missing")
	}
}

func findToolDescriptor(t *testing.T, svc *Service, name string) mcp.ToolDescriptor {
	t.Helper()
	for _, descriptor := range mustRegistry(t, svc) {
		if descriptor.Tool.Name == name {
			return descriptor
		}
	}
	t.Fatalf("missing descriptor for %s", name)
	return mcp.ToolDescriptor{}
}
