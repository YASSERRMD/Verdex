// Package observability provides the shared logging, correlation-ID
// propagation, metrics, tracing, health/readiness, audit-logging, and
// log-redaction primitives used across Verdex services.
//
// Every Verdex service is expected to depend on this package rather than
// reaching for slog, Prometheus, or OpenTelemetry directly, so that
// cross-cutting concerns (log shape, correlation IDs, metric naming,
// span conventions) stay consistent fleet-wide. See the package README
// for usage examples covering each of the areas above.
package observability
