package graph

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// TenantID identifies the tenant a node/edge belongs to. Defined locally
// here (rather than imported from packages/tenancy) so this package has
// no hard dependency on packages/tenancy — mirroring packages/irac's
// convention of keeping cross-cutting identifiers as opaque local types
// (see packages/irac/jurisdiction.go's JurisdictionCode/LegalFamily).
type TenantID string

// tenantOf extracts the TenantID a node belongs to. irac.Node has no
// native tenant field (packages/irac has no dependency on tenancy
// concepts), so TenantScopedStore tracks tenant ownership itself,
// keyed by node id, rather than deriving it from the node's own fields.
//
// TenantScopedStore wraps an inner GraphStore and enforces that every
// node/edge write and read is scoped to a single tenant. Cross-tenant
// reads/writes are rejected with ErrCrossTenantAccess rather than
// silently filtered, so a caller cannot mistake "no data" for "no
// access".
type TenantScopedStore struct {
	inner  GraphStore
	tenant TenantID

	// owners maps node id -> owning TenantID, tracked by this wrapper
	// since the underlying GraphStore (and irac.Node itself) has no
	// notion of tenancy. This map is shared across every
	// TenantScopedStore wrapping the same inner store (see
	// NewTenantScopedStore), so tenant ownership is consistent
	// regardless of which tenant's view performed the write.
	owners map[string]TenantID
}

// NewTenantScopedStore wraps inner, scoping every operation to tenant.
// owners is a shared map of node id -> owning TenantID; pass the same
// map when constructing multiple TenantScopedStore values over the same
// inner store (one per tenant) so ownership tracked by one tenant's
// writes is visible to another tenant's cross-access checks. Passing nil
// allocates a fresh, unshared map (suitable for a single-tenant view or
// tests).
func NewTenantScopedStore(inner GraphStore, tenant TenantID, owners map[string]TenantID) *TenantScopedStore {
	if owners == nil {
		owners = make(map[string]TenantID)
	}
	return &TenantScopedStore{inner: inner, tenant: tenant, owners: owners}
}

// Owners returns the shared node-id -> TenantID ownership map backing
// this store, so callers can construct additional TenantScopedStore
// values (for other tenants) over the same inner GraphStore with
// consistent ownership tracking.
func (s *TenantScopedStore) Owners() map[string]TenantID {
	return s.owners
}

// CreateNode records node's ownership under this store's tenant and
// delegates to the inner store. Overwriting a node owned by a different
// tenant is rejected with ErrCrossTenantAccess.
func (s *TenantScopedStore) CreateNode(ctx context.Context, node irac.Node) error {
	if owner, ok := s.owners[node.ID]; ok && owner != s.tenant {
		return ErrCrossTenantAccess
	}
	if err := s.inner.CreateNode(ctx, node); err != nil {
		return err
	}
	s.owners[node.ID] = s.tenant
	return nil
}

// CreateEdge delegates to the inner store after verifying both of
// edge's endpoint nodes are owned by this store's tenant.
func (s *TenantScopedStore) CreateEdge(ctx context.Context, edge irac.Edge) error {
	if owner, ok := s.owners[edge.FromID]; ok && owner != s.tenant {
		return ErrCrossTenantAccess
	}
	if owner, ok := s.owners[edge.ToID]; ok && owner != s.tenant {
		return ErrCrossTenantAccess
	}
	return s.inner.CreateEdge(ctx, edge)
}

// GetNode returns the node with the given id, rejecting the read with
// ErrCrossTenantAccess if the node is owned by a different tenant.
func (s *TenantScopedStore) GetNode(ctx context.Context, id string) (irac.Node, error) {
	if owner, ok := s.owners[id]; ok && owner != s.tenant {
		return irac.Node{}, ErrCrossTenantAccess
	}
	return s.inner.GetNode(ctx, id)
}

// Traverse delegates to the inner store and then filters out any node
// not owned by this store's tenant, so a traversal can never leak
// another tenant's nodes even if the inner store's CaseID index spans
// tenants.
func (s *TenantScopedStore) Traverse(ctx context.Context, query TraversalQuery) ([]irac.Node, error) {
	nodes, err := s.inner.Traverse(ctx, query)
	if err != nil {
		return nil, err
	}

	out := make([]irac.Node, 0, len(nodes))
	for _, n := range nodes {
		if owner, ok := s.owners[n.ID]; !ok || owner == s.tenant {
			out = append(out, n)
		}
	}
	return out, nil
}

// DeleteTree deletes caseID's tree only if every node currently
// registered as belonging to caseID is owned by this store's tenant (or
// unowned). If any node in the case is owned by a different tenant,
// DeleteTree rejects the whole operation with ErrCrossTenantAccess
// rather than partially deleting only this tenant's nodes.
func (s *TenantScopedStore) DeleteTree(ctx context.Context, caseID string) error {
	nodes, err := s.inner.Traverse(ctx, TraversalQuery{CaseID: caseID})
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if owner, ok := s.owners[n.ID]; ok && owner != s.tenant {
			return ErrCrossTenantAccess
		}
	}

	if err := s.inner.DeleteTree(ctx, caseID); err != nil {
		return err
	}
	for _, n := range nodes {
		delete(s.owners, n.ID)
	}
	return nil
}
