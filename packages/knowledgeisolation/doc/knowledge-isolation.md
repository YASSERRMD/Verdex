# Cross-case knowledge isolation (`packages/knowledgeisolation`)

`packages/graph`'s `TenantScopedStore` already enforces cross-**tenant**
isolation: it rejects (not silently filters) any read or write outside a
tenant's owned nodes. That leaves a narrower gap open within a single
tenant: nothing stops a caller holding a node ID or query from case A
from reaching case B's reasoning tree, because `graph.GraphStore`'s
`Traverse`/`GetNode`/`CreateEdge` methods have no notion of "the caller
is only authorized for case B" at all — and every retrieval-layer
package built on top (`hybridretrieval`, `adaptiveretrieval`,
`traversal`, `vectorindex`) takes a `CaseID` as part of its query type
but relies entirely on the underlying store to enforce it.

`packages/knowledgeisolation` is that enforcement layer: a set of
wrapper types that sit between a caller and a `graph.GraphStore` /
`vectorindex.VectorStore`, closing the case-boundary gap the same way
`TenantScopedStore` closes the tenant-boundary gap.

## The isolation model: case vs. tenant vs. shared law

Three independent axes classify every read or write this package
mediates:

| Axis | Enforced by | Boundary |
|---|---|---|
| Tenant | `graph.TenantScopedStore` (existing, Phase 032) | A tenant's nodes are never visible to another tenant, full stop — no exceptions. |
| Case | `CaseScopedStore` / `CaseScopedVectorStore` (this package) | A case's private facts are never visible to another case **within the same tenant** — except shared law. |
| Shared law | `NodeScope` / `ClassifyNodeType` / `IsSharedLawNode` (this package) | Statute/precedent (`irac.NodeRule`) is not case-exclusive: it is read-shareable across every case in a tenant, even though each `irac.Node` still carries a `CaseID` for provenance (which case first ingested or referenced it). |

Concretely: `irac.Node.CaseID` on a `RuleNode` records *provenance*, not
*ownership*. `ClassifyNodeType` maps `irac.NodeRule` to `ScopeSharedLaw`
and every other recognized `NodeType` (`Issue`, `Fact`, `Application`,
`Conclusion`) to `ScopeCaseFacts`. An unrecognized `NodeType` defaults to
`ScopeCaseFacts` — the more restrictive choice — so a guard never
accidentally widens access for a node type it doesn't understand.

## The primary type: `CaseScopedStore`

```go
type CaseScopedStore struct {
    // unexported: inner graph.GraphStore, caseID CaseID, auditor *auditRecorder
}

func NewCaseScopedStore(inner graph.GraphStore, caseID CaseID, sink AlertSink) (*CaseScopedStore, error)
```

`CaseScopedStore` implements `graph.GraphStore` in full (`CreateNode`,
`CreateEdge`, `GetNode`, `Traverse`, `DeleteTree`), so it can be dropped
in anywhere a `graph.GraphStore` is expected — including as the `inner`
store passed to `hybridretrieval`/`traversal`/`vectorindex` construction
paths, with no changes to those packages.

Its rules, mirroring `graph.TenantScopedStore`'s reject-don't-filter
philosophy:

- **`GetNode`** — rejects with `ErrCrossCaseAccess` if the node is
  case-scoped and owned by a different case. Shared-law nodes always
  succeed.
- **`CreateNode`** — rejects a case-scoped node not belonging to this
  store's case. A shared-law node is accepted regardless of the `CaseID`
  it carries.
- **`CreateEdge`** — rejects if either endpoint is a case-scoped node
  belonging to a different case. This is the specific leakage vector the
  guard exists to close: stitching case-A facts into case-B's reasoning
  tree via a single shared edge. (Note: the underlying
  `graph.InMemoryGraphStore.CreateEdge` already refuses edges whose two
  endpoints have different `CaseID` values at all, so in practice this
  check matters most against a future store implementation that is more
  permissive than `InMemoryGraphStore`.)
- **`Traverse`** — the one exception to "reject": like
  `TenantScopedStore.Traverse`, it **filters** rather than erroring,
  because a traversal's result set is expected to legitimately mix
  case-owned and shared-law nodes. Every filtered-out node is still
  recorded as an `AccessAttempt`.
- **`DeleteTree`** — rejects outright if the requested `caseID` does not
  match this store's authorized case.

## `CaseScopedVectorStore`: the same rule at the retrieval layer

```go
func NewCaseScopedVectorStore(inner vectorindex.VectorStore, caseID CaseID, sink AlertSink) (*CaseScopedVectorStore, error)
```

