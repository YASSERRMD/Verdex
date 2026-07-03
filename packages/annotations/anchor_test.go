package annotations_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_ListByCase_FiltersByAnchor proves that ListByCase's
// AnchorFilter correctly narrows results to a given anchor type, and
// further to a specific anchor ID within that type — the "get all
// annotations for a given tree node or segment" contract task 9
// requires.
func TestService_ListByCase_FiltersByAnchor(t *testing.T) {
	svc, c, tenantID := newTestService(t)
	author := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(author)

	caseLevel, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "case-level note",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create case-level: %v", err)
	}
	nodeA, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "issue on node A",
		AnchorType: annotations.AnchorTreeNode,
		AnchorID:   "node-a",
	})
	if err != nil {
		t.Fatalf("Create tree node A: %v", err)
	}
	nodeB, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "issue on node B",
		AnchorType: annotations.AnchorTreeNode,
		AnchorID:   "node-b",
	})
	if err != nil {
		t.Fatalf("Create tree node B: %v", err)
	}
	segment, err := svc.Create(ctx, tenantID, &annotations.Annotation{
		CaseID:     c.ID,
		Body:       "flag on evidence segment",
		AnchorType: annotations.AnchorEvidenceSegment,
		AnchorID:   "segment-1",
	})
	if err != nil {
		t.Fatalf("Create evidence segment: %v", err)
	}

	all, err := svc.ListByCase(ctx, tenantID, c.ID, annotations.AnchorFilter{})
	if err != nil {
		t.Fatalf("ListByCase (no filter): %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("len(all) = %d, want 4", len(all))
	}

	treeNodes, err := svc.ListByCase(ctx, tenantID, c.ID, annotations.AnchorFilter{Type: annotations.AnchorTreeNode})
	if err != nil {
		t.Fatalf("ListByCase (tree_node): %v", err)
	}
	if len(treeNodes) != 2 {
		t.Fatalf("len(treeNodes) = %d, want 2", len(treeNodes))
	}
	for _, a := range treeNodes {
		if a.AnchorType != annotations.AnchorTreeNode {
			t.Fatalf("unexpected AnchorType %s in tree_node filter result", a.AnchorType)
		}
	}

	onlyNodeA, err := svc.ListByCase(ctx, tenantID, c.ID, annotations.AnchorFilter{
		Type: annotations.AnchorTreeNode,
		ID:   "node-a",
	})
	if err != nil {
		t.Fatalf("ListByCase (tree_node, node-a): %v", err)
	}
	if len(onlyNodeA) != 1 || onlyNodeA[0].ID != nodeA.ID {
		t.Fatalf("onlyNodeA = %v, want exactly [%s]", onlyNodeA, nodeA.ID)
	}

	segments, err := svc.ListByCase(ctx, tenantID, c.ID, annotations.AnchorFilter{Type: annotations.AnchorEvidenceSegment})
	if err != nil {
		t.Fatalf("ListByCase (evidence_segment): %v", err)
	}
	if len(segments) != 1 || segments[0].ID != segment.ID {
		t.Fatalf("segments = %v, want exactly [%s]", segments, segment.ID)
	}

	caseOnly, err := svc.ListByCase(ctx, tenantID, c.ID, annotations.AnchorFilter{Type: annotations.AnchorCase})
	if err != nil {
		t.Fatalf("ListByCase (case): %v", err)
	}
	if len(caseOnly) != 1 || caseOnly[0].ID != caseLevel.ID {
		t.Fatalf("caseOnly = %v, want exactly [%s]", caseOnly, caseLevel.ID)
	}

	found := false
	for _, a := range treeNodes {
		if a.ID == nodeB.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected node B's annotation %s to appear in the tree_node filter result", nodeB.ID)
	}
}

func TestAnnotation_Validate_AnchorRules(t *testing.T) {
	base := func() *annotations.Annotation {
		return &annotations.Annotation{
			CaseID:   uuid.New(),
			TenantID: uuid.New(),
			AuthorID: uuid.New(),
			Body:     "some text",
		}
	}

	t.Run("case anchor with anchor id is rejected", func(t *testing.T) {
		a := base()
		a.AnchorType = annotations.AnchorCase
		a.AnchorID = "should-be-empty"
		if err := a.Validate(); err == nil {
			t.Fatal("expected an error for case anchor with non-empty AnchorID")
		}
	})

	t.Run("tree node anchor without anchor id is rejected", func(t *testing.T) {
		a := base()
		a.AnchorType = annotations.AnchorTreeNode
		if err := a.Validate(); err == nil {
			t.Fatal("expected an error for tree_node anchor with empty AnchorID")
		}
	})

	t.Run("invalid anchor type is rejected", func(t *testing.T) {
		a := base()
		a.AnchorType = annotations.AnchorType("bogus")
		if err := a.Validate(); err == nil {
			t.Fatal("expected an error for an invalid AnchorType")
		}
	})

	t.Run("valid evidence segment anchor passes", func(t *testing.T) {
		a := base()
		a.AnchorType = annotations.AnchorEvidenceSegment
		a.AnchorID = "segment-42"
		if err := a.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
