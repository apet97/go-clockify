//go:build postgres

package postgres

import (
	"github.com/apet97/go-clockify/internal/controlplane"
)

// init registers the Postgres opener with the parent package's
// dispatch table. After this runs, controlplane.Open can resolve the
// "postgres" and "postgresql" DSN schemes. Registration panics on
// double-register; a second init (e.g. a mis-wired test harness)
// surfaces the bug at startup.
func init() {
	controlplane.RegisterOpener("postgres", open)
	controlplane.RegisterOpener("postgresql", open)
}
