package knowledgeapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// edgeLister is implemented by a graph.GraphStore that can return every
// irac.Edge belonging to a case directly (graph.InMemoryGraphStore does,
// via EdgesForCase). This mirrors packages/traversal's and
// packages/treeindex's own opportunistic capability-detection pattern for
// edge listing rather than widening graph.GraphStore's interface just for
// this convenience; see traversal/edges.go for the precedent.
type edgeLister interface {
	EdgesForCase(caseID string) []irac.Edge
}

// GetTree returns every node and edge in this KnowledgeAPI's case,
// optionally filtered by node type, paginated over the node list. Reads
// are served through the case-scoped GraphStore (Traverse), so a node
// belonging to a different case can never appear in Nodes even if the
// underlying store were somehow asked for one — knowledgeisolation.
// CaseScopedStore.Traverse filters, it does not merely reject.
//
// Edges returned are only those whose FromID and ToID both resolve to a
// node present in the (unpaginated) node set for this case, so the edge
// list never dangles past a node this case cannot see.
func (api *KnowledgeAPI) GetTree(ctx context.Context, req GetTreeRequest) (GetTreeResponse, error) {
	if _, err := authorize(ctx); err != nil {
		return GetTreeResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return GetTreeResponse{}, ErrEmptyCaseID
	}

	page, perPage, err := normalizePage(req.Page)
	if err != nil {
		return GetTreeResponse{}, err
	}

	query := graph.TraversalQuery{CaseID: api.caseID}
	if req.NodeTypeFilter != "" {
		query.NodeType = irac.NodeType(req.NodeTypeFilter)
	}

	nodes, err := api.store.Traverse(ctx, query)
	if err != nil {
		return GetTreeResponse{}, err
	}

	nodeIDs := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeIDs[n.ID] = struct{}{}
	}

	edges, err := loadCaseEdges(ctx, api.store, api.caseID)
	if err != nil {
		return GetTreeResponse{}, err
	}

	edgeDTOs := make([]EdgeDTO, 0, len(edges))
	for _, e := range edges {
		_, fromOK := nodeIDs[e.FromID]
		_, toOK := nodeIDs[e.ToID]
		if fromOK && toOK {
			edgeDTOs = append(edgeDTOs, edgeDTOFromEdge(e))
		}
	}

	pageNodes, meta := paginate(nodes, page, perPage)
	nodeDTOs := make([]NodeDTO, 0, len(pageNodes))
	for _, n := range pageNodes {
		nodeDTOs = append(nodeDTOs, nodeDTOFromNode(n))
	}

	return GetTreeResponse{
		Version: APIVersionV1,
		CaseID:  api.caseID,
		Nodes:   nodeDTOs,
		Edges:   edgeDTOs,
		Meta:    meta,
	}, nil
}

// loadCaseEdges returns every irac.Edge belonging to caseID, preferring a
// store's direct edgeLister capability and falling back to a
// Traverse-based one-hop-per-node reconstruction otherwise (mirroring
// packages/traversal.loadCaseEdges's identical fallback). The fallback
// path cannot recover exact EdgeType fidelity (Traverse reports reachable
// nodes, not typed edges), so reconstructed edges carry an empty
// EdgeType; a store implementing edgeLister (e.g.
// graph.InMemoryGraphStore) does not have this limitation.
func loadCaseEdges(ctx context.Context, store *knowledgeisolation.CaseScopedStore, caseID string) ([]irac.Edge, error) {
	if lister, ok := any(store).(edgeLister); ok {
		return lister.EdgesForCase(caseID), nil
	}

	nodes, err := store.Traverse(ctx, graph.TraversalQuery{CaseID: caseID})
	if err != nil {
		return nil, fmt.Errorf("knowledgeapi: load edges for case %q: traverse case: %w", caseID, err)
	}

	var edges []irac.Edge
	seen := make(map[irac.Edge]struct{})
	for _, n := range nodes {
		neighbors, err := store.Traverse(ctx, graph.TraversalQuery{
			CaseID:     caseID,
			FromNodeID: n.ID,
			MaxDepth:   1,
		})
		if err != nil {
			return nil, fmt.Errorf("knowledgeapi: load edges for case %q: traverse from node %q: %w", caseID, n.ID, err)
		}
		for _, neighbor := range neighbors {
			if neighbor.ID == n.ID {
				continue
			}
			e := irac.Edge{FromID: n.ID, ToID: neighbor.ID}
			if _, ok := seen[e]; ok {
				continue
			}
			seen[e] = struct{}{}
			edges = append(edges, e)
		}
	}
	return edges, nil
}

