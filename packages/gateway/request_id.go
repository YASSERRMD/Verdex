package gateway

import (
	"net/http"

	"github.com/google/uuid"
)

const headerRequestID = "X-Request-ID"

// RequestIDMiddleware generates a unique request ID for each incoming request
// (or propagates an existing one from the X-Request-ID header) and stores it
// in the request context. The ID is also echoed back in the X-Request-ID
// response header.
//
// Use RequestIDFromContext to retrieve the ID in downstream handlers.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = uuid.New().String()
		}

		// Propagate to response.
		w.Header().Set(headerRequestID, id)

		// Store in context for downstream handlers and response envelopes.
		ctx := withRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
