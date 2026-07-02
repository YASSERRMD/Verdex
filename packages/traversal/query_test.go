package traversal_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

func TestQuery_BuilderIsImmutable(t *testing.T) {
	base := traversal.NewQuery("case-1", "issue-1")
	withGoverns := base.ViaGoverningRule()

	if len(base.Hops) != 0 {
		t.Fatalf("expected base Query to remain unmodified, got %d hops", len(base.Hops))
	}
	if len(withGoverns.Hops) != 1 {
		t.Fatalf("expected withGoverns to have 1 hop, got %d", len(withGoverns.Hops))
	}

	withPrecedent := withGoverns.ViaControllingPrecedent()
	if len(withGoverns.Hops) != 1 {
		t.Fatalf("expected withGoverns to remain unmodified after deriving withPrecedent, got %d hops", len(withGoverns.Hops))
	}
	if len(withPrecedent.Hops) != 2 {
		t.Fatalf("expected withPrecedent to have 2 hops, got %d", len(withPrecedent.Hops))
	}
}

func TestQuery_FullChainBuilder(t *testing.T) {
	q := traversal.NewQuery("case-1", "issue-1").
		ViaGoverningRule().
		ViaControllingPrecedent().
		ViaDistinguishingFacts().
		WithMaxDepth(3)

	if len(q.Hops) != 3 {
		t.Fatalf("expected 3 hops, got %d: %+v", len(q.Hops), q.Hops)
	}
	wantKinds := []traversal.HopKind{
		traversal.HopKindGoverningRule,
		traversal.HopKindControllingPrecedent,
		traversal.HopKindDistinguishingFacts,
	}
	for i, want := range wantKinds {
		if q.Hops[i].Kind != want {
			t.Errorf("hop %d: expected kind %q, got %q", i, want, q.Hops[i].Kind)
		}
	}
	if q.MaxDepth != 3 {
		t.Errorf("expected MaxDepth 3, got %d", q.MaxDepth)
	}
}

func TestQuery_ViaCustomHop(t *testing.T) {
	q := traversal.NewQuery("case-1", "app-1").Via(irac.EdgeSupports, traversal.Reverse, irac.NodeFact)

	if len(q.Hops) != 1 {
		t.Fatalf("expected 1 hop, got %d", len(q.Hops))
	}
	hop := q.Hops[0]
	if hop.Kind != traversal.HopKindCustom {
		t.Errorf("expected HopKindCustom, got %q", hop.Kind)
	}
	if hop.EdgeType != irac.EdgeSupports {
		t.Errorf("expected EdgeSupports, got %q", hop.EdgeType)
	}
	if hop.Direction != traversal.Reverse {
		t.Errorf("expected Reverse direction, got %v", hop.Direction)
	}
	if hop.NodeTypeFilter != irac.NodeFact {
		t.Errorf("expected NodeFact filter, got %q", hop.NodeTypeFilter)
	}
}

func TestQuery_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   traversal.Query
		wantErr error
	}{
		{
			name:    "empty case id",
			query:   traversal.NewQuery("", "issue-1").ViaGoverningRule(),
			wantErr: traversal.ErrEmptyCaseID,
		},
		{
			name:    "empty start node id",
			query:   traversal.NewQuery("case-1", "").ViaGoverningRule(),
			wantErr: traversal.ErrEmptyStartNodeID,
		},
		{
			name:    "no hops",
			query:   traversal.NewQuery("case-1", "issue-1"),
			wantErr: traversal.ErrNoHops,
		},
		{
			name:    "negative max depth",
			query:   traversal.NewQuery("case-1", "issue-1").ViaGoverningRule().WithMaxDepth(-1),
			wantErr: traversal.ErrInvalidMaxDepth,
		},
	}

	store := newSeededStore(t)
	walker, err := traversal.NewWalker(store)
	if err != nil {
		t.Fatalf("NewWalker: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := walker.Execute(ctxBackground(), tt.query)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
