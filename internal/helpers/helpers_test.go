package helpers

import (
	"strings"
	"testing"
)

func TestErrorMessage401(t *testing.T) {
	msg := ErrorMessage(401, "")
	if msg != "Authentication failed. Verify your CLOCKIFY_API_KEY is correct." {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestErrorMessage403(t *testing.T) {
	msg := ErrorMessage(403, "")
	if msg != "Permission denied. You may need workspace admin access." {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestErrorMessage404(t *testing.T) {
	msg := ErrorMessage(404, "")
	if msg != "Not found. Check that the ID exists and use the corresponding list tool to find valid IDs." {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestErrorMessage429(t *testing.T) {
	msg := ErrorMessage(429, "")
	if msg != "Rate limit exceeded. Wait a moment and retry." {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestErrorMessage500(t *testing.T) {
	msg := ErrorMessage(500, "internal error")
	want := "Clockify server error (HTTP 500): internal error"
	if msg != want {
		t.Fatalf("got %q, want %q", msg, want)
	}
}

func TestErrorMessageTruncatesBody(t *testing.T) {
	longBody := strings.Repeat("x", 1000)
	msg := ErrorMessage(500, longBody)
	// The body portion should be 500 chars, not 1000.
	want := "Clockify server error (HTTP 500): " + strings.Repeat("x", 500)
	if msg != want {
		t.Fatalf("body not truncated: got len %d", len(msg))
	}
}

func TestPaginatedResult(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := PaginatedResult(items, 1, 50, "projects", true)

	if result["count"] != 3 {
		t.Fatalf("expected count 3, got %v", result["count"])
	}
	if result["page"] != 1 {
		t.Fatalf("expected page 1, got %v", result["page"])
	}
	if result["page_size"] != 50 {
		t.Fatalf("expected page_size 50, got %v", result["page_size"])
	}
	if result["has_more"] != true {
		t.Fatalf("expected has_more true, got %v", result["has_more"])
	}
	if result["projects"] == nil {
		t.Fatal("expected projects key")
	}
}

func TestPaginatedResultNilItems(t *testing.T) {
	result := PaginatedResult(nil, 1, 50, "entries", false)

	if result["count"] != 0 {
		t.Fatalf("expected count 0 for nil items, got %v", result["count"])
	}
	if result["entries"] != nil {
		t.Fatalf("expected nil entries, got %v", result["entries"])
	}
}
