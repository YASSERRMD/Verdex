# Graph optimization checklist (Phase 091)

This checklist documents concrete, actionable indexing recommendations
against `packages/graph/index.go`'s existing, unmodified content. It is
the human-readable companion to `GraphIndexRecommendations()`
(recommendations.go), which returns the same list as structured
`Recommendation` values for programmatic consumption.

**`packages/perf` does not implement any recommendation below.**
`packages/graph` is not imported for write access, and no file in that
package is edited by this phase -- see `packages/perf/doc.go`'s "what
this phase does NOT modify" section.

## Why these recommendations, grounded in what index.go actually does

`packages/graph/index.go` today provides two independent mechanisms:

1. **`indexMigrations()`**: returns two separate Neo4j Cypher `CREATE
   INDEX` statements for the graph-backed path --
   `irac_node_type_index` (`ON (n.type)`) and `irac_node_case_id_index`
   (`ON (n.case_id)`).
2. **`inMemoryIndex`**: a secondary index for `InMemoryGraphStore`
   holding only `byType map[string]map[string]struct{}` (node IDs
   grouped by `NodeType`). `InMemoryGraphStore`'s separate `byCase`
   field (in `inmemory.go`) is the only case-scoping index, maintained
   independently with no structural link to `byType`.

Meanwhile, `graph.TraversalQuery` (store.go) accepts `CaseID` and
`NodeType` **together** in a single `Traverse` call, and every named hop
`packages/traversal.Query` exposes (`ViaGoverningRule`,
`ViaControllingPrecedent`, `ViaDistinguishingFacts`) walks edges scoped
to one case's tree while filtering candidates by `NodeTypeFilter` in the
same operation. The combined `(case_id, type)` filter is this platform's
real, load-bearing query shape today -- not a hypothetical one this
checklist invents to sound thorough.

## Checklist

- [ ] **GRAPH-IDX-001** (priority: high) -- Add a composite
  `(case_id, type)` Neo4j index instead of two single-column indexes.

  `indexMigrations()` registers `irac_node_case_id_index` and
  `irac_node_type_index` independently. Every real caller filters on
  both together. Neo4j must currently intersect two separate
  single-column index lookups (or resolve one index plus a filter scan)
  instead of a single composite-index seek. Recommended addition:

  ```cypher
  CREATE INDEX irac_node_case_type_index IF NOT EXISTS
  FOR (n:IracNode) ON (n.case_id, n.type)
  ```

- [ ] **GRAPH-IDX-002** (priority: high) -- Add a composite case+type
  secondary index for `InMemoryGraphStore`, not just `byType`.

  `inMemoryIndex.byType` has no counterpart joining case scoping and
  type filtering. A `Traverse` call filtering on both today resolves one
  index (most naturally `byCase`, since `CaseID` is mandatory on every
  `TraversalQuery`) and then linearly filters that result set by
  `NodeType` -- an intersection scan, not an index-backed lookup, even
  in the reference in-memory backend most deployments and tests actually
  exercise. Recommended addition: a
  `byCaseAndType map[string]map[string]map[string]struct{}` (or an
  equivalent composite string key `"<case_id>|<type>"` mapping to a node
  ID set) maintained alongside the existing `byType` index, giving
  `Traverse` an O(1) path for the combined filter.

- [ ] **GRAPH-IDX-003** (priority: low) -- Document `indexMigrations()`
  as append-only, requiring a migration-count bump for any future index
  addition.

  `indexMigrations()` returns a fixed `[]Migration` slice with no
  versioning metadata beyond each entry's `Name`. As GRAPH-IDX-001 and
  similar future indexes are added, a contributor could edit an existing
  `Migration`'s `Cypher` in place rather than appending a new entry,
  which will not reapply cleanly against a Neo4j instance that already
  ran the old migration (`CREATE INDEX ... IF NOT EXISTS` is a no-op
  against an index of the same name with a different definition).
  Recommend adding a doc comment on `indexMigrations()` stating the
  append-only convention, mirroring the numbered up/down migration-file
  convention `packages/persistence/migrations` already uses elsewhere in
  this codebase.

- [ ] **GRAPH-IDX-004** (priority: medium) -- Expose `inMemoryIndex`
  hit/miss counters for observability.

  `inMemoryIndex.nodeIDsByType` has no instrumentation: there is no way
  to observe, in a live deployment running `InMemoryGraphStore`, how
  often `Traverse` resolves via the type index versus falling through to
  a full scan for an untyped query. Recommend exposing a simple counter
  pair (`indexHits`/`indexMisses`) via `packages/observability` (already
  a sibling dependency of `packages/graph`), so an operator can confirm
  GRAPH-IDX-002's composite index is actually being hit once
  implemented, rather than inferring it from code inspection alone.

## Status

All four recommendations are `StatusProposed` as of this phase. See
`GraphIndexRecommendations()` (recommendations.go) for the structured
form, including each recommendation's `TargetFile` and `Impact`.
