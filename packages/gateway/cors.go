package gateway

import (
	"net/http"
	"strings"
)

// CORSOptions configures the CORS middleware.
type CORSOptions struct {
	// AllowedOrigins is the list of origins allowed to make cross-origin requests.
	// Use "*" to allow any origin (not recommended in production).
	AllowedOrigins []string
	// AllowedMethods is the list of HTTP methods exposed to the browser.
	// Defaults to GET, POST, PUT, PATCH, DELETE, OPTIONS.
	AllowedMethods []string
	// AllowedHeaders is the list of request headers the browser is permitted to send.
	// Defaults to Content-Type, Authorization, X-Request-ID.
	AllowedHeaders []string
	// ExposedHeaders is the list of response headers the browser is allowed to read.
	ExposedHeaders []string
	// AllowCredentials indicates whether cookies / auth headers are included.
	AllowCredentials bool
	// MaxAge is the value (in seconds) for the Access-Control-Max-Age header.
	MaxAge int
}

var defaultMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
}

var defaultHeaders = []string{
	"Content-Type",
	"Authorization",
	"X-Request-ID",
}

// CORSMiddleware returns a middleware that handles CORS preflight requests and
// injects CORS headers into every response.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return CORSMiddlewareWithOptions(CORSOptions{
		AllowedOrigins: allowedOrigins,
	})
}

// CORSMiddlewareWithOptions is like CORSMiddleware but accepts a full CORSOptions
// struct for fine-grained control.
func CORSMiddlewareWithOptions(opts CORSOptions) func(http.Handler) http.Handler {
	if len(opts.AllowedMethods) == 0 {
		opts.AllowedMethods = defaultMethods
	}
	if len(opts.AllowedHeaders) == 0 {
		opts.AllowedHeaders = defaultHeaders
	}

	methods := strings.Join(opts.AllowedMethods, ", ")
	headers := strings.Join(opts.AllowedHeaders, ", ")

	var exposed string
	if len(opts.ExposedHeaders) > 0 {
		exposed = strings.Join(opts.ExposedHeaders, ", ")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if origin != "" && isOriginAllowed(origin, opts.AllowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")

				if opts.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if exposed != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposed)
				}

				// Handle preflight.
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", methods)
					w.Header().Set("Access-Control-Allow-Headers", headers)

					if opts.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age",
							strings.TrimSpace(strings.Join([]string{
								"", // leading comma workaround – just format inline
							}, "")))
						// Write MaxAge as a decimal string.
						w.Header().Set("Access-Control-Max-Age", formatInt(opts.MaxAge))
					}

					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isOriginAllowed reports whether origin matches any entry in allowed.
// "*" is treated as a wildcard.
func isOriginAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

// formatInt converts an int to its decimal string representation without
// importing strconv at the call site.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
