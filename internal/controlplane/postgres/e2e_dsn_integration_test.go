//go:build postgres && integration

package postgres_test

import (
	"os"
	"strings"
	"testing"
)

func e2eControlPlaneDSN(t *testing.T, _ string) string {
	t.Helper()
	if raw := strings.TrimSpace(os.Getenv("MCP_LIVE_CONTROL_PLANE_DSN")); raw != "" {
		return raw
	}
	return dsn(t)
}
