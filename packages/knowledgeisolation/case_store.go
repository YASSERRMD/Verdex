package knowledgeisolation

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// CaseScopedStore wraps any graph.GraphStore and enforces cross-CASE
// isolation within a single tenant, analogous to graph.TenantScopedStore's
// cross-tenant enforcement. graph.GraphStore's Traverse/GetNode/CreateEdge
// have no case-boundary enforcement of their own: any caller holding a
// node ID from case A can pass it to a query scoped to case B without the
// interface stopping them. CaseScopedStore closes that gap.
//
// Unlike TenantScopedStore, CaseScopedStore does not need a separate
// ownership-tracking map: irac.Node already carries its owning CaseID
// directly (irac.Node.CaseID), so ownership is read straight off the
// node rather than tracked out-of-band. The one exception is
// ScopeSharedLaw nodes (irac.NodeRule): those are readable across every
// case in the tenant regardless of the CaseID they carry (see scope.go).
//
// CaseScopedStore rejects cross-case access with ErrCrossCaseAccess
// rather than silently filtering it, mirroring TenantScopedStore's
// "never mistake no access for no data" philosophy — with one
// deliberate exception: Traverse filters rather than errors (see
// Traverse's doc comment), matching TenantScopedStore.Traverse's own
// precedent, while still recording every filtered node as an
// AccessAttempt for audit.
type CaseScopedStore struct {
	inner   graph.GraphStore
	caseID  CaseID
	auditor *auditRecorder
}

// NewCaseScopedStore wraps inner, scoping every operation to caseID.
// sink receives every detected cross-case access attempt; pass nil to
// use NoOpAlertSink. Returns ErrNilStore if inner is nil, or
// ErrEmptyCaseID if caseID is empty.
func NewCaseScopedStore(inner graph.GraphStore, caseID CaseID, sink AlertSink) (*CaseScopedStore, error) {
	if inner == nil {
		return nil, ErrNilStore
	}
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	return &CaseScopedStore{
		inner:   inner,
		caseID:  caseID,
		auditor: newAuditRecorder(sink, nil),
	}, nil
}

// CaseID returns the case this store is scoped to.
func (s *CaseScopedStore) CaseID() CaseID {
	return s.caseID
}

// AccessAttempts returns every cross-case access attempt recorded by
// this store so far, in recording order. Intended for tests and security
// review tooling; production alerting should instead be wired through
// the AlertSink passed to NewCaseScopedStore.
func (s *CaseScopedStore) AccessAttempts() []AccessAttempt {
	return s.auditor.snapshot()
}

// authorized reports whether node is readable/writable by this store's
// case: either node itself belongs to s.caseID, or node is part of the
// shared-law corpus (readable from any case).
func (s *CaseScopedStore) authorized(node irac.Node) bool {
	if IsSharedLawNode(node) {
		return true
	}
	return node.CaseID == s.caseID
}

// CreateNode delegates to the inner store after verifying node either
// belongs to this store's case or is a shared-law node. A case-scoped
// node belonging to a different case is rejected with
// ErrCrossCaseAccess.
func (s *CaseScopedStore) CreateNode(ctx context.Context, node irac.Node) error {
	if !s.authorized(node) {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationGetNode,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   node.CaseID,
			NodeID:          node.ID,
			Detail:          fmt.Sprintf("CreateNode: node %q belongs to case %q, not authorized case %q", node.ID, node.CaseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	return s.inner.CreateNode(ctx, node)
}

// CreateEdge delegates to the inner store after verifying both of edge's
// endpoint nodes are either owned by this store's case or are shared-law
// nodes. An edge whose endpoints span this case and a different case is
// rejected with ErrCrossCaseAccess, since that is exactly the leakage
// vector this guard exists to close: linking case-A facts into case-B's
// reasoning tree via a single shared edge.
func (s *CaseScopedStore) CreateEdge(ctx context.Context, edge irac.Edge) error {
	from, err := s.inner.GetNode(ctx, edge.FromID)
	if err != nil {
		return err
	}
	to, err := s.inner.GetNode(ctx, edge.ToID)
	if err != nil {
		return err
	}

	if !s.authorized(from) {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationCreateEdge,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   from.CaseID,
			NodeID:          from.ID,
			Detail:          fmt.Sprintf("CreateEdge: from-node %q belongs to case %q, not authorized case %q", from.ID, from.CaseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	if !s.authorized(to) {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationCreateEdge,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   to.CaseID,
			NodeID:          to.ID,
			Detail:          fmt.Sprintf("CreateEdge: to-node %q belongs to case %q, not authorized case %q", to.ID, to.CaseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	return s.inner.CreateEdge(ctx, edge)
}

// GetNode returns the node with the given id, rejecting the read with
// ErrCrossCaseAccess if the node is a case-scoped node owned by a
// different case. Shared-law nodes are always returned regardless of
// which case they were originally attributed to.
func (s *CaseScopedStore) GetNode(ctx context.Context, id string) (irac.Node, error) {
	node, err := s.inner.GetNode(ctx, id)
	if err != nil {
		return irac.Node{}, err
	}
	if !s.authorized(node) {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationGetNode,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   node.CaseID,
			NodeID:          node.ID,
			Detail:          fmt.Sprintf("GetNode: node %q belongs to case %q, not authorized case %q", node.ID, node.CaseID, s.caseID),
		})
		return irac.Node{}, ErrCrossCaseAccess
	}
	return node, nil
}

// Traverse delegates to the inner store and then filters out any
// case-scoped node not owned by this store's case, so a traversal can
// never leak another case's facts even if the query itself (e.g. a
// FromNodeID seed belonging to another case) was mistakenly constructed
// against this store's case. Shared-law nodes always pass the filter.
//
// Traverse filters rather than rejecting outright — mirroring
// graph.TenantScopedStore.Traverse's own precedent — because a
// traversal's result set is expected to legitimately mix case-owned and
// shared-law nodes; the interesting security property is that no
// foreign case-owned node ever appears in the output, not that the
// query must be rejected wholesale. Every filtered-out node is still
// recorded as an AccessAttempt.
func (s *CaseScopedStore) Traverse(ctx context.Context, query graph.TraversalQuery) ([]irac.Node, error) {
	nodes, err := s.inner.Traverse(ctx, query)
	if err != nil {
		return nil, err
	}

	out := make([]irac.Node, 0, len(nodes))
	for _, n := range nodes {
		if s.authorized(n) {
			out = append(out, n)
			continue
		}
		s.auditor.record(AccessAttempt{
			Kind:            ViolationTraverse,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   n.CaseID,
			NodeID:          n.ID,
			Detail:          fmt.Sprintf("Traverse: filtered node %q belonging to case %q, not authorized case %q", n.ID, n.CaseID, s.caseID),
		})
	}
	return out, nil
}

// DeleteTree deletes caseID's tree only if caseID matches this store's
// authorized case. Deleting a different case's tree is rejected with
// ErrCrossCaseAccess without touching the inner store.
func (s *CaseScopedStore) DeleteTree(ctx context.Context, caseID string) error {
	if caseID != s.caseID {
		s.auditor.record(AccessAttempt{
			Kind:            ViolationDeleteTree,
			AuthorizedCases: []CaseID{s.caseID},
			AttemptedCase:   caseID,
			Detail:          fmt.Sprintf("DeleteTree: requested case %q, not authorized case %q", caseID, s.caseID),
		})
		return ErrCrossCaseAccess
	}
	return s.inner.DeleteTree(ctx, caseID)
}

// Ensure CaseScopedStore satisfies graph.GraphStore.
var _ graph.GraphStore = (*CaseScopedStore)(nil)
