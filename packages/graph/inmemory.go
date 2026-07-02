package graph

import (
	"context"
	"sync"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// InMemoryGraphStore is a fully in-memory GraphStore implementation
// backed by maps. It is the default implementation used by unit tests
// and by downstream packages (033-040) that do not need a live Neo4j
// instance: everything in this package that operates purely in terms of
// the GraphStore interface (TenantScopedStore, WithTransaction, Export/
// Import, HealthCheck) works identically against InMemoryGraphStore or a
// future Neo4j-backed store.
//
// InMemoryGraphStore is safe for concurrent use: all access to its
// internal maps is serialized by mu.
type InMemoryGraphStore struct {
	mu sync.RWMutex

	// nodes maps node id -> node.
	nodes map[string]irac.Node

	// edges maps case id -> edges belonging to that case's tree. Edges
	// are case-scoped via their endpoint nodes' CaseID, resolved at
	// CreateEdge time.
	edges map[string][]irac.Edge

	// byCase maps case id -> set of node ids belonging to that case,
	// backing Traverse's mandatory CaseID filter.
	byCase map[string]map[string]struct{}

	// typeIndex is the NodeType secondary index (index.go), backing
	// Traverse's optional NodeType filter without a full scan.
	typeIndex *inMemoryIndex
}

// NewInMemoryGraphStore constructs an empty InMemoryGraphStore.
func NewInMemoryGraphStore() *InMemoryGraphStore {
	return &InMemoryGraphStore{
		nodes:     make(map[string]irac.Node),
		edges:     make(map[string][]irac.Edge),
		byCase:    make(map[string]map[string]struct{}),
		typeIndex: newInMemoryIndex(),
	}
}

// CreateNode persists node, overwriting any existing node with the same
// ID (idempotent upsert — see GraphStore.CreateNode).
func (s *InMemoryGraphStore) CreateNode(_ context.Context, node irac.Node) error {
	if node.ID == "" {
		return ErrEmptyNodeID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.nodes[node.ID]; ok {
		if existing.CaseID != node.CaseID {
			s.removeFromCaseIndexLocked(existing)
		}
		if existing.Type != node.Type {
			s.typeIndex.removeType(string(existing.Type), existing.ID)
		}
	}
	s.nodes[node.ID] = node
	s.addToCaseIndexLocked(node)
	s.typeIndex.addType(string(node.Type), node.ID)
	return nil
}

// addToCaseIndexLocked registers node under its CaseID in byCase.
// Callers must hold s.mu for writing.
func (s *InMemoryGraphStore) addToCaseIndexLocked(node irac.Node) {
	set, ok := s.byCase[node.CaseID]
	if !ok {
		set = make(map[string]struct{})
		s.byCase[node.CaseID] = set
	}
	set[node.ID] = struct{}{}
}

// removeFromCaseIndexLocked unregisters node from byCase. Callers must
// hold s.mu for writing.
func (s *InMemoryGraphStore) removeFromCaseIndexLocked(node irac.Node) {
	if set, ok := s.byCase[node.CaseID]; ok {
		delete(set, node.ID)
	}
}

// CreateEdge persists edge under the case of its endpoint nodes. Both
// endpoints must already exist via CreateNode and must belong to the
// same case, or CreateEdge returns ErrNodeNotFound.
func (s *InMemoryGraphStore) CreateEdge(_ context.Context, edge irac.Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	from, ok := s.nodes[edge.FromID]
	if !ok {
		return ErrNodeNotFound
	}
	to, ok := s.nodes[edge.ToID]
	if !ok {
		return ErrNodeNotFound
	}

	caseID := from.CaseID
	if to.CaseID != caseID {
		// Edges must stay within a single case's tree; there is no
		// legal cross-case edge in the IRAC schema.
		return ErrNodeNotFound
	}

	s.edges[caseID] = append(s.edges[caseID], edge)
	return nil
}

// GetNode returns the node with the given id, or ErrNodeNotFound.
func (s *InMemoryGraphStore) GetNode(_ context.Context, id string) (irac.Node, error) {
	if id == "" {
		return irac.Node{}, ErrEmptyNodeID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[id]
	if !ok {
		return irac.Node{}, ErrNodeNotFound
	}
	return node, nil
}

// Traverse returns every node matching query, using the secondary
// indexes (byCase, typeIndex — see index.go) for the CaseID/NodeType
// filters and a breadth-first edge walk when FromNodeID is set.
func (s *InMemoryGraphStore) Traverse(_ context.Context, query TraversalQuery) ([]irac.Node, error) {
	if query.CaseID == "" {
		return nil, ErrEmptyCaseID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	candidateIDs := s.caseNodeIDsLocked(query.CaseID)

	if query.NodeType != "" {
		candidateIDs = intersect(candidateIDs, s.typeIndex.nodeIDsByType(string(query.NodeType)))
	}

	if query.FromNodeID != "" {
		reachable := s.reachableIDsLocked(query.CaseID, query.FromNodeID, query.MaxDepth)
		candidateIDs = intersect(candidateIDs, reachable)
	}

	out := make([]irac.Node, 0, len(candidateIDs))
	for _, id := range candidateIDs {
		if node, ok := s.nodes[id]; ok {
			out = append(out, node)
		}
	}
	return out, nil
}

// caseNodeIDsLocked returns every node id registered under caseID.
// Callers must hold s.mu (read or write).
func (s *InMemoryGraphStore) caseNodeIDsLocked(caseID string) []string {
	set := s.byCase[caseID]
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// reachableIDsLocked returns the set of node IDs reachable from
// fromNodeID by following outward edges within caseID, breadth-first,
// bounded by maxDepth hops (0 means unbounded). fromNodeID itself is
// included. Callers must hold s.mu (read or write).
func (s *InMemoryGraphStore) reachableIDsLocked(caseID, fromNodeID string, maxDepth int) []string {
	visited := map[string]struct{}{fromNodeID: {}}
	frontier := []string{fromNodeID}
	depth := 0

	for len(frontier) > 0 {
		if maxDepth > 0 && depth >= maxDepth {
			break
		}
		var next []string
		for _, id := range frontier {
			for _, e := range s.edges[caseID] {
				if e.FromID != id {
					continue
				}
				if _, seen := visited[e.ToID]; seen {
					continue
				}
				visited[e.ToID] = struct{}{}
				next = append(next, e.ToID)
			}
		}
		frontier = next
		depth++
	}

	out := make([]string, 0, len(visited))
	for id := range visited {
		out = append(out, id)
	}
	return out
}

// DeleteTree removes every node and edge belonging to caseID.
func (s *InMemoryGraphStore) DeleteTree(_ context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range s.caseNodeIDsLocked(caseID) {
		if node, ok := s.nodes[id]; ok {
			s.typeIndex.removeType(string(node.Type), id)
		}
		delete(s.nodes, id)
	}
	delete(s.byCase, caseID)
	delete(s.edges, caseID)
	return nil
}

// intersect returns the elements present in both a and b.
func intersect(a, b []string) []string {
	bSet := make(map[string]struct{}, len(b))
	for _, id := range b {
		bSet[id] = struct{}{}
	}
	out := make([]string, 0, len(a))
	for _, id := range a {
		if _, ok := bSet[id]; ok {
			out = append(out, id)
		}
	}
	return out
}
