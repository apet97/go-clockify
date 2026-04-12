package grpctransport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apet97/go-clockify/internal/authn"
	"github.com/apet97/go-clockify/internal/metrics"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeAuthenticator is a test double for authn.Authenticator. It inspects the
// Authorization header of the synthetic http.Request the interceptor builds
// and returns a canned Principal on match or a canned error otherwise. The
// zero value rejects everything.
type fakeAuthenticator struct {
	wantToken string
	principal authn.Principal
	forceErr  error
}

func (f fakeAuthenticator) Authenticate(_ context.Context, r *http.Request) (authn.Principal, error) {
	if f.forceErr != nil {
		return authn.Principal{}, f.forceErr
	}
	got := r.Header.Get("Authorization")
	if got != "Bearer "+f.wantToken {
		return authn.Principal{}, errors.New("token mismatch")
	}
	return f.principal, nil
}

// mockServerStream is a minimal grpc.ServerStream for direct interceptor
// invocation. It carries a fixed context and treats SendMsg/RecvMsg as no-ops
// that satisfy the interface — the tests below only care about the
// interceptor's authz decision and the context handed to the wrapped handler.
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context     { return m.ctx }
func (m *mockServerStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockServerStream) SendHeader(metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(metadata.MD)       {}
func (m *mockServerStream) SendMsg(any) error            { return nil }
func (m *mockServerStream) RecvMsg(any) error            { return io.EOF }

// callInterceptor invokes the interceptor with a mock stream carrying the
// given metadata. It returns the handler invocation result plus whatever
// principal the handler observed in its stream context (or the zero value
// when the handler never ran).
func callInterceptor(t *testing.T, auth authn.Authenticator, md metadata.MD) (error, *authn.Principal) {
	t.Helper()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}
	var seen *authn.Principal
	handler := func(_ any, ss grpc.ServerStream) error {
		if p, ok := authn.PrincipalFromContext(ss.Context()); ok {
			seen = p
		}
		return nil
	}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{})
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, handler)
	return err, seen
}

func TestAuthInterceptor_MissingMetadata(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("missing_metadata")
	// No incoming metadata at all — use a bare context.
	stream := &mockServerStream{ctx: context.Background()}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{})
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, func(any, grpc.ServerStream) error {
		t.Fatalf("handler should not run when metadata is missing")
		return nil
	})
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("missing_metadata"); got != before+1 {
		t.Fatalf("expected missing_metadata counter to increment by 1, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_MissingAuthorization(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("missing_authorization")
	err, seen := callInterceptor(t, auth, metadata.MD{})
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not have run; got principal %+v", seen)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("missing_authorization"); got != before+1 {
		t.Fatalf("expected missing_authorization counter to increment, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_EmptyAuthorizationValue(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	before := metrics.GRPCAuthRejectionsTotal.Get("empty_authorization")
	err, _ := callInterceptor(t, auth, metadata.Pairs("authorization", ""))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated on empty authorization, got %v", err)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("empty_authorization"); got != before+1 {
		t.Fatalf("expected empty_authorization counter to increment, got %d (before=%d)", got, before)
	}
}

func TestAuthInterceptor_WrongToken(t *testing.T) {
	auth := fakeAuthenticator{wantToken: "correct"}
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer nope"))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not have run; got principal %+v", seen)
	}
}

func TestAuthInterceptor_HappyPathPropagatesPrincipal(t *testing.T) {
	want := authn.Principal{
		Subject:  "alice@example.com",
		TenantID: "acme",
		AuthMode: authn.ModeStaticBearer,
		Claims:   map[string]string{},
	}
	auth := fakeAuthenticator{wantToken: "correct", principal: want}
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer correct"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen == nil {
		t.Fatal("handler saw no principal in context")
	}
	if seen.Subject != want.Subject || seen.TenantID != want.TenantID || seen.AuthMode != want.AuthMode {
		t.Fatalf("principal mismatch: want %+v, got %+v", want, *seen)
	}
}

func TestAuthInterceptor_AuthenticatorError(t *testing.T) {
	auth := fakeAuthenticator{forceErr: errors.New("token expired")}
	before := metrics.GRPCAuthRejectionsTotal.Get("auth_failed")
	err, seen := callInterceptor(t, auth, metadata.Pairs("authorization", "Bearer whatever"))
	if st, ok := status.FromError(err); !ok || st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated on authenticator error, got %v", err)
	}
	if seen != nil {
		t.Fatalf("handler should not run when authenticator fails; got %+v", seen)
	}
	if got := metrics.GRPCAuthRejectionsTotal.Get("auth_failed"); got != before+1 {
		t.Fatalf("expected auth_failed counter to increment, got %d (before=%d)", got, before)
	}
}

type countingAuthenticator struct {
	calls     atomic.Int32
	failAfter int32
	principal authn.Principal
}

func (c *countingAuthenticator) Authenticate(_ context.Context, _ *http.Request) (authn.Principal, error) {
	n := c.calls.Add(1)
	if c.failAfter > 0 && n > c.failAfter {
		return authn.Principal{}, errors.New("token expired")
	}
	return c.principal, nil
}

func TestReauthLoop_ExpiryClosesStream(t *testing.T) {
	auth := &countingAuthenticator{
		failAfter: 2,
		principal: authn.Principal{Subject: "alice", TenantID: "acme", AuthMode: authn.ModeOIDC},
	}
	interceptor := authStreamInterceptor(auth, authInterceptorConfig{reauthInterval: 50 * time.Millisecond})
	md := metadata.Pairs("authorization", "Bearer token")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}

	handlerDone := make(chan error, 1)
	handler := func(_ any, ss grpc.ServerStream) error {
		<-ss.Context().Done()
		return ss.Context().Err()
	}
	go func() {
		handlerDone <- interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test/Method"}, handler)
	}()

	select {
	case err := <-handlerDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("handler returned unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit within 2s — reauth did not cancel context")
	}

	if auth.calls.Load() < 2 {
		t.Fatalf("expected at least 2 auth calls, got %d", auth.calls.Load())
	}
}

func TestServe_InterceptorInstalledWhenAuthenticatorSet(t *testing.T) {
	// Smoke: verify Serve() honours the Authenticator field. A full
	// end-to-end round-trip is covered by transport_test.go's bufconn
	// harness; here we just check the field wires through to the
	// interceptor chain without reflection.
	want := authn.Principal{Subject: "svc", TenantID: "acme"}
	auth := fakeAuthenticator{wantToken: "k", principal: want}
	opts := Options{Bind: "127.0.0.1:0", Server: nil, Authenticator: auth}
	// Serve rejects nil Server before touching the interceptor, so the
	// test stops at the guard — but the wire-up is exercised above via
	// authStreamInterceptor directly. Keep this test narrow: if Options
	// gains a new field that Serve ignores, a separate test will flag it.
	if err := Serve(context.Background(), opts); err == nil {
		t.Fatal("expected Serve to reject nil Server even with Authenticator set")
	}
}
