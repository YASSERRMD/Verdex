package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// Export serializes every node and edge belonging to caseID in store
// into a lossless JSON envelope, reusing irac.MarshalTree as the wire
// format so a backup produced here is byte-for-byte the same shape
// packages/irac itself would produce for the same tree contents.
//
// GraphStore.CreateNode/GetNode/Traverse all operate on the base
// irac.Node shape rather than irac's concrete typed wrappers
// (IssueNode, RuleNode, ...), since those wrappers carry fields
// (Spans, JurisdictionCode, Label, ...) this package's GraphStore
// interface does not persist. Export bridges that gap by reconstructing
// the minimal typed wrapper implied by each node's NodeType before
// handing it to irac.MarshalTree — for NodeConclusion this means
// attaching the mandatory draft_analysis guardrail label via
// irac.NewConclusionNode, since irac.MarshalTree refuses to encode a
// ConclusionNode without it.
func Export(ctx context.Context, store GraphStore, caseID string) ([]byte, error) {
	if store == nil {
		return nil, fmt.Errorf("graph: Export: store must not be nil")
	}
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}

	nodes, err := store.Traverse(ctx, TraversalQuery{CaseID: caseID})
	if err != nil {
		return nil, fmt.Errorf("graph: Export: %w", err)
	}

	edges, err := collectEdges(ctx, store, nodes)
	if err != nil {
		return nil, fmt.Errorf("graph: Export: %w", err)
	}

	nodeLikes := make([]irac.NodeLike, 0, len(nodes))
	for _, n := range nodes {
		nodeLikes = append(nodeLikes, toNodeLike(n))
	}

	revision := irac.NewInitialRevision(caseID, latestCreatedAt(nodes))

	data, err := irac.MarshalTree(nodeLikes, edges, revision)
	if err != nil {
		return nil, fmt.Errorf("graph: Export: %w", err)
	}
	return data, nil
}

// Import decodes a JSON envelope produced by Export (or by
// irac.MarshalTree over the same node/edge shapes) and writes every
// node and edge it contains into store.
func Import(ctx context.Context, store GraphStore, data []byte) error {
	if store == nil {
		return fmt.Errorf("graph: Import: store must not be nil")
	}

	nodeLikes, edges, _, err := irac.UnmarshalTree(data)
	if err != nil {
		return fmt.Errorf("graph: Import: %w", err)
	}

	for _, nl := range nodeLikes {
		if err := store.CreateNode(ctx, toNode(nl)); err != nil {
			return fmt.Errorf("graph: Import: create node %q: %w", nl.GetID(), err)
		}
	}
	for _, e := range edges {
		if err := store.CreateEdge(ctx, e); err != nil {
			return fmt.Errorf("graph: Import: create edge %s->%s: %w", e.FromID, e.ToID, err)
		}
	}
	return nil
}

// collectEdges gathers every edge belonging to nodes' case. The
// GraphStore interface itself has no "list edges" method (only
// CreateEdge), so this opportunistically type-asserts for an
// EdgesForCase(caseID) accessor that concrete implementations may
// expose (InMemoryGraphStore does; a Neo4j-backed store would answer
// this with a Cypher MATCH instead). If store does not implement that
// accessor, Export proceeds with zero edges rather than failing, since
// nodes are still a valid (if edge-less) backup.
func collectEdges(_ context.Context, store GraphStore, nodes []irac.Node) ([]irac.Edge, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	edgeSource, ok := store.(interface {
		EdgesForCase(caseID string) []irac.Edge
	})
	if !ok {
		return nil, nil
	}
	return edgeSource.EdgesForCase(nodes[0].CaseID), nil
}

// toNodeLike reconstructs the minimal concrete NodeLike wrapper implied
// by n's NodeType, so it can be passed to irac.MarshalTree.
func toNodeLike(n irac.Node) irac.NodeLike {
	switch n.Type {
	case irac.NodeIssue:
		return irac.NewIssueNode(n.ID, n.CaseID, n.Text, n.CreatedAt, n.Confidence, n.Provenance)
	case irac.NodeRule:
		return irac.NewRuleNode(n.ID, n.CaseID, n.Text, "", "", n.CreatedAt, n.Confidence, n.Provenance)
	case irac.NodeFact:
		return irac.NewFactNode(n.ID, n.CaseID, n.Text, n.CreatedAt, n.Confidence, n.Provenance)
	case irac.NodeApplication:
		return irac.NewApplicationNode(n.ID, n.CaseID, n.Text, n.CreatedAt, n.Confidence, n.Provenance)
	case irac.NodeConclusion:
		return irac.NewConclusionNode(n.ID, n.CaseID, n.Text, n.CreatedAt, n.Confidence, n.Provenance)
	default:
		return n
	}
}

// toNode flattens any NodeLike back down to the base irac.Node shape
// GraphStore persists.
func toNode(nl irac.NodeLike) irac.Node {
	switch v := nl.(type) {
	case irac.IssueNode:
		return v.Node
	case irac.RuleNode:
		return v.Node
	case irac.FactNode:
		return v.Node
	case irac.ApplicationNode:
		return v.Node
	case irac.ConclusionNode:
		return v.Node
	case irac.Node:
		return v
	default:
		return irac.Node{ID: nl.GetID(), Type: nl.GetType()}
	}
}

// latestCreatedAt returns the most recent CreatedAt among nodes, or the
// zero time if nodes is empty.
func latestCreatedAt(nodes []irac.Node) (latest time.Time) {
	for _, n := range nodes {
		if n.CreatedAt.After(latest) {
			latest = n.CreatedAt
		}
	}
	return latest
}
