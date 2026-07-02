package treeindex

// PathIndex is the materialized set of Paths computed for a single case's
// reasoning tree. It is the payload an Indexer builds (via RebuildCase)
// and caches lookups against.
//
// Paths are grouped two ways, matching the two lookup shapes this package
// supports:
//
//   - byRoot maps a root NodeRef.ID (the RuleNode of a
//     PathKindRuleGroupedIssues Path, or the IssueNode of a
//     PathKindReasoningChain Path) to every Path rooted there. This backs
//     LookupPaths's fromNodeID parameter.
//   - byKind maps a PathKind to every Path of that kind, letting a caller
//     ask "give me every reasoning chain in this case" without knowing any
//     specific root ID up front.
//
// PathIndex is a plain data structure with no synchronization of its own;
// Indexer (indexer.go) is responsible for guarding concurrent access.
type PathIndex struct {
	// CaseID identifies the case this index was built for.
	CaseID string

	// Paths is every materialized Path for this case, in the order they
	// were built. byRoot and byKind are derived views over this slice.
	Paths []Path

	// byRoot maps a root node id to the indices (into Paths) of every Path
	// rooted at that node.
	byRoot map[string][]int

	// byKind maps a PathKind to the indices (into Paths) of every Path of
	// that kind.
	byKind map[PathKind][]int
}

// newPathIndex constructs an empty PathIndex for caseID.
func newPathIndex(caseID string) *PathIndex {
	return &PathIndex{
		CaseID: caseID,
		byRoot: make(map[string][]int),
		byKind: make(map[PathKind][]int),
	}
}

// add appends p to the index and updates the byRoot/byKind views.
func (idx *PathIndex) add(p Path) {
	i := len(idx.Paths)
	idx.Paths = append(idx.Paths, p)

	root := p.RootID()
	if root != "" {
		idx.byRoot[root] = append(idx.byRoot[root], i)
	}
	idx.byKind[p.Kind] = append(idx.byKind[p.Kind], i)
}

// PathsFromRoot returns every Path rooted at nodeID, in build order.
func (idx *PathIndex) PathsFromRoot(nodeID string) []Path {
	if idx == nil {
		return nil
	}
	positions := idx.byRoot[nodeID]
	out := make([]Path, 0, len(positions))
	for _, i := range positions {
		out = append(out, idx.Paths[i])
	}
	return out
}

// PathsOfKind returns every Path of the given kind, in build order.
func (idx *PathIndex) PathsOfKind(kind PathKind) []Path {
	if idx == nil {
		return nil
	}
	positions := idx.byKind[kind]
	out := make([]Path, 0, len(positions))
	for _, i := range positions {
		out = append(out, idx.Paths[i])
	}
	return out
}

// Len returns the total number of materialized Paths in the index.
func (idx *PathIndex) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.Paths)
}
