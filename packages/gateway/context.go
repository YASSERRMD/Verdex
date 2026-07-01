package gateway

import "context"

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	contextKeyVersion   contextKey = iota
	contextKeyRequestID contextKey = iota
)

// withVersion stores an APIVersion in the context.
func withVersion(ctx context.Context, v APIVersion) context.Context {
	return context.WithValue(ctx, contextKeyVersion, v)
}

// VersionFromContext retrieves the APIVersion stored by VersionMiddleware.
// Returns CurrentVersion if no version is set.
func VersionFromContext(ctx context.Context) APIVersion {
	if v, ok := ctx.Value(contextKeyVersion).(APIVersion); ok {
		return v
	}
	return CurrentVersion
}

// withRequestID stores a request ID string in the context.
func withRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKeyRequestID, id)
}

// RequestIDFromContext retrieves the request ID stored by RequestIDMiddleware.
// Returns an empty string if no request ID is set.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKeyRequestID).(string); ok {
		return id
	}
	return ""
}
