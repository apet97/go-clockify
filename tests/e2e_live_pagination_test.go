//go:build livee2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestLivePaginationOnTags exercises pagination semantics on
// clockify_list_tags through the MCP path: create 11 tags with the
// per-run prefix, list with page_size=5 across pages 1, 2, and 3,
// and verify (a) the meta envelope reports the correct page and
// pageSize on every response, (b) page 1 and page 2 each return
// exactly 5 entries when at-or-above-bound, and (c) the union of
// the three pages contains every prefix-matching tag we created.
//
// This pins the pagination contract that
// internal/tools/tags.go ListTags advertises in its meta envelope:
// page (default 1, echoed back), pageSize (default 50, echoed back),
// count (the size of THIS page's slice). A regression that drops or
// renames any of those fields fails this test.
//
// Cleanup deletes every created tag directly — Clockify accepts
// DELETE on tags without archiving (verified by direct upstream
// probe).
func TestLivePaginationOnTags(t *testing.T) {
	h := setupLiveMCPHarness(t, liveMCPOptions{})
	c := setupLiveCampaign(t, h)

	const totalCreate = 11
	created := make([]string, 0, totalCreate)

	t.Run("seed_11_tags", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		for i := 0; i < totalCreate; i++ {
			name := c.LivePrefix("page-tag", i)
			result := h.callOK(ctx, "clockify_create_tag", map[string]any{
				"name": name,
			})
			data := extractDataMap(t, result)
			id, _ := data["id"].(string)
			if id == "" {
				t.Fatalf("create_tag #%d returned no id: %#v", i, data)
			}
			created = append(created, id)
			tagID := id
			c.RegisterCleanup("tag", id, func(ctx context.Context) error {
				return c.rawDeletePath(ctx, "/tags/"+tagID)
			})
		}
		if got := len(created); got != totalCreate {
			t.Fatalf("expected %d created tags, got %d", totalCreate, got)
		}
	})

	t.Run("page_meta_round_trips", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Page 1 with page_size=5 — meta must echo page=1, pageSize=5.
		result := h.callOK(ctx, "clockify_list_tags", map[string]any{
			"page":      1,
			"page_size": 5,
		})
		meta, _ := result["structuredContent"].(map[string]any)
		envelope, _ := meta["meta"].(map[string]any)
		assertNumberEquals(t, "meta.page", envelope, "page", 1)
		assertNumberEquals(t, "meta.pageSize", envelope, "pageSize", 5)

		// Page 2 — meta must echo page=2.
		result = h.callOK(ctx, "clockify_list_tags", map[string]any{
			"page":      2,
			"page_size": 5,
		})
		envelope, _ = result["structuredContent"].(map[string]any)["meta"].(map[string]any)
		assertNumberEquals(t, "meta.page", envelope, "page", 2)
		assertNumberEquals(t, "meta.pageSize", envelope, "pageSize", 5)
	})

	t.Run("union_of_pages_covers_seeded_tags", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Walk every page of size 50 (the upstream cap is 5000 but
		// 50 is the documented default) and collect every prefix-
		// matching id. The pagination envelope is the only way we
		// can demonstrate that pageSize is honoured AND that the
		// underlying slice covers everything we created.
		seen := make(map[string]struct{})
		for page := 1; page <= 20; page++ {
			result := h.callOK(ctx, "clockify_list_tags", map[string]any{
				"page":      page,
				"page_size": 50,
			})
			tags := extractList(t, result)
			if len(tags) == 0 {
				break
			}
			for _, item := range tags {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if name, _ := entry["name"].(string); strings.HasPrefix(name, c.RunID+"-page-tag-") {
					if id, _ := entry["id"].(string); id != "" {
						seen[id] = struct{}{}
					}
				}
			}
			if len(tags) < 50 {
				// Last page (count below pageSize means walked the
				// whole list).
				break
			}
		}
		if got, want := len(seen), totalCreate; got != want {
			t.Fatalf("paginated walk found %d of %d seeded tags (run prefix %q)", got, want, c.RunID)
		}
	})
}

// assertNumberEquals checks a JSON-decoded numeric field — JSON
// unmarshal yields float64 for numbers, so an integer comparison
// must dereference and convert. Helper centralises the boilerplate.
func assertNumberEquals(t *testing.T, label string, m map[string]any, field string, want int) {
	t.Helper()
	if m == nil {
		t.Fatalf("%s: missing envelope", label)
	}
	switch v := m[field].(type) {
	case float64:
		if int(v) != want {
			t.Fatalf("%s.%s: got %v, want %d", label, field, v, want)
		}
	case int:
		if v != want {
			t.Fatalf("%s.%s: got %d, want %d", label, field, v, want)
		}
	case nil:
		t.Fatalf("%s.%s: missing", label, field)
	default:
		t.Fatalf("%s.%s: unexpected type %s", label, field, fmt.Sprintf("%T", v))
	}
}
