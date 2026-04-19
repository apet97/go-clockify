package runtime

import (
	"context"
	"os"

	"github.com/apet97/go-clockify/internal/mcp"
)

// runStdio is the default transport: read JSON-RPC frames from stdin,
// write responses to stdout. No listener, no auth — one client per
// process lifetime. The ctx is honoured by server.Run for shutdown.
func (r *Runtime) runStdio(ctx context.Context, server *mcp.Server) error {
	return server.Run(ctx, os.Stdin, os.Stdout)
}
