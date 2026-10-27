// Package auth carries the authenticated caller through a request context.
//
// An authentication interceptor verifies credentials and stores a Principal;
// handlers read it with FromContext and pass TenantID and Subject explicitly
// into application requests. Request-scoped identity is one of the few values
// that belongs in a context; business parameters do not.
package auth

import "context"

// Principal is the verified identity of a caller.
type Principal struct {
	// TenantID scopes every read and write the caller may perform.
	TenantID string
	// Subject identifies the user or service account.
	Subject string
	// Scopes are the granted permissions.
	Scopes []string
}

type principalKey struct{}

// WithPrincipal returns a context carrying p. Only the authentication
// interceptor (or a background job that acts as a known system identity)
// should call it.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext returns the principal and whether one was present.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok && p.TenantID != "" && p.Subject != ""
}
