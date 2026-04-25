//go:build postgres

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apet97/go-clockify/internal/config"
	"github.com/apet97/go-clockify/internal/controlplane"
)

type backendDoctorChecker interface {
	DoctorCheck(context.Context) error
}

func backendDoctorFindings(cfg config.Config) []doctorFinding {
	var findings []doctorFinding
	add := func(key, message string) {
		findings = append(findings, doctorFinding{
			Severity: "ERROR",
			Key:      key,
			Message:  message,
		})
	}

	dsn := strings.TrimSpace(cfg.ControlPlaneDSN)
	if !isDoctorPostgresDSN(dsn) {
		add("MCP_CONTROL_PLANE_DSN", "--check-backends requires a postgres:// or postgresql:// control-plane DSN")
		return findings
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	store, err := controlplane.Open(dsn, controlplane.WithAuditCap(cfg.ControlPlaneAuditCap))
	if err != nil {
		add("MCP_CONTROL_PLANE_DSN", fmt.Sprintf("backend check failed opening control-plane store: %v", err))
		return findings
	}
	defer func() {
		if err := store.Close(); err != nil {
			add("MCP_CONTROL_PLANE_DSN", fmt.Sprintf("backend check failed closing control-plane store: %v", err))
		}
	}()

	checker, ok := store.(backendDoctorChecker)
	if !ok {
		add("MCP_CONTROL_PLANE_DSN", "postgres backend opened but does not expose doctor backend checks")
		return findings
	}
	if err := checker.DoctorCheck(ctx); err != nil {
		add("MCP_CONTROL_PLANE_DSN", fmt.Sprintf("postgres backend health check failed: %v", err))
	}
	return findings
}
