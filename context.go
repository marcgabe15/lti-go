package lti

import "context"

type contextKey int

const claimsContextKey contextKey = iota

// ContextWithClaims returns a copy of ctx carrying c, retrievable with
// ClaimsFromContext.
func ContextWithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, c)
}

// ClaimsFromContext returns the Claims stored in ctx by
// Tool.LaunchHandler (or ContextWithClaims), if any.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsContextKey).(*Claims)
	return c, ok
}
