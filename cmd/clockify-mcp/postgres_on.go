//go:build postgres

package main

// Side-import of the Postgres control-plane sub-module. This is the
// ONLY main-module file that references it; the //go:build postgres
// tag ensures the default build never links pgx. The top-level go.mod
// has zero github.com/jackc/pgx rows and the nm-gate in
// scripts/check-build-tags.sh asserts the symbol absence (ADR 0001).
//
// Importing the package runs its init(), which calls
// controlplane.RegisterOpener("postgres", …). After this, a
// controlplane.Open() call with a "postgres://" DSN dispatches to the
// pgx-backed Store. See ADR 0011.
import _ "github.com/apet97/go-clockify/internal/controlplane/postgres"
