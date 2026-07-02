package citation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestResolveWithLookupResolver(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "no person shall..."))

	records := map[string]citation.ResolvedCitation{
		"rule-1": {Text: "Act 12, s.5(a)", Origin: citation.OriginStatute},
	}
	resolver := citation.LookupResolver(records, nil)

	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1"}
	resolved, err := citation.Resolve(ctx, store, resolver, unit)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Citation != "Act 12, s.5(a)" {
		t.Errorf("Citation = %q, want %q", resolved.Citation, "Act 12, s.5(a)")
	}
	if resolved.Origin != citation.OriginStatute {
		t.Errorf("Origin = %q, want %q", resolved.Origin, citation.OriginStatute)
	}
	if resolved.Text != "no person shall..." {
		t.Errorf("Text = %q, want node text", resolved.Text)
	}
	if resolved.NodeType != irac.NodeRule {
		t.Errorf("NodeType = %q, want %q", resolved.NodeType, irac.NodeRule)
	}
}

func TestResolveFallsBackToFallback(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-2", "case-1", "rule text"))

	resolver := citation.LookupResolver(nil, citation.RuleTextResolver(citation.OriginPrecedent))

	unit := citation.CitedUnit{NodeID: "rule-2", CaseID: "case-1"}
	resolved, err := citation.Resolve(ctx, store, resolver, unit)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.Citation != "rule text" {
		t.Errorf("Citation = %q, want fallback heuristic text", resolved.Citation)
	}
	if resolved.Origin != citation.OriginPrecedent {
		t.Errorf("Origin = %q, want %q", resolved.Origin, citation.OriginPrecedent)
	}
}

func TestRuleTextResolverIgnoresNonRuleNodes(t *testing.T) {
	resolver := citation.RuleTextResolver(citation.OriginStatute)
	rc, err := resolver(context.Background(), factNode("fact-1", "case-1", "some fact"))
	if err != nil {
		t.Fatalf("resolver() error = %v", err)
	}
	if rc.Certainty != citation.CertaintyNone {
		t.Errorf("Certainty = %q, want %q for a fact node", rc.Certainty, citation.CertaintyNone)
	}
}

func TestNoResolver(t *testing.T) {
	rc, err := citation.NoResolver(context.Background(), ruleNode("rule-1", "case-1", "text"))
	if err != nil {
		t.Fatalf("NoResolver() error = %v", err)
	}
	if rc.Certainty != citation.CertaintyNone {
		t.Errorf("Certainty = %q, want %q", rc.Certainty, citation.CertaintyNone)
	}
}

func TestResolveNilDependencies(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1"}

	if _, err := citation.Resolve(ctx, nil, citation.NoResolver, unit); !errors.Is(err, citation.ErrNilGraphStore) {
		t.Errorf("Resolve(nil store) error = %v, want ErrNilGraphStore", err)
	}
	if _, err := citation.Resolve(ctx, store, nil, unit); !errors.Is(err, citation.ErrNilResolver) {
		t.Errorf("Resolve(nil resolver) error = %v, want ErrNilResolver", err)
	}
	if _, err := citation.Resolve(ctx, store, citation.NoResolver, citation.CitedUnit{CaseID: "case-1"}); !errors.Is(err, citation.ErrEmptyNodeID) {
		t.Errorf("Resolve(empty node id) error = %v, want ErrEmptyNodeID", err)
	}
}

func TestResolvePropagatesNodeNotFound(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	unit := citation.CitedUnit{NodeID: "missing", CaseID: "case-1"}

	_, err := citation.Resolve(ctx, store, citation.NoResolver, unit)
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Errorf("Resolve() error = %v, want graph.ErrNodeNotFound", err)
	}
}

func TestResolveAll(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text one"))
	mustCreateNode(t, store, ruleNode("rule-2", "case-1", "text two"))

	units := []citation.CitedUnit{
		{NodeID: "rule-1", CaseID: "case-1"},
		{NodeID: "rule-2", CaseID: "case-1"},
	}
	resolver := citation.RuleTextResolver(citation.OriginStatute)

	resolved, err := citation.ResolveAll(ctx, store, resolver, units)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("len(resolved) = %d, want 2", len(resolved))
	}
	if resolved[0].Citation != "text one" || resolved[1].Citation != "text two" {
		t.Errorf("resolved citations = %q, %q", resolved[0].Citation, resolved[1].Citation)
	}
}

func TestResolveAllStopsAtFirstError(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text one"))

	units := []citation.CitedUnit{
		{NodeID: "rule-1", CaseID: "case-1"},
		{NodeID: "missing", CaseID: "case-1"},
	}

	resolved, err := citation.ResolveAll(ctx, store, citation.NoResolver, units)
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("ResolveAll() error = %v, want graph.ErrNodeNotFound", err)
	}
	if len(resolved) != 1 {
		t.Errorf("len(resolved) = %d, want 1 (partial progress)", len(resolved))
	}
}

func TestCertaintyIsValid(t *testing.T) {
	cases := []struct {
		c    citation.Certainty
		want bool
	}{
		{citation.CertaintyExact, true},
		{citation.CertaintyHeuristic, true},
		{citation.CertaintyNone, true},
		{citation.Certainty("bogus"), false},
	}
	for _, tc := range cases {
		if got := tc.c.IsValid(); got != tc.want {
			t.Errorf("Certainty(%q).IsValid() = %v, want %v", tc.c, got, tc.want)
		}
	}
}
