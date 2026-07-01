package gateway

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware is a function that wraps an http.Handler with additional behaviour.
type Middleware func(http.Handler) http.Handler

// Chain composes multiple middleware into a single Middleware. The middleware
// are applied in declaration order: the first entry in the list is outermost
// (executes first on the way in and last on the way out).
func Chain(mw ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		// Apply in reverse so the first middleware ends up outermost.
		for i := len(mw) - 1; i >= 0; i-- {
			next = mw[i](next)
		}
		return next
	}
}

// Recovery is a middleware that catches panics, logs the stack trace, and
// returns a 500 Internal Server Error response instead of crashing the server.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("gateway: panic recovered: %v\n%s", rec, debug.Stack())
				WriteError(w, &APIError{
					Code:    ErrCodeInternal,
					Message: "an internal server error occurred",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Timeout returns a middleware that cancels the request context after d. If the
// handler does not finish before the deadline the client receives a 503 response.
//
// Note: Timeout only cancels the context; it does not forcibly terminate a
// handler that ignores context cancellation. Handlers should check
// r.Context().Err() or use context-aware I/O.
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			done := make(chan struct{})
			tw := &timeoutWriter{w: w}

			go func() {
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				// Handler finished in time; flush headers if not yet sent.
				tw.flush()
			case <-ctx.Done():
				tw.timedOut()
			}
		})
	}
}

// timeoutWriter is a ResponseWriter wrapper that buffers the status code and
// headers until the handler completes, allowing Timeout to send a 503 when the
// deadline is exceeded before the handler writes anything.
type timeoutWriter struct {
	w       http.ResponseWriter
	code    int
	written bool
}

func (tw *timeoutWriter) Header() http.Header { return tw.w.Header() }

func (tw *timeoutWriter) WriteHeader(code int) {
	if !tw.written {
		tw.code = code
	}
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	if !tw.written {
		tw.flush()
	}
	return tw.w.Write(b)
}

func (tw *timeoutWriter) flush() {
	if tw.written {
		return
	}
	tw.written = true
	if tw.code != 0 {
		tw.w.WriteHeader(tw.code)
	}
}

func (tw *timeoutWriter) timedOut() {
	if tw.written {
		return
	}
	tw.written = true
	tw.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	tw.w.WriteHeader(http.StatusServiceUnavailable)
}
