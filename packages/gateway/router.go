package gateway

import (
	"net/http"
	"strings"
)

// Router wraps http.ServeMux and provides grouped routing with middleware
// support and automatic version-prefix injection.
type Router struct {
	mux        *http.ServeMux
	prefix     string
	middleware []Middleware
}

// NewRouter creates a new Router backed by a fresh http.ServeMux.
func NewRouter() *Router {
	return &Router{
		mux: http.NewServeMux(),
	}
}

// Group returns a child Router whose handlers share the given path prefix.
// Middleware registered on the parent is not automatically inherited; register
// shared middleware on the child using Use or by passing it to Handle/HandleFunc.
func (ro *Router) Group(prefix string) *Router {
	return &Router{
		mux:    ro.mux,
		prefix: joinPath(ro.prefix, prefix),
	}
}

// Use appends middleware to the router's middleware chain. Middleware is applied
// in registration order.
func (ro *Router) Use(mw ...Middleware) {
	ro.middleware = append(ro.middleware, mw...)
}

// Handle registers h to handle requests matching pattern. The router's prefix is
// prepended to pattern and all registered middleware is applied around h.
func (ro *Router) Handle(pattern string, h http.Handler) {
	full := joinPath(ro.prefix, pattern)
	ro.mux.Handle(full, ro.applyMiddleware(h))
}

// HandleFunc registers fn to handle requests matching pattern.
func (ro *Router) HandleFunc(pattern string, fn http.HandlerFunc) {
	ro.Handle(pattern, fn)
}

// Mount registers a sub-handler at pattern. Unlike Handle, the path prefix is
// stripped before calling h so that h sees paths relative to its mount point.
func (ro *Router) Mount(pattern string, h http.Handler) {
	full := joinPath(ro.prefix, pattern)
	// Ensure trailing slash so http.StripPrefix works correctly.
	if !strings.HasSuffix(full, "/") {
		full += "/"
	}
	stripped := http.StripPrefix(strings.TrimSuffix(full, "/"), h)
	ro.mux.Handle(full, ro.applyMiddleware(stripped))
}

// ServeHTTP implements http.Handler so Router can be used directly as a server
// handler or mounted into another router.
func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ro.mux.ServeHTTP(w, r)
}

// applyMiddleware wraps h with the router's registered middleware chain.
func (ro *Router) applyMiddleware(h http.Handler) http.Handler {
	if len(ro.middleware) == 0 {
		return h
	}
	return Chain(ro.middleware...)(h)
}

// joinPath concatenates a and b into a clean path, ensuring exactly one slash
// between segments and no trailing slash (unless b is "/").
func joinPath(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	a = strings.TrimSuffix(a, "/")
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return a + b
}
