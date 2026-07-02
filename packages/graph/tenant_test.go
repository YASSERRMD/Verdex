package graph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestTenantScopedStore_RejectsCrossTenantGet(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	tenantB := graph.NewTenantScopedStore(inner, "tenant-b", owners)
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if _, err := tenantA.GetNode(ctx, "n1"); err != nil {
		t.Fatalf("expected owning tenant to read its own node, got %v", err)
	}

	if _, err := tenantB.GetNode(ctx, "n1"); !errors.Is(err, graph.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess for cross-tenant GetNode, got %v", err)
	}
}

func TestTenantScopedStore_RejectsCrossTenantOverwrite(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	tenantB := graph.NewTenantScopedStore(inner, "tenant-b", owners)
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	err := tenantB.CreateNode(ctx, n)
	if !errors.Is(err, graph.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess overwriting another tenant's node, got %v", err)
	}
}

func TestTenantScopedStore_RejectsCrossTenantEdge(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	tenantB := graph.NewTenantScopedStore(inner, "tenant-b", owners)
	ctx := context.Background()

	ruleA := testNode("rule-a", "case1", irac.NodeRule)
	issueA := testNode("issue-a", "case1", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, ruleA); err != nil {
		t.Fatalf("CreateNode ruleA: %v", err)
	}
	if err := tenantA.CreateNode(ctx, issueA); err != nil {
		t.Fatalf("CreateNode issueA: %v", err)
	}

	err := tenantB.CreateEdge(ctx, irac.Edge{FromID: "rule-a", ToID: "issue-a", Type: irac.EdgeGoverns})
	if !errors.Is(err, graph.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess creating an edge over another tenant's nodes, got %v", err)
	}
}

func TestTenantScopedStore_TraverseFiltersOtherTenants(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	tenantB := graph.NewTenantScopedStore(inner, "tenant-b", owners)
	ctx := context.Background()

	// Both tenants happen to write into nodes under the same CaseID
	// value; TenantScopedStore's Traverse must still only return the
	// calling tenant's own nodes, proving isolation does not depend on
	// case ids never colliding across tenants.
	nodeA := testNode("node-a", "shared-case", irac.NodeIssue)
	nodeB := testNode("node-b", "shared-case", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, nodeA); err != nil {
		t.Fatalf("CreateNode nodeA: %v", err)
	}
	if err := tenantB.CreateNode(ctx, nodeB); err != nil {
		t.Fatalf("CreateNode nodeB: %v", err)
	}

	nodes, err := tenantA.Traverse(ctx, graph.TraversalQuery{CaseID: "shared-case"})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "node-a" {
		t.Fatalf("expected tenant-a's Traverse to only see node-a, got %+v", nodes)
	}
}

func TestTenantScopedStore_DeleteTree_RejectsCrossTenant(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	tenantB := graph.NewTenantScopedStore(inner, "tenant-b", owners)
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	err := tenantB.DeleteTree(ctx, "case1")
	if !errors.Is(err, graph.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess deleting another tenant's tree, got %v", err)
	}

	// The node must still be present: DeleteTree must not partially
	// delete anything when it rejects.
	if _, err := tenantA.GetNode(ctx, "n1"); err != nil {
		t.Fatalf("expected n1 to survive the rejected cross-tenant delete, got %v", err)
	}
}

func TestTenantScopedStore_DeleteTree_OwnTenant(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	owners := map[string]graph.TenantID{}
	tenantA := graph.NewTenantScopedStore(inner, "tenant-a", owners)
	ctx := context.Background()

	n := testNode("n1", "case1", irac.NodeIssue)
	if err := tenantA.CreateNode(ctx, n); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if err := tenantA.DeleteTree(ctx, "case1"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	if _, err := tenantA.GetNode(ctx, "n1"); !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected n1 to be gone after owning tenant's DeleteTree, got %v", err)
	}
}
