//go:build !postgres

package main

import (
	"strings"

	"github.com/apet97/go-clockify/internal/config"
)

func backendDoctorFindings(cfg config.Config) []doctorFinding {
	dsn := strings.TrimSpace(cfg.ControlPlaneDSN)
	message := "hosted backend checks require a postgres:// or postgresql:// control-plane DSN"
	if isDoctorPostgresDSN(dsn) {
		message = "--check-backends requires a binary built with -tags=postgres to verify Postgres reachability and migrations"
	}
	return []doctorFinding{{
		Severity: "ERROR",
		Key:      "MCP_CONTROL_PLANE_DSN",
		Message:  message,
	}}
}
