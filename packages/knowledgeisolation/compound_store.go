package knowledgeisolation

import (
	"github.com/YASSERRMD/verdex/packages/graph"
)

// CompoundScopedStore composes graph.TenantScopedStore's cross-tenant
// enforcement with CaseScopedStore's cross-case enforcement into a
// single call chain, so one GraphStore value enforces both isolation
// axes at once. This reuses each guard's own logic rather than
// reimplementing tenant checks here: CompoundScopedStore wraps a
// *graph.TenantScopedStore inside a *CaseScopedStore (tenant check
// first, then case check), documented explicitly since Go's type system
// does not otherwise make the wrap order visible to callers.
//
// Construct one directly with NewCompoundScopedStore, or compose the two
// guards yourself if you need a different layering:
//
//	tenantStore := graph.NewTenantScopedStore(inner, tenantID, owners)
//	caseStore, err := knowledgeisolation.NewCaseScopedStore(tenantStore, caseID, sink)
//
// Either order of composition (tenant-then-case, as above, or
// case-then-tenant) yields an equivalent isolation guarantee, since each
// guard only ever narrows what the other can see; NewCompoundScopedStore
// fixes tenant-then-case as the documented default so every caller in
// this codebase composes them the same way.
type CompoundScopedStore struct {
	*CaseScopedStore

	tenantStore *graph.TenantScopedStore
	tenant      graph.TenantID
}

// NewCompoundScopedStore wraps inner with a graph.TenantScopedStore
// scoped to tenant, then a CaseScopedStore scoped to caseID, so every
// operation must satisfy both the tenant boundary and the case boundary.
// owners is the shared node-id -> TenantID ownership map passed to
// graph.NewTenantScopedStore (see its doc comment); sink receives every
// detected cross-case access attempt (cross-tenant attempts surface as
// graph.ErrCrossTenantAccess from the tenant layer, which this
// constructor does not separately audit — see doc/knowledge-isolation.md
// for why tenant-layer auditing is out of scope for this package).
func NewCompoundScopedStore(inner graph.GraphStore, tenant graph.TenantID, owners map[string]graph.TenantID, caseID CaseID, sink AlertSink) (*CompoundScopedStore, error) {
	if inner == nil {
		return nil, ErrNilStore
	}
	tenantStore := graph.NewTenantScopedStore(inner, tenant, owners)
	caseStore, err := NewCaseScopedStore(tenantStore, caseID, sink)
	if err != nil {
		return nil, err
	}
	return &CompoundScopedStore{CaseScopedStore: caseStore, tenantStore: tenantStore, tenant: tenant}, nil
}

// TenantID returns the tenant this compound store's tenant layer is
// scoped to.
func (s *CompoundScopedStore) TenantID() graph.TenantID {
	return s.tenant
}

// Ensure CompoundScopedStore satisfies graph.GraphStore. Every call is
// served by the embedded *CaseScopedStore, which itself calls into the
// wrapped *graph.TenantScopedStore, which calls into inner — so
// CompoundScopedStore needs no CreateNode/CreateEdge/GetNode/Traverse/
// DeleteTree overrides of its own.
var _ graph.GraphStore = (*CompoundScopedStore)(nil)