Implements `vectorindex.VectorStore` in full. `vectorindex.VectorRecord`
carries its own `NodeType` and `CaseID`, so records are classified the
same way nodes are (`ClassifyNodeType`).

- **`Query`** unconditionally overwrites `req.CaseID` with the store's
  authorized case before delegating — a caller cannot widen or redirect
  a search by supplying a different `CaseID`. Because the inner store's
  own `CaseID` filter (see `vectorindex.VectorStore.Query`) then does the
  actual scoping, `Query` never needs to post-filter results itself. If
  the caller's original `req.CaseID` differed from the authorized case,
  that override is recorded as an `AccessAttempt` even though the
  request degrades gracefully instead of failing outright — so a
  mis-scoped caller is still visible to a security review.
- **`Upsert`** rejects a case-scoped record belonging to a different
  case; a shared-law record (by `NodeType`) is always accepted.
- **`DeleteCase`** rejects outright for any case other than the
  authorized one.
- **`Delete`** and **`Health`** delegate directly — a delete-by-record-id
  has no case identity to check against, matching
  `vectorindex.VectorStore.Delete`'s own "not an error to delete an
  absent id" convention.

## Guard composition: `CompoundScopedStore`

Tenant isolation and case isolation are orthogonal and meant to be used
together. Rather than reimplementing tenant logic, `CompoundScopedStore`
composes the two existing guards:

```go
type CompoundScopedStore struct {
    *CaseScopedStore
    // unexported: tenantStore *graph.TenantScopedStore, tenant graph.TenantID
}

func NewCompoundScopedStore(
    inner graph.GraphStore,
    tenant graph.TenantID, owners map[string]graph.TenantID,
    caseID CaseID, sink AlertSink,
) (*CompoundScopedStore, error)
```

This wraps `inner` in a `graph.TenantScopedStore` (tenant check), then
wraps *that* in a `CaseScopedStore` (case check) — tenant-then-case is
the fixed, documented default. Either order of composition yields an
equivalent isolation guarantee, since each guard only ever narrows what
the other can see; if a caller needs a different layering for some
reason, composing the two guards manually (as `NewCompoundScopedStore`
does internally) is fully supported:

```go
tenantStore := graph.NewTenantScopedStore(inner, tenantID, owners)
caseStore, err := knowledgeisolation.NewCaseScopedStore(tenantStore, caseID, sink)
```

A `CompoundScopedStore` enforces both boundaries on every call: a
same-tenant, different-case read is rejected by the case layer even if
two tenants happen to reuse the same `CaseID` string value, and a
different-tenant read is rejected by the tenant layer even for the
"same" case ID. Cross-tenant rejections surface as
`graph.ErrCrossTenantAccess` from the tenant layer; this package does
not separately audit those, since `graph.TenantScopedStore` is
out-of-package and has no audit hook of its own — auditing here covers
only the case-boundary checks this package owns.

## The explicit opt-in escape hatch: `CrossCaseAuthorization` / `CrossCaseReader`

Legitimate cross-case features exist (e.g. a future cross-case analytics
dashboard). Rather than adding a bypass flag to `CaseScopedStore` itself
— which would be one accidental `true` away from a leak — cross-case
reads are only reachable through a dedicated type that never implements
`graph.GraphStore` and is never the store a normal retrieval call chain
is wired to:

```go
type CrossCaseAuthorization struct {
    Cases     []CaseID  // must be non-empty
    Reason    string    // recorded in every audit entry
    ExpiresAt time.Time // zero means no expiry
}

func NewCrossCaseReader(inner graph.GraphStore, sink AlertSink) (*CrossCaseReader, error)
func (r *CrossCaseReader) GetNodeAcrossCases(ctx context.Context, id string, auth CrossCaseAuthorization) (irac.Node, error)
func (r *CrossCaseReader) TraverseAcrossCases(ctx context.Context, query graph.TraversalQuery, auth CrossCaseAuthorization) ([]irac.Node, error)
```

