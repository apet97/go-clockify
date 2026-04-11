package authn

import "context"

type principalCtxKey struct{}

// WithPrincipal attaches p to ctx so downstream handlers (enforcement,
// rate limiters, audit, tools) can read the authenticated identity.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	if p == nil {
		return ctx
	}
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFromContext returns the principal attached by WithPrincipal, or
// (nil, false) when the request is unauthenticated (static-bearer without
// context installation, tests, etc.).
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	v := ctx.Value(principalCtxKey{})
	if v == nil {
		return nil, false
	}
	p, ok := v.(*Principal)
	return p, ok
}
