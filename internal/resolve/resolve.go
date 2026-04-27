package resolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

// maxIDLength caps any ID we'll bother validating. Real Clockify IDs are
// 24-char hex (BSON ObjectIDs). Anything longer than 128 bytes is either
// a fuzz pathological input or an attempt to wedge the rune loop, both
// of which we reject up front.
const maxIDLength = 128

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
	if len(ref) > maxIDLength {
		return fmt.Errorf("%s exceeds %d bytes", kind, maxIDLength)
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

func ResolveProjectID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	return resolveByNameOrID(ctx, client, "/workspaces/"+workspaceID+"/projects", ref, "project")
}

func ResolveClientID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	return resolveByNameOrID(ctx, client, "/workspaces/"+workspaceID+"/clients", ref, "client")
}

func ResolveTagID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
	return resolveByNameOrID(ctx, client, "/workspaces/"+workspaceID+"/tags", ref, "tag")
}

func ResolveUserID(ctx context.Context, client *clockify.Client, workspaceID, ref string) (string, error) {
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

	var users []map[string]any
	if err := client.Get(ctx, "/workspaces/"+workspaceID+"/users", map[string]string{"page-size": "200"}, &users); err != nil {
		return "", err
	}

	var matches []map[string]any
	isEmail := looksLikeEmail(ref)
	for _, user := range users {
		name, _ := user["name"].(string)
		email, _ := user["email"].(string)
		if isEmail {
			if strings.EqualFold(email, ref) {
				matches = append(matches, user)
			}
		} else {
			if strings.EqualFold(name, ref) || strings.EqualFold(email, ref) {
				matches = append(matches, user)
			}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("user '%s' not found. Use clockify_list_users to see available users", ref)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple users match '%s' (%d found). Use the full user ID instead", ref, len(matches))
	}
	id, _ := matches[0]["id"].(string)
	if id == "" {
		return "", fmt.Errorf("user response missing id")
	}
	return id, nil
}

func ResolveTaskID(ctx context.Context, client *clockify.Client, workspaceID, projectID, ref string) (string, error) {
	return resolveByNameOrID(ctx, client, "/workspaces/"+workspaceID+"/projects/"+projectID+"/tasks", ref, "task")
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

	items, err := clockify.ListAll[map[string]any](ctx, client, path, map[string]string{"name": ref, "strict-name-search": "true", "page-size": "50"})
	if err != nil {
		return "", err
	}

	matches := exactMatches(items, ref, "name")
	if len(matches) == 0 {
		return "", fmt.Errorf("%s '%s' not found. Use clockify_list_%ss to see available %ss", kind, ref, kind, kind)
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
