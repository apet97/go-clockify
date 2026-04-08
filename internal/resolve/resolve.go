package resolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/apet97/go-clockify/internal/clockify"
)

func ValidateID(id, name string) error {
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
	if err := ValidateID(ref, "user"); err != nil {
		return "", err
	}
	if looksLikeClockifyID(ref) {
		return ref, nil
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
	if err := ValidateID(ref, kind); err != nil {
		return "", err
	}
	if looksLikeClockifyID(ref) {
		return ref, nil
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