// GetNode returns a single node by ID within this KnowledgeAPI's case.
// The read is served through the case-scoped GraphStore, so a request for
// a node belonging to a different case is rejected with
// knowledgeisolation.ErrCrossCaseAccess (shared-law nodes are always
// readable regardless of case, per knowledgeisolation's design).
func (api *KnowledgeAPI) GetNode(ctx context.Context, req GetNodeRequest) (GetNodeResponse, error) {
	if _, err := authorize(ctx); err != nil {
		return GetNodeResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return GetNodeResponse{}, ErrEmptyCaseID
	}
	if req.NodeID == "" {
		return GetNodeResponse{}, ErrEmptyNodeID
	}

	node, err := api.store.GetNode(ctx, req.NodeID)
	if err != nil {
		return GetNodeResponse{}, err
	}

	return GetNodeResponse{
		Version: APIVersionV1,
		Node:    nodeDTOFromNode(node),
	}, nil
}

// LookupPaths returns treeindex's materialized paths from req.FromNodeID
// following req.EdgeType, paginated. When req.MaxDepth is positive,
// treeindex.Indexer.LookupPathsWithDepth is used instead of LookupPaths so
// each returned path is truncated to that depth.
//
// treeindex.Indexer requires a case to be built via RebuildCase before
// LookupPaths will serve it (see treeindex.ErrCaseNotIndexed); this
// facade hides that lifecycle detail from callers by transparently
// calling RebuildCase once, on first use, when the underlying Indexer
// reports the case has never been indexed.
func (api *KnowledgeAPI) LookupPaths(ctx context.Context, req LookupPathsRequest) (LookupPathsResponse, error) {
	if _, err := authorize(ctx); err != nil {
		return LookupPathsResponse{}, err
	}
	if req.CaseID == "" || req.CaseID != api.caseID {
		return LookupPathsResponse{}, ErrEmptyCaseID
	}
	if req.FromNodeID == "" {
		return LookupPathsResponse{}, ErrEmptyNodeID
	}

	page, perPage, err := normalizePage(req.Page)
	if err != nil {
		return LookupPathsResponse{}, err
	}

	edgeType := irac.EdgeType(req.EdgeType)

	paths, err := api.lookupPaths(ctx, req.FromNodeID, edgeType, req.MaxDepth)
	if errors.Is(err, treeindex.ErrCaseNotIndexed) {
		if rebuildErr := api.indexer.RebuildCase(ctx, api.caseID); rebuildErr != nil {
			return LookupPathsResponse{}, rebuildErr
		}
		paths, err = api.lookupPaths(ctx, req.FromNodeID, edgeType, req.MaxDepth)
	}
	if err != nil {
		return LookupPathsResponse{}, err
	}

	pagePaths, meta := paginate(paths, page, perPage)
	pathDTOs := make([]PathDTO, 0, len(pagePaths))
	for _, p := range pagePaths {
		pathDTOs = append(pathDTOs, pathDTOFromPath(p))
	}

	return LookupPathsResponse{
		Version: APIVersionV1,
		CaseID:  api.caseID,
		Paths:   pathDTOs,
		Meta:    meta,
	}, nil
}

// lookupPaths calls the appropriate treeindex.Indexer lookup method
// depending on whether maxDepth was requested.
func (api *KnowledgeAPI) lookupPaths(ctx context.Context, fromNodeID string, edgeType irac.EdgeType, maxDepth int) ([]treeindex.Path, error) {
	if maxDepth > 0 {
		return api.indexer.LookupPathsWithDepth(ctx, api.caseID, fromNodeID, edgeType, maxDepth)
	}
	return api.indexer.LookupPaths(ctx, api.caseID, fromNodeID, edgeType)
}

// pathDTOFromPath converts a treeindex.Path into a PathDTO.
func pathDTOFromPath(p treeindex.Path) PathDTO {
	nodes := make([]NodeDTO, 0, len(p.Nodes))
	for _, ref := range p.Nodes {
		nodes = append(nodes, NodeDTO{
			Version: APIVersionV1,
			ID:      ref.ID,
			Type:    string(ref.Type),
			CaseID:  ref.CaseID,
			Text:    ref.Text,
		})
	}

	hops := make([]PathHopDTO, 0, len(p.Hops))
	for i, hop := range p.Hops {
		nodeIndex := i + 1
		if hop.FromIndex >= len(p.Nodes) || nodeIndex >= len(p.Nodes) {
			continue
		}
		from := p.Nodes[hop.FromIndex].ID
		to := p.Nodes[nodeIndex].ID
		if hop.Reverse {
			from, to = to, from
		}
		hops = append(hops, PathHopDTO{
			FromID:   from,
			ToID:     to,
			EdgeType: string(hop.EdgeType),
		})
	}

	return PathDTO{
		Kind:  string(p.Kind),
		Root:  p.RootID(),
		Nodes: nodes,
		Hops:  hops,
	}
}
