package perf

// Priority classifies how urgently a Recommendation should be acted on.
type Priority string

const (
	// PriorityLow marks a recommendation worth doing eventually but not
	// blocking any current workload.
	PriorityLow Priority = "low"

	// PriorityMedium marks a recommendation that measurably improves a
	// real query pattern already in production use.
	PriorityMedium Priority = "medium"

	// PriorityHigh marks a recommendation addressing a query pattern that
	// degrades to a full scan or an O(n) intersection under real load.
	PriorityHigh Priority = "high"
)

// RecommendationStatus tracks whether a Recommendation has been acted on.
type RecommendationStatus string

const (
	// StatusProposed is the default status: documented, not yet
	// implemented.
	StatusProposed RecommendationStatus = "proposed"

	// StatusAccepted marks a recommendation a maintainer has agreed to
	// implement, but has not yet merged.
	StatusAccepted RecommendationStatus = "accepted"

	// StatusImplemented marks a recommendation whose change has merged.
	StatusImplemented RecommendationStatus = "implemented"

	// StatusRejected marks a recommendation a maintainer considered and
	// declined, e.g. because the tradeoff was not worth it.
	StatusRejected RecommendationStatus = "rejected"
)

// Recommendation is one structured, actionable performance recommendation
// against an existing package -- in this phase, exclusively against
// packages/graph's existing index.go (see recommendations() below).
// packages/perf documents recommendations; it never implements them (see
// doc.go's "what this phase does NOT modify" section) -- TargetPackage
// names the package a Recommendation is about, not a package this phase
// imports or edits.
type Recommendation struct {
	// ID is a short, stable identifier (e.g. "GRAPH-IDX-001").
	ID string

	// Title is a one-line summary.
	Title string

	// Rationale explains why this recommendation matters, grounded in the
	// referenced package's real, current behavior.
	Rationale string

	// TargetPackage names the package this recommendation is about (e.g.
	// "packages/graph"). Reference only: this package does not import
	// TargetPackage as a result of documenting a recommendation about it.
	TargetPackage string

	// TargetFile names the specific file within TargetPackage the
	// recommendation concerns, when applicable.
	TargetFile string

	// Impact is this recommendation's priority (see Priority).
	Impact Priority

	// Status tracks whether this recommendation has been acted on (see
	// RecommendationStatus).
	Status RecommendationStatus
}

// validate reports whether r has every field a well-formed Recommendation
// requires.
func (r Recommendation) validate() error {
	if r.ID == "" || r.Title == "" || r.Rationale == "" || r.TargetPackage == "" {
		return ErrInvalidRecommendation
	}
	if r.Impact != PriorityLow && r.Impact != PriorityMedium && r.Impact != PriorityHigh {
		return ErrInvalidRecommendation
	}
	switch r.Status {
	case StatusProposed, StatusAccepted, StatusImplemented, StatusRejected:
	default:
		return ErrInvalidRecommendation
	}
	return nil
}

