package observability

import (
	"context"

	"github.com/google/uuid"
)

// correlationIDKey is an unexported context key type so values stored
// by this package can never collide with keys set by other packages.
type correlationIDKey struct{}

// CorrelationIDHeader is the HTTP header used to propagate a
// correlation ID across service boundaries, both inbound (read from
// the request) and outbound (written to the response).
const CorrelationIDHeader = "X-Correlation-ID"

// NewCorrelationID generates a new random correlation ID (a UUID v4
// string). Call this when no inbound ID is available, e.g. at the
// start of a background job or when handling a request that omitted
// the correlation header.
func NewCorrelationID() string {
	return uuid.NewString()
}

// WithCorrelationID returns a copy of ctx carrying id as the active
// correlation ID. A subsequent CorrelationIDFromContext(ctx) call
// returns id.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

// CorrelationIDFromContext returns the correlation ID stored in ctx by
// WithCorrelationID, and whether one was present. Callers that want a
// best-effort ID even when none was set should prefer
// EnsureCorrelationID.
func CorrelationIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(correlationIDKey{}).(string)
	return id, ok && id != ""
}

// EnsureCorrelationID returns ctx unchanged if it already carries a
// correlation ID, or a derived context with a freshly generated one
// attached otherwise. It always returns a context with a non-empty
// correlation ID, plus that ID for convenience.
func EnsureCorrelationID(ctx context.Context) (context.Context, string) {
	if id, ok := CorrelationIDFromContext(ctx); ok {
		return ctx, id
	}
	id := NewCorrelationID()
	return WithCorrelationID(ctx, id), id
}
