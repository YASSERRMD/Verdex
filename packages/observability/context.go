package observability

import "context"

// loggerKey is an unexported context key type for storing a
// request-scoped *Logger, distinct from correlationIDKey.
type loggerKey struct{}

// WithLogger returns a copy of ctx carrying logger as the active
// request-scoped logger. A subsequent FromContext(ctx) call returns
// logger.
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the *Logger attached to ctx by WithLogger, or
// fallback (with the context's correlation ID attached, if any) when
// none is present. fallback must not be nil.
//
// This is the standard way request-handling code obtains a logger:
// call FromContext(ctx, baseLogger) rather than reaching for a package
// global, so every log line automatically carries the correlation ID
// (and any other fields) bound earlier in the request's lifecycle.
func FromContext(ctx context.Context, fallback *Logger) *Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*Logger); ok && logger != nil {
		return logger
	}
	if id, ok := CorrelationIDFromContext(ctx); ok {
		return fallback.With(correlationIDLogField, id)
	}
	return fallback
}

// correlationIDLogField is the structured log field name used for the
// correlation ID whenever a Logger is derived from a context that
// carries one (see FromContext and the CorrelationMiddleware).
const correlationIDLogField = "correlation_id"
