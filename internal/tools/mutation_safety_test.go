package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/clockify"
	"github.com/apet97/go-clockify/internal/testclockify"
)

// TestUpdateMemberProfileRejectsImageConflict locks P2-5: image_url and
// remove_profile_image are mutually exclusive, so a payload that sets both —
// which leaves the upstream result undefined — is rejected before the write.
func TestUpdateMemberProfileRejectsImageConflict(t *testing.T) {
	fake := testclockify.NewServer("65b382b606de527a7ee2b60e")
	defer fake.Close()
	svc := New(clockify.NewClient("k", fake.URL, time.Second, 0), fake.WorkspaceID)

	_, err := svc.UpdateMemberProfile(context.Background(), map[string]any{
		"user_id":              "65b382b606de527a7ee2b622",
		"image_url":            "https://example.com/avatar.png",
		"remove_profile_image": true,
	})
	if err == nil {
		t.Fatal("expected an error when image_url and remove_profile_image are both set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error should explain the conflict: %v", err)
	}
}
