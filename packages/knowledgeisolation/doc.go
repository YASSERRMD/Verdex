// Package knowledgeisolation prevents one case's facts from leaking into
// another case's reasoning, within a single tenant. packages/graph's
// TenantScopedStore already enforces cross-TENANT isolation by rejecting
// (not silently filtering) access outside a tenant's owned nodes; this
// package adds a narrower, additional isolation axis on top of it:
// cross-CASE isolation within a single tenant. graph.GraphStore's
// Traverse/GetNode/CreateEdge have no case-boundary enforcement of their
// own — any caller holding a node ID from case A can pass it to a query
// scoped to case B without the interface stopping them — and every
// CaseID-carrying retrieval-layer package (hybridretrieval,
// adaptiveretrieval, traversal, vectorindex) relies on the underlying
// store to actually enforce that boundary. This package is that
// enforcement layer.
//
// # Core concepts
//
//   - CaseScopedStore: wraps any graph.GraphStore and enforces that
//     every node/edge write and read is scoped to a single case,
//     rejecting cross-case access with ErrCrossCaseAccess (case_store.go).
//     Mirrors graph.TenantScopedStore's method signatures and
//     reject-don't-filter philosophy, keyed on CaseID instead of
//     TenantID.
//   - NodeScope / ClassifyNodeType / IsSharedLawNode: classify whether
//     an irac.Node is case-private (Issue/Fact/Application/Conclusion)
//     or part of the shared statute/precedent corpus (Rule), so the
//     guard can allow shared-law reads across every case while still
//     blocking case-fact leakage (scope.go).
//   - CaseScopedVectorStore: the same case-boundary rule enforced at the
//     semantic/hybrid retrieval layer, wrapping any
//     vectorindex.VectorStore (vector_store.go).
//   - CompoundScopedStore: composes graph.TenantScopedStore's
//     cross-tenant guard with CaseScopedStore's cross-case guard into a
//     single call chain, so one GraphStore value enforces both isolation
//     axes (compound_store.go).
//   - CrossCaseAuthorization / CrossCaseReader: a narrowly-scoped,
//     explicit opt-in escape hatch for legitimate cross-case features
//     (e.g. a future analytics dashboard), deliberately unreachable
//     through the normal guard types (crosscase.go).
//   - AccessAttempt / AlertSink: every rejected (or, for Traverse,
//     filtered) cross-case access attempt is recorded with enough detail
//     to support a security review, and forwarded to a pluggable,
//     no-op-by-default AlertSink (audit.go).
//
// # Design principles
//
//   - Reject, don't silently filter — except Traverse, which filters
//     (matching graph.TenantScopedStore.Traverse's own precedent) since
//     a traversal's result set is expected to legitimately mix
//     case-owned and shared-law nodes.
//   - Shared law is not a leak. RuleNodes carry a CaseID for provenance
//     only; that field is never treated as an ownership boundary.
//   - The cross-case escape hatch must be hard to reach by accident: no
//     method on CaseScopedStore or CaseScopedVectorStore can ever return
//     another case's data; only CrossCaseReader, given an explicit,
//     non-empty, unexpired CrossCaseAuthorization, can.
//   - No hard dependency on packages/tenancy. CaseID is a plain string,
//     matching every sibling retrieval-layer package's convention.
//   - No hardcoded LLM provider; this package makes no provider calls at
//     all.
//
// See doc/knowledge-isolation.md for a fuller write-up, including the
// case-vs-tenant-vs-shared-law isolation model, the guard composition
// pattern, the audit/alert mechanism, and the explicit opt-in escape
// hatch.
package knowledgeisolation
