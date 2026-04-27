// Package paths centralises Clockify URL path construction for handlers.
//
// clockify.Client.doOnce concatenates baseURL + path without any
// per-segment escaping; every ID-bearing segment must therefore be
// validated by the caller. The static gate
// TestPathSafety_HandlersValidateIDsBeforeConcat
// (internal/tools/path_safety_test.go) enforces that every handler
// file calls resolve.ValidateID or a resolve.Resolve*ID helper before
// concatenating non-workspace IDs. This package adds the next layer:
// a single typed builder so the discipline can shift from "every
// handler writes its own concat" to "every handler calls Workspace()
// and lets the helper escape and validate centrally".
//
// Audit finding 3 follow-up — see the "Out of scope" section of
// /Users/15x/.claude/plans/ultrathink-and-validate-findings-delegated-catmull.md.
package paths

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/apet97/go-clockify/internal/resolve"
)

// Workspace returns "/workspaces/<wsID>[/sub1/sub2/...]" with the
// workspace ID validated via resolve.ValidateID and every sub-segment
// percent-encoded via url.PathEscape.
//
// Empty sub-segments are rejected — they collapse into "//" which
// most servers (including Clockify) treat as a path canonicalisation
// quirk; refuse them at construction time so the caller catches the
// bug locally rather than diagnosing a 404 later. Sub-segments that
// already contain a "/" are likewise rejected: they almost always
// indicate the caller forgot to split a path or accidentally passed a
// raw URL fragment.
func Workspace(wsID string, sub ...string) (string, error) {
	if err := resolve.ValidateID(wsID, "workspace_id"); err != nil {
		return "", err
	}
	if len(sub) == 0 {
		return "/workspaces/" + url.PathEscape(wsID), nil
	}
	var b strings.Builder
	b.WriteString("/workspaces/")
	b.WriteString(url.PathEscape(wsID))
	for i, seg := range sub {
		if seg == "" {
			return "", fmt.Errorf("path segment %d is empty", i)
		}
		if strings.Contains(seg, "/") {
			return "", fmt.Errorf("path segment %d contains a slash: %q", i, seg)
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(seg))
	}
	return b.String(), nil
}
