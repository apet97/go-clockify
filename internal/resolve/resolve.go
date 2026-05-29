// Package resolve centralises Clockify entity resolution and ID
// validation. Tool handlers in internal/tools call ValidateID before
// any path concatenation and the Resolve*ID helpers below to map
// caller-supplied "name or ID" arguments to canonical Clockify IDs.
// Static gates (TestPathSafety_* in internal/tools and this package)
// enforce the validation discipline at build time.
package resolve

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

// workspacePath builds "/workspaces/<wsID>[/sub1/sub2...]" with the same
// safety contract as paths.Workspace: workspaceID is re-validated via
// ValidateID and every sub-segment is percent-encoded via url.PathEscape.
//
// internal/paths cannot be imported here because paths already depends on
// this package for ValidateID; duplicating the tiny validate+escape pattern
// gives the five Resolve*ID helpers below the same belt-and-braces drift
// protection that internal/tools/* gets from paths.Workspace. Keep the two
// helpers in sync. Audit finding 2026-05-29 S6.
func workspacePath(workspaceID string, sub ...string) (string, error) {
	if err := ValidateID(workspaceID, "workspace_id"); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("/workspaces/")
	b.WriteString(url.PathEscape(workspaceID))
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

// maxIDLength caps an entity ID validated for path-segment use. Real
// Clockify IDs are usually 24-char hex BSON ObjectIDs; some integration IDs
// are 36-char UUIDs. Keep the boundary tight so pasted tokens/key fragments
// fail before path construction.
const maxIDLength = 36

// maxNameRefLength caps a name-or-ID reference accepted by ValidateNameRef
// for name-lookup resolution. Names are free-text workspace, project, and
// client labels, so this is far more permissive than maxIDLength; it only
// rejects pathologically long input.
const maxNameRefLength = 128

// ValidateID checks that id is suitable for use as a path segment in a
// Clockify request URL. The character class is intentionally narrower
// than the full Clockify ID surface so a pasted token or key fragment
// fails before path construction: "/", "?", "#", "%", control bytes,
// "..", empty/whitespace, and lengths over maxIDLength are rejected.
// name appears verbatim in the returned error so the caller learns
// which field was invalid (e.g. "project_id").
func ValidateID(id, name string) error {
	if len(id) > maxIDLength {
		return fmt.Errorf("%s exceeds %d bytes", name, maxIDLength)
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	if strings.ContainsAny(id, "/?#%") {
		return fmt.Errorf("%s contains invalid characters", name)
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("%s contains invalid characters", name)
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("%s contains invalid characters", name)
		}
	}
	return nil
}

// ValidateNameRef is the permissive sibling of ValidateID, used for
// name-or-ID inputs that resolve to a Clockify entity by name. Names
// reach the Clockify API as URL query-parameter values (which url.Values
// safely percent-encodes) — never as path segments — so the strict
// path-safety character class ValidateID enforces would over-reject
// legitimate workspace names like "ACME / Support" or "R&D 50%".
//
// The contract here mirrors the API surface: anything that's not a
// control byte and isn't pathologically long is accepted; the caller
// is expected to resolve it via name lookup, then validate the
// resulting Clockify ID with ValidateID before any path use.
func ValidateNameRef(ref, kind string) error {
	if len(ref) > maxNameRefLength {
		return fmt.Errorf("%s exceeds %d bytes", kind, maxNameRefLength)
	}
	if strings.TrimSpace(ref) == "" {
		return fmt.Errorf("%s cannot be empty", kind)
	}
	for _, r := range ref {
		if r < 0x20 || r == 0x7F {
			return fmt.Errorf("%s contains invalid control characters", kind)
		}
	}
	return nil
}

// ProjectID returns the Clockify project ID for ref, which may
// be either a canonical project ID (returned verbatim after validation)
// or a workspace-local project name (resolved via case-insensitive
// strict-name search against the workspace's project list). Returns an
// error when the ref is unknown, ambiguous (multiple matches), or
// fails ID validation when treated as an ID.
func ProjectID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	path, err := workspacePath(workspaceID, "projects")
	if err != nil {
		return "", err
	}
	return resolveByNameOrID(ctx, client, path, ref, "project")
}

// ClientID returns the Clockify client ID for ref, which may
// be a canonical client ID or a workspace-local client name. Same
// resolution and error contract as ProjectID.
func ClientID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	path, err := workspacePath(workspaceID, "clients")
	if err != nil {
		return "", err
	}
	return resolveByNameOrID(ctx, client, path, ref, "client")
}

// TagID returns the Clockify tag ID for ref, which may be a
// canonical tag ID or a workspace-local tag name. Same resolution and
// error contract as ProjectID.
func TagID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	path, err := workspacePath(workspaceID, "tags")
	if err != nil {
		return "", err
	}
	return resolveByNameOrID(ctx, client, path, ref, "tag")
}

