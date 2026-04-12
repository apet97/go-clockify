package helpers

import (
	"fmt"
	"reflect"
)

// ErrorMessage maps an HTTP status code to an actionable message for the LLM.
// The body is trimmed to 500 characters max.
func ErrorMessage(statusCode int, body string) string {
	if len(body) > 500 {
		body = body[:500]
	}

	switch {
	case statusCode == 401:
		return "Authentication failed. Verify your CLOCKIFY_API_KEY is correct."
	case statusCode == 403:
		return "Permission denied. You may need workspace admin access."
	case statusCode == 404:
		return "Not found. Check that the ID exists and use the corresponding list tool to find valid IDs."
	case statusCode == 429:
		return "Rate limit exceeded. Wait a moment and retry."
	case statusCode >= 400 && statusCode < 500:
		return fmt.Sprintf("Client error (HTTP %d): %s", statusCode, body)
	case statusCode >= 500 && statusCode < 600:
		return fmt.Sprintf("Clockify server error (HTTP %d): %s", statusCode, body)
	default:
		return fmt.Sprintf("Unexpected error (HTTP %d): %s", statusCode, body)
	}
}

// PaginatedResult builds a standard paginated response map.
func PaginatedResult(items any, page, pageSize int, entityName string, hasMore bool) map[string]any {
	count := 0
	if items != nil {
		v := reflect.ValueOf(items)
		if v.Kind() == reflect.Slice {
			count = v.Len()
		}
	}

	return map[string]any{
		entityName:  items,
		"count":     count,
		"page":      page,
		"page_size": pageSize,
		"has_more":  hasMore,
	}
}