Both methods reject with `ErrMissingAuthorization` (no cases listed),
`ErrAuthorizationExpired` (past `ExpiresAt`), or `ErrCaseNotAuthorized`
(the requested case isn't in `auth.Cases`) before touching the inner
store. Shared-law nodes bypass the case-coverage check (they were never
case-exclusive to begin with) but still require a *valid, unexpired*
authorization — an empty `CrossCaseAuthorization{}` is rejected even for
a shared-law read, so the escape hatch is never a true no-op call.

Every call through `CrossCaseReader` — authorized or rejected — is
recorded as an `AccessAttempt` with `Kind: ViolationCrossCaseAnalysis`,
so a legitimate cross-case read leaves the same kind of audit trail a
rejected leakage attempt does, distinguished by `Detail`.

## Audit and alerting

```go
type AccessAttempt struct {
    Kind            ViolationKind
    AuthorizedCases []CaseID
    AttemptedCase   CaseID
    NodeID          string
    Detail          string
    OccurredAt      time.Time
}

type AlertSink interface {
    Notify(attempt AccessAttempt)
}
```

Every guard in this package (`CaseScopedStore`, `CaseScopedVectorStore`,
`CrossCaseReader`) owns a package-local `auditRecorder` (mutex + slice),
matching the `treeindex`/`traversal`/`adaptiveretrieval` convention of a
package-local telemetry struct rather than depending on
`packages/observability`. `AccessAttempts()` returns a defensive copy of
everything recorded so far, for tests and ad hoc security review.

`AlertSink` mirrors `packages/accounting`'s `AlertSink` pattern:
`NoOpAlertSink` (the default, used when a guard is constructed with a
nil sink), `FuncAlertSink` (adapts a plain function, for simple
stateless sinks — the same function-type-indirection convention as
`packages/traversal.PrecedentResolver`), and `MultiAlertSink` (fans out
to several sinks).

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/graph` | `GraphStore`, `TenantScopedStore`, `InMemoryGraphStore`. | `CaseScopedStore` wraps any `GraphStore` (including a `TenantScopedStore`, via `CompoundScopedStore`) and implements the same interface. Never reimplements tenant logic. |
| `packages/vectorindex` | `VectorStore`, `InMemoryVectorStore`, `VectorRecord`. | `CaseScopedVectorStore` wraps any `VectorStore` and implements the same interface. Reuses `VectorRecord.NodeType`/`CaseID` rather than inventing a parallel record shape. |
| `packages/irac` | `Node`, `NodeType`, `Edge`. | Read-only consumer: classifies nodes by their existing `Type`/`CaseID` fields (`ClassifyNodeType`); never adds fields to `irac.Node` or defines a competing node shape. |
| `packages/hybridretrieval`, `packages/adaptiveretrieval`, `packages/traversal` | `CaseID`-carrying query types (`HybridQuery`, `AdaptiveQuery`, `Query`) that assume the underlying store enforces case scoping. | Not imported. This package is the enforcement those packages assume exists; a caller wires a `CaseScopedStore`/`CaseScopedVectorStore` in as the `GraphStore`/`VectorStore` those packages already accept, with no changes to their own code. |
| `packages/accounting` | `AlertSink` pattern (`Send(ctx, event) error`). | Not imported; this package redeclares an equivalent `AlertSink` shape (`Notify(attempt)`, no `ctx`/error return) scoped to access-attempt events, matching the convention without a cross-package dependency for a one-method interface. |
| `packages/tenancy` | `WithTenant`/`TenantFromContext` context-propagation pattern. | Not adopted here. This package keeps case scope as an explicit constructor parameter (`NewCaseScopedStore(inner, caseID, sink)`), matching `graph.TenantScopedStore`'s own explicit-wrapper-parameter design, rather than threading `CaseID` through `context.Context`. Consistency with `TenantScopedStore` was judged more valuable than matching `tenancy`'s context-propagation style, since the two guards are meant to compose directly (`CompoundScopedStore`). |

## What this package deliberately does not do

- **It does not enforce tenant isolation itself.** That remains
  `graph.TenantScopedStore`'s job; `CompoundScopedStore` composes with
  it rather than duplicating it.
- **It does not persist anything.** No new storage backend, no schema
  migration, no database connection — every guard is a pure wrapper over
  a caller-supplied `graph.GraphStore` / `vectorindex.VectorStore`.
- **It does not classify nodes by anything other than `NodeType`.** A
  future "some Fact nodes are actually cross-case-shareable" requirement
  is out of scope; the shared-law/case-facts split is a fixed, two-way
  classification keyed purely on `irac.NodeType`.
- **It does not thread case scope through `context.Context`.** Case
  scope is always an explicit wrapper parameter, matching
  `graph.TenantScopedStore`'s design (see the compose-with table above
  for why).
- **It does not rate-limit, throttle, or block callers.** `AlertSink` is
  a notification mechanism only; deciding what to do with repeated
  violations (paging, suspending a credential, etc.) is a consumer
  concern.
- **It does not make the cross-case escape hatch discoverable by
  accident.** There is no flag, no default-true option, and no partial
  bypass on `CaseScopedStore`/`CaseScopedVectorStore` — the only way to
  read across cases is to construct a `CrossCaseReader` and present an
  explicit, non-empty, unexpired `CrossCaseAuthorization` naming exactly
  the cases being analyzed.
- **It does not make any LLM/provider calls.** No hardcoded provider,
  no provider calls of any kind.
