// Package reliability is Phase 093: graceful degradation and fault
// tolerance for this platform's dependencies (databases, the graph
// store, external integrations from Phase 087, and downstream
// providers), composing with -- and explicitly not duplicating --
// packages/router's provider-specific circuit breaker (Phase 012),
// packages/ingestion's per-stage idempotency store (Phase 029),
// packages/perf's benchmark budgets and resource limiter (Phase 091),
// and packages/observability's liveness/readiness health-check shape
// (Phase 003).
//
// See doc/reliability.md for the full write-up, including the
// composition table naming every prior phase this package builds on.
package reliability
