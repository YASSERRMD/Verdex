// Package gateway provides the HTTP API gateway for the Verdex judicial reasoning
// platform. It implements versioning, request/response envelopes, error handling,
// rate limiting, CORS, security headers, and request-ID propagation.
//
// # API Versioning
//
// All endpoints are served under a versioned path prefix (e.g. /v1/, /v2/).
// The VersionMiddleware validates the version segment and rejects unknown versions
// with a 404 response. Use ParseVersion to convert a raw string to an APIVersion.
//
// # Response Envelope
//
// Every successful response is wrapped in Response[T], which carries the API
// version, a "success" status, the data payload, optional pagination metadata,
// and the request ID. Error responses use ErrorResponse with a machine-readable
// error code, a human-readable message, and optional detail strings.
//
// # Rate Limiting
//
// InMemoryRateLimiter implements a sliding-window rate limiter keyed by an
// arbitrary string (typically the client IP or tenant ID). RateLimitMiddleware
// wires it into the handler chain.
//
// # Middleware
//
// The package ships the following middleware, all satisfying the Middleware type
// (func(http.Handler) http.Handler):
//
//   - RequestIDMiddleware  - generates or propagates X-Request-ID
//   - CORSMiddleware       - CORS preflight and headers
//   - SecurityHeadersMiddleware - defensive security headers
//   - RateLimitMiddleware  - sliding-window rate limiter
//   - VersionMiddleware    - rejects unknown API versions
//   - Recovery            - panic recovery with 500 response
//   - Timeout             - per-request deadline
//
// # Router
//
// Router wraps http.ServeMux and adds Group (path prefix grouping), Handle /
// HandleFunc with automatic version-prefix injection, and a Chain helper that
// applies a slice of Middleware in declaration order.
package gateway