// GraphIndexRecommendations returns this phase's concrete, actionable
// indexing recommendations for packages/graph's existing index.go, grounded
// in what that file actually implements today:
//
//   - indexMigrations() registers two SEPARATE single-column Neo4j Cypher
//     indexes -- "CREATE INDEX ... FOR (n:IracNode) ON (n.type)" and
//     "... ON (n.case_id)" -- rather than one composite index.
//   - inMemoryIndex (the in-memory secondary index) holds only a byType
//     map[string]map[string]struct{}; InMemoryGraphStore.byCase is a
//     separate, independently-maintained index (see inmemory.go), with no
//     composite case+type structure connecting them.
//   - graph.TraversalQuery accepts CaseID and NodeType together in a single
//     call (see store.go), and packages/traversal's every named hop
//     (ViaGoverningRule et al.) walks edges scoped to one case's tree while
//     filtering candidates by NodeTypeFilter -- i.e. the (case_id, type)
//     combination is exactly the platform's real, load-bearing query shape,
//     not a hypothetical one.
//
// packages/perf does not implement any of these -- see
// doc/graph-optimization-checklist.md for the full write-up and doc.go's
// "what this phase does NOT modify" section.
func GraphIndexRecommendations() []Recommendation {
	return []Recommendation{
		{
			ID:    "GRAPH-IDX-001",
			Title: "Add a composite (case_id, type) Neo4j index instead of two single-column indexes",
			Rationale: "indexMigrations() (packages/graph/index.go) registers " +
				"irac_node_case_id_index and irac_node_type_index as two " +
				"independent single-column indexes. Every real caller " +
				"(graph.TraversalQuery, walked by every packages/traversal " +
				"named hop) filters on CaseID and NodeType together in the " +
				"same query. Neo4j must currently intersect two separate " +
				"index lookups (or fall back to one index plus a filter " +
				"scan) instead of resolving the combined filter with one " +
				"composite-index seek: CREATE INDEX irac_node_case_type_index " +
				"FOR (n:IracNode) ON (n.case_id, n.type).",
			TargetPackage: "packages/graph",
			TargetFile:    "index.go",
			Impact:        PriorityHigh,
			Status:        StatusProposed,
		},
		{
			ID:    "GRAPH-IDX-002",
			Title: "Add a composite case+type secondary index for InMemoryGraphStore, not just byType",
			Rationale: "inMemoryIndex (index.go) only maintains a byType " +
				"map[string]map[string]struct{}; InMemoryGraphStore.byCase " +
				"(inmemory.go) is a second, separately-maintained index with " +
				"no structural link between them. A Traverse call filtering " +
				"on both CaseID and NodeType (the traversal package's " +
				"universal query shape) has no single-index path and must " +
				"resolve one index, then linearly filter the result set " +
				"against the other -- an intersection scan, not an " +
				"index-backed lookup, even in the in-memory backend that a " +
				"correctness-focused deployment relies on today. Adding a " +
				"byCaseAndType map[string]map[string]map[string]struct{} (or " +
				"a composite string key \"case|type\" -> set) alongside the " +
				"existing byType index would give Traverse an O(1) path for " +
				"the combined filter.",
			TargetPackage: "packages/graph",
			TargetFile:    "index.go",
			Impact:        PriorityHigh,
			Status:        StatusProposed,
		},
		{
			ID:    "GRAPH-IDX-003",
			Title: "Document index.go's indexMigrations() as append-only and require a migration-count bump",
			Rationale: "indexMigrations() returns a fixed []Migration slice " +
				"with no versioning metadata beyond each entry's Name. As " +
				"more indexes are proposed (see GRAPH-IDX-001), a future " +
				"contributor could edit an existing Migration's Cypher " +
				"in-place rather than appending a new one, which would not " +
				"reapply cleanly against a Neo4j instance that already ran " +
				"the old migration (CREATE INDEX ... IF NOT EXISTS is a " +
				"no-op against an index with the same name but different " +
				"definition). Recommend documenting (in index.go's own " +
				"doc comment) that migrations are append-only, mirroring " +
				"packages/persistence/migrations' up/down numbered-file " +
				"convention this platform already uses elsewhere.",
			TargetPackage: "packages/graph",
			TargetFile:    "index.go",
			Impact:        PriorityLow,
			Status:        StatusProposed,
		},
		{
			ID:    "GRAPH-IDX-004",
			Title: "Expose inMemoryIndex hit/miss counters for observability",
			Rationale: "inMemoryIndex.nodeIDsByType (index.go) has no " +
				"instrumentation: there is no way to observe, in a live " +
				"deployment running InMemoryGraphStore, how often Traverse " +
				"resolves via the type index versus falling through to a " +
				"full scan for an untyped query. Exposing a simple counter " +
				"pair (indexHits/indexMisses) via packages/observability " +
				"(already a sibling dependency of packages/graph) would let " +
				"an operator confirm GRAPH-IDX-002's composite index is " +
				"actually being hit once implemented, rather than assuming " +
				"it from code inspection alone.",
			TargetPackage: "packages/graph",
			TargetFile:    "index.go",
			Impact:        PriorityMedium,
			Status:        StatusProposed,
		},
	}
}
