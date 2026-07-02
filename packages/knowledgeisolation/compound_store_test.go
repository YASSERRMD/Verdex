package knowledgeisolation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

func TestNewCompoundScopedStore_Validation(t *testing.T) {
	t.Parallel()

	_, err := knowledgeisolation.NewCompoundScopedStore(nil, "tenant-a", nil, "case-a", nil)
	if !errors.Is(err, knowledgeisolation.ErrNilStore) {
		t.Fatalf("expected ErrNilStore, got %v", err)
	}

	inner := graph.NewInMemoryGraphStore()
	_, err = knowledgeisolation.NewCompoundScopedStore(inner, "tenant-a", nil, "", nil)
	if !errors.Is(err, knowledgeisolation.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestCompoundScopedStore_RejectsCrossTenantEvenWithinSameCaseID(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}

	tenantA, err := knowledgeisolation.NewCompoundScopedStore(inner, "tenant-a", owners, "case-shared", nil)
	if err != nil {
		t.Fatalf("NewCompoundScopedStore tenant-a: %v", err)
	}
	tenantB, err := knowledgeisolation.NewCompoundScopedStore(inner, "tenant-b", owners, "case-shared", nil)
	if err != nil {
		t.Fatalf("NewCompoundScopedStore tenant-b: %v", err)
	}

	ctx := context.Background()
	nodeA := irac.Node{ID: "node-a", Type: irac.NodeFact, CaseID: "case-shared"}
	if err := tenantA.CreateNode(ctx, nodeA); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// tenant-b uses the *same* CaseID value ("case-shared") as tenant-a,
	// proving compound scoping enforces the tenant boundary even when
	// two tenants' case identifiers collide.
	_, err = tenantB.GetNode(ctx, "node-a")
	if !errors.Is(err, graph.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}
}

func TestCompoundScopedStore_RejectsCrossCaseWithinSameTenant(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}

	caseA, err := knowledgeisolation.NewCompoundScopedStore(inner, "tenant-a", owners, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCompoundScopedStore case-a: %v", err)
	}
	caseB, err := knowledgeisolation.NewCompoundScopedStore(inner, "tenant-a", owners, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCompoundScopedStore case-b: %v", err)
	}

	ctx := context.Background()
	nodeA := irac.Node{ID: "node-a", Type: irac.NodeFact, CaseID: "case-a"}
	if err := caseA.CreateNode(ctx, nodeA); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Same tenant, different case: must be rejected by the case layer.
	_, err = caseB.GetNode(ctx, "node-a")
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}
}

func TestCompoundScopedStore_AllowsOwnTenantOwnCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCompoundScopedStore(inner, "tenant-a", nil, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCompoundScopedStore: %v", err)
	}

	ctx := context.Background()
	node := irac.Node{ID: "node-1", Type: irac.NodeFact, CaseID: "case-a"}
	if err := store.CreateNode(ctx, node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	got, err := store.GetNode(ctx, "node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.ID != "node-1" {
		t.Fatalf("GetNode returned %+v", got)
	}

	if store.TenantID() != "tenant-a" {
		t.Fatalf("TenantID() = %q, want tenant-a", store.TenantID())
	}
	if store.CaseID() != "case-a" {
		t.Fatalf("CaseID() = %q, want case-a", store.CaseID())
	}
}
