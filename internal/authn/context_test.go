package authn

import (
	"context"
	"testing"
)

func TestWithPrincipalRoundTrip(t *testing.T) {
	p := &Principal{Subject: "alice", TenantID: "tenant-1", AuthMode: ModeOIDC}
	ctx := WithPrincipal(context.Background(), p)
	got, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("PrincipalFromContext returned !ok after WithPrincipal")
	}
	if got.Subject != "alice" || got.TenantID != "tenant-1" {
		t.Fatalf("principal: %+v", got)
	}
}

func TestWithPrincipalNilIsNoop(t *testing.T) {
	ctx := WithPrincipal(context.Background(), nil)
	if _, ok := PrincipalFromContext(ctx); ok {
		t.Fatal("PrincipalFromContext returned ok for nil principal")
	}
}

func TestPrincipalFromContextEmpty(t *testing.T) {
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("empty context should return !ok")
	}
}
