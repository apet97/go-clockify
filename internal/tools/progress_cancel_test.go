package tools

import (
	"context"
	"testing"

	"github.com/apet97/go-clockify/internal/mcp"
)

type progressCancelNotifier struct{ count int }

func (n *progressCancelNotifier) Notify(string, any) error {
	n.count++
	return nil
}

func TestProgressStopsAfterCancellation(t *testing.T) {
	n := &progressCancelNotifier{}
	svc := &Service{Notifier: n}
	ctx := mcp.WithProgressToken(context.Background(), "tok-cancel")
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	svc.EmitProgress(ctx, 1, -1, "after cancellation")
	if n.count != 0 {
		t.Fatalf("EmitProgress emitted %d notifications after cancellation, want 0", n.count)
	}
}
