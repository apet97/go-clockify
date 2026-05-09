package harness

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeHTTPErrorResponse(status int, body []byte) Response {
	var out Response
	if err := json.Unmarshal(body, &out); err == nil && out.Error != nil {
		return out
	}
	msg := fmt.Sprintf("http status %d", status)
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		msg += " body=" + trimmed
	}
	return Response{Error: &RPCError{Code: -32603, Message: msg}}
}