// UserID returns the Clockify user ID for ref. ref may be a
// canonical user ID (returned verbatim after ValidateID), an email
// address (matched against the user record's email field with
// case-insensitive equality), or a display name (matched against the
// name field with strict-name-search semantics). Email matches use
// the Clockify "email=" query parameter and name matches use "name="
// with strict-name-search=true to avoid prefix collisions. Returns an
// error when the ref is unknown or ambiguous.
func UserID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	if looksLikeClockifyID(ref) {
		// Strict path-segment validation only when the input is being
		// returned verbatim for use in a URL path.
		if err := ValidateID(ref, "user"); err != nil {
			return "", err
		}
		return ref, nil
	}
	// Otherwise the ref is a name or email going to a query-parameter
	// (clockify.Client URL-encodes those), so the strict ID character
	// class would over-reject legitimate values.
	if err := ValidateNameRef(ref, "user"); err != nil {
		return "", err
	}

	const pageSize = 200
	isEmail := looksLikeEmail(ref)
	path, err := workspacePath(workspaceID, "users")
	if err != nil {
		return "", err
	}
	query := map[string]string{"page-size": strconv.Itoa(pageSize)}
	if isEmail {
		query["email"] = ref
	} else {
		query["name"] = ref
		query["strict-name-search"] = "true"
	}
	matches := 0
	matchedID := ""
	for page := 1; ; page++ {
		query["page"] = strconv.Itoa(page)
		var users []map[string]any
		if err := client.Get(ctx, path, query, &users); err != nil {
			return "", err
		}

		for _, user := range users {
			name, _ := user["name"].(string)
			email, _ := user["email"].(string)
			matched := false
			if isEmail {
				if strings.EqualFold(email, ref) {
					matched = true
				}
			} else {
				if strings.EqualFold(name, ref) || strings.EqualFold(email, ref) {
					matched = true
				}
			}
			if !matched {
				continue
			}
			matches++
			if matches > 1 {
				return "", fmt.Errorf("multiple users match '%s' (%d found). Use the full user ID instead", ref, matches)
			}
			id, _ := user["id"].(string)
			if id == "" {
				return "", fmt.Errorf("user response missing id")
			}
			matchedID = id
		}
		if len(users) == 0 || len(users) < pageSize {
			break
		}
		if page >= 1000 {
			return "", fmt.Errorf("pagination safety stop reached: path=%s page-size=%d", path, pageSize)
		}
	}
	if matchedID != "" {
		return matchedID, nil
	}
	return "", fmt.Errorf("user '%s' not found. Use clockify_list_users to see available users", ref)
}

// TaskID returns the Clockify task ID for ref under the given
// project. ref may be a canonical task ID or a project-local task
// name. projectID must already be a validated Clockify project ID
// (e.g. the result of ProjectID). Same resolution and error
// contract as ProjectID.
func TaskID(ctx context.Context, client *clockify.Client, workspaceID, projectID, ref string) (string, error) {
	path, err := workspacePath(workspaceID, "projects", projectID, "tasks")
	if err != nil {
		return "", err
	}
	return resolveByNameOrID(ctx, client, path, ref, "task")
}

func resolveByNameOrID(ctx context.Context, client *clockify.Client, path, ref, kind string) (string, error) {
	if looksLikeClockifyID(ref) {
		// Path-segment use: strict validation. Real Clockify IDs always
		// pass this; the check is here so a 24-char-shaped value that
		// nonetheless contains a "/" or control byte is rejected before
		// it can reach a URL path.
		if err := ValidateID(ref, kind); err != nil {
			return "", err
		}
		return ref, nil
	}
	// Name path: query-parameter use, so url.Values handles encoding
	// safely. ValidateID's character class would reject legitimate
	// names like "ACME / Support" or "R&D 50%"; ValidateNameRef only
	// blocks empty / oversized / control-byte strings.
	if err := ValidateNameRef(ref, kind); err != nil {
		return "", err
	}

	items, err := clockify.ListAll[map[string]any](ctx, client, path, map[string]string{"name": ref, "strict-name-search": "true", "page-size": "200"})
	if err != nil {
		return "", err
	}

	matches := exactMatches(items, ref, "name")
	if len(matches) == 0 {
		if kind == "project" {
			return "", fmt.Errorf("project '%s' not found. List existing projects with clockify_projects_list, or create one with clockify_create_work_package and retry with the returned project_id", ref)
		}
		return "", fmt.Errorf("%s '%s' not found. Use clockify_%ss_list to see available %ss", kind, ref, kind, kind)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple %ss match '%s' (%d found). Use the full %s ID instead", kind, ref, len(matches), kind)
	}
	id, _ := matches[0]["id"].(string)
	if id == "" {
		return "", fmt.Errorf("%s response missing id", kind)
	}
	return id, nil
}

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at < 1 || at >= len(s)-1 {
		return false
	}
	dot := strings.LastIndexByte(s[at:], '.')
	return dot > 1
}

func looksLikeClockifyID(value string) bool {
	if len(value) != 24 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

func exactMatches(items []map[string]any, candidate, field string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, item := range items {
		v, _ := item[field].(string)
		if strings.EqualFold(v, candidate) {
			out = append(out, item)
		}
	}
	return out
}
