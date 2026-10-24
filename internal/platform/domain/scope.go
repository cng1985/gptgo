package domain

import "context"

type scopeKey struct{}

type Scope struct {
	ActorID   int64
	RequestID string
}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func ScopeFrom(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || scope.ActorID <= 0 {
		return Scope{}, false
	}
	return scope, true
}
