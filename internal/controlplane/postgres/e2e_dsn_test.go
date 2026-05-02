//go:build postgres && !integration

package postgres_test

import (
	"os"
	"strings"
	"testing"
)

func e2eControlPlaneDSN(t *testing.T, skipMsg string) string {
	t.Helper()
	if raw := strings.TrimSpace(os.Getenv("MCP_LIVE_CONTROL_PLANE_DSN")); raw != "" {
		return raw
	}
	if os.Getenv("INTEGRATION_REQUIRED") == "1" {
		t.Fatal("MCP_LIVE_CONTROL_PLANE_DSN unset under INTEGRATION_REQUIRED=1")
	}
	t.Skip(skipMsg)
	return ""
}
