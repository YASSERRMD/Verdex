package observability

import "net/http"

// CorrelationMiddleware returns net/http middleware that:
//
//  1. Reads the inbound CorrelationIDHeader from the request.
//  2. Generates a new correlation ID if the header is missing or empty.
//  3. Stores the ID on the request's context (retrievable via
//     CorrelationIDFromContext) along with a child of base that has the
//     ID attached as a structured field (retrievable via FromContext).
//  4. Sets the resolved ID on the response's CorrelationIDHeader before
//     any bytes are written, so downstream clients and proxies can
//     correlate the response with the request that produced it.
//
// base is the application's root Logger; a per-request child logger
// (base.With("correlation_id", id)) is attached to the context so that
// observability.FromContext(r.Context(), base) returns a logger that
// always includes the correlation ID, without every handler needing to
// add it manually.
func CorrelationMiddleware(base *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(CorrelationIDHeader)
			if id == "" {
				id = NewCorrelationID()
			}

			w.Header().Set(CorrelationIDHeader, id)

			ctx := WithCorrelationID(r.Context(), id)
			requestLogger := base.With(correlationIDLogField, id)
			ctx = WithLogger(ctx, requestLogger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
