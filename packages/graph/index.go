package graph

// indexMigrations are the Cypher CREATE INDEX statements registered for
// the Neo4j-backed path, mirroring the secondary indexes
// InMemoryGraphStore keeps in memory (see inMemoryIndex below): one
// index on node type, one on case id, so Traverse's filters translate
// into index-backed Cypher lookups (MATCH (n:IracNode {type: $type,
// case_id: $caseID})) rather than full label scans.
func indexMigrations() []Migration {
	return []Migration{
		{
			Name:   "irac_node_type_index",
			Cypher: "CREATE INDEX irac_node_type_index IF NOT EXISTS FOR (n:IracNode) ON (n.type)",
		},
		{
			Name:   "irac_node_case_id_index",
			Cypher: "CREATE INDEX irac_node_case_id_index IF NOT EXISTS FOR (n:IracNode) ON (n.case_id)",
		},
	}
}

// inMemoryIndex holds InMemoryGraphStore's secondary indexes: node ids
// grouped by NodeType and by CaseID, so Traverse can look candidates up
// by either filter in O(1) instead of scanning every node.
//
// InMemoryGraphStore currently keeps its CaseID index inline (the
// byCase field in inmemory.go) because CaseID filtering is required on
// every Traverse call (TraversalQuery.CaseID is mandatory) and predates
// this file. inMemoryIndex adds the NodeType secondary index on top,
// and is the natural home for both indexes going forward.
type inMemoryIndex struct {
	byType map[string]map[string]struct{}
}

// newInMemoryIndex builds an empty inMemoryIndex.
func newInMemoryIndex() *inMemoryIndex {
	return &inMemoryIndex{byType: make(map[string]map[string]struct{})}
}

// addType registers nodeID under typeKey.
func (idx *inMemoryIndex) addType(typeKey, nodeID string) {
	set, ok := idx.byType[typeKey]
	if !ok {
		set = make(map[string]struct{})
		idx.byType[typeKey] = set
	}
	set[nodeID] = struct{}{}
}

// removeType unregisters nodeID from typeKey.
func (idx *inMemoryIndex) removeType(typeKey, nodeID string) {
	if set, ok := idx.byType[typeKey]; ok {
		delete(set, nodeID)
	}
}

// nodeIDsByType returns every node id registered under typeKey.
func (idx *inMemoryIndex) nodeIDsByType(typeKey string) []string {
	set := idx.byType[typeKey]
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}
