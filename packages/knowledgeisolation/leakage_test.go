package knowledgeisolation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// seedTwoCaseTree builds, in inner, two independent IRAC trees for
// case-a and case-b, each Rule -> Issue -> ... -> Conclusion, plus one
// shared-law RuleNode nominally attributed to case-a but intended to be
// readable from both. Returns the node IDs of interest for assertions.
func seedTwoCaseTree(t *testing.T, inner graph.GraphStore) (caseANodes, caseBNodes []string, sharedRuleID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	mustCreate := func(n irac.Node) {
		t.Helper()
		if err := inner.CreateNode(ctx, n); err != nil {
			t.Fatalf("seed CreateNode(%s): %v", n.ID, err)
		}
	}

	// Shared-law rule, attributed to case-a for provenance but readable
	// from any case. graph.InMemoryGraphStore.CreateEdge itself refuses
	// to link nodes across two different CaseID values (there is no
	// legal cross-case edge in the raw IRAC schema), so a shared-law
	// node cannot be wired into more than one case's edge graph at the
	// storage layer; readability across cases is purely a property this
	// package's isolation guard grants via IsSharedLawNode on direct
	// GetNode/Traverse-result reads, independent of edges.
	sharedRuleID = "rule-shared"
	mustCreate(irac.Node{ID: sharedRuleID, Type: irac.NodeRule, CaseID: "case-a", Text: "shared statute", CreatedAt: now})

	// Case A's private tree.
	mustCreate(irac.Node{ID: "issue-a", Type: irac.NodeIssue, CaseID: "case-a", Text: "issue a", CreatedAt: now})
	mustCreate(irac.Node{ID: "fact-a", Type: irac.NodeFact, CaseID: "case-a", Text: "secret fact a", CreatedAt: now})
	mustCreate(irac.Node{ID: "app-a", Type: irac.NodeApplication, CaseID: "case-a", Text: "application a", CreatedAt: now})
	mustCreate(irac.Node{ID: "conclusion-a", Type: irac.NodeConclusion, CaseID: "case-a", Text: "conclusion a", CreatedAt: now})
	if err := inner.CreateEdge(ctx, irac.Edge{FromID: sharedRuleID, ToID: "issue-a", Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("seed CreateEdge governs a: %v", err)
	}
	if err := inner.CreateEdge(ctx, irac.Edge{FromID: "fact-a", ToID: "app-a", Type: irac.EdgeSupports}); err != nil {
		t.Fatalf("seed CreateEdge supports a: %v", err)
	}

	// Case B's private tree, governed by its own case-b-attributed rule
	// node (rule-b) rather than sharing an edge with rule-shared, for
	// the reason noted above.
	mustCreate(irac.Node{ID: "rule-b", Type: irac.NodeRule, CaseID: "case-b", Text: "case-b's own rule reference", CreatedAt: now})
	mustCreate(irac.Node{ID: "issue-b", Type: irac.NodeIssue, CaseID: "case-b", Text: "issue b", CreatedAt: now})
	mustCreate(irac.Node{ID: "fact-b", Type: irac.NodeFact, CaseID: "case-b", Text: "secret fact b", CreatedAt: now})
	mustCreate(irac.Node{ID: "app-b", Type: irac.NodeApplication, CaseID: "case-b", Text: "application b", CreatedAt: now})
	mustCreate(irac.Node{ID: "conclusion-b", Type: irac.NodeConclusion, CaseID: "case-b", Text: "conclusion b", CreatedAt: now})
	if err := inner.CreateEdge(ctx, irac.Edge{FromID: "rule-b", ToID: "issue-b", Type: irac.EdgeGoverns}); err != nil {
		t.Fatalf("seed CreateEdge governs b: %v", err)
	}
	if err := inner.CreateEdge(ctx, irac.Edge{FromID: "fact-b", ToID: "app-b", Type: irac.EdgeSupports}); err != nil {
		t.Fatalf("seed CreateEdge supports b: %v", err)
	}

	return []string{"issue-a", "fact-a", "app-a", "conclusion-a"},
		[]string{"issue-b", "fact-b", "app-b", "conclusion-b"},
		sharedRuleID
}

// TestLeakage_GetNode_DirectNodeIDGuess proves that a case-B-scoped
// store cannot retrieve a case-A fact even when handed case-A's exact
// node ID directly (the simplest possible leakage attempt).
func TestLeakage_GetNode_DirectNodeIDGuess(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	_, _, _ = seedTwoCaseTree(t, inner)

	caseBStore, err := knowledgeisolation.NewCaseScopedStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	_, err = caseBStore.GetNode(context.Background(), "fact-a")
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess reading case-a's fact from case-b's store, got %v", err)
	}
}

// TestLeakage_MultiHopTraversal_AdversarialCaseIDRejected proves that a
// case-A-scoped guard refuses to serve a traversal whose query.CaseID
// has been adversarially set to case-b, even though the underlying
// InMemoryGraphStore would happily answer that query on its own (it has
// no notion of "the caller is only authorized for case-a"). This is
// exactly the leakage vector CaseScopedStore exists to close: a caller
// holding a guard scoped to one case constructing a query naming a
// different case.
func TestLeakage_MultiHopTraversal_AdversarialCaseIDRejected(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	_, caseBNodes, _ := seedTwoCaseTree(t, inner)

	// Sanity check: the raw inner store, with no isolation guard, does
	// happily return case-b's facts (and case-b's own rule node) for a
	// case-b query.
	rawNodes, err := inner.Traverse(context.Background(), graph.TraversalQuery{CaseID: "case-b"})
	if err != nil {
		t.Fatalf("sanity Traverse: %v", err)
	}
	const caseBNodeCount = 5 // rule-b, issue-b, fact-b, app-b, conclusion-b
	if len(rawNodes) != caseBNodeCount {
		t.Fatalf("sanity check: expected raw store to return all %d case-b nodes, got %d", caseBNodeCount, len(rawNodes))
	}

	caseAStore, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	// The case-a guard forwards the adversarial case-b query to the
	// inner store (CaseScopedStore does not rewrite query.CaseID the
	// way CaseScopedVectorStore does). Every case-b-owned, case-scoped
	// node (Issue/Fact/Application/Conclusion) must be filtered out;
	// rule-b, being a shared-law RuleNode, is allowed through even
	// though it is nominally attributed to case-b — that is the
	// intended shared-law exemption, not a leak of case-b's private
	// facts.
	nodes, err := caseAStore.Traverse(context.Background(), graph.TraversalQuery{CaseID: "case-b"})
	if err != nil {
		t.Fatalf("Traverse: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "rule-b" {
		t.Fatalf("expected only the shared-law rule-b node to pass through, got %+v", nodes)
	}

	attempts := caseAStore.AccessAttempts()
	if len(attempts) != len(caseBNodes) {
		t.Fatalf("expected %d audited filter attempts (one per case-b-owned fact node), got %d: %+v", len(caseBNodes), len(attempts), attempts)
	}
	for _, a := range attempts {
		if a.Kind != knowledgeisolation.ViolationTraverse {
			t.Fatalf("expected ViolationTraverse, got %v", a.Kind)
		}
		if a.NodeID == "rule-b" {
			t.Fatalf("shared-law node rule-b must not be recorded as a violation")
		}
	}
}

// TestLeakage_SharedLawNode_ReadableFromBothCases proves the isolation
// guard's other half: a genuinely shared RuleNode must remain readable
// from every case, so isolation does not overcorrect into blocking
// legitimate shared statute/precedent lookups.
func TestLeakage_SharedLawNode_ReadableFromBothCases(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	_, _, sharedRuleID := seedTwoCaseTree(t, inner)

	caseAStore, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore case-a: %v", err)
	}
	caseBStore, err := knowledgeisolation.NewCaseScopedStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore case-b: %v", err)
	}

	if _, err := caseAStore.GetNode(context.Background(), sharedRuleID); err != nil {
		t.Fatalf("expected case-a to read shared rule, got %v", err)
	}
	if _, err := caseBStore.GetNode(context.Background(), sharedRuleID); err != nil {
		t.Fatalf("expected case-b to read shared rule, got %v", err)
	}
}

// TestLeakage_CreateEdge_RejectsCrossCaseLink proves a caller cannot
// stitch case-A's facts into case-B's reasoning tree by creating a
// single edge whose endpoints span both cases.
func TestLeakage_CreateEdge_RejectsCrossCaseLink(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	seedTwoCaseTree(t, inner)

	caseBStore, err := knowledgeisolation.NewCaseScopedStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	// Attempt to link case-a's fact into case-b's application node.
	err = caseBStore.CreateEdge(context.Background(), irac.Edge{FromID: "fact-a", ToID: "app-b", Type: irac.EdgeSupports})
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess linking case-a fact into case-b tree, got %v", err)
	}
}

// vectorRecordFor builds a minimal VectorRecord for leakage tests.
func vectorRecordFor(id string, nodeType irac.NodeType, caseID string) vectorindex.VectorRecord {
	return vectorindex.VectorRecord{
		ID:        id,
		NodeType:  nodeType,
		CaseID:    caseID,
		Text:      id,
		Vector:    embedding.EmbeddingVector{1, 0, 0},
		UpdatedAt: time.Now(),
	}
}

// TestLeakage_VectorQuery_CannotRecallForeignCaseFacts proves a case-B
// scoped vector store query cannot recall case-A's vector records, even
// when the caller explicitly (adversarially) sets req.CaseID to case-a.
func TestLeakage_VectorQuery_CannotRecallForeignCaseFacts(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	if err := inner.Upsert(ctx, vectorRecordFor("fact-a", irac.NodeFact, "case-a")); err != nil {
		t.Fatalf("seed upsert fact-a: %v", err)
	}
	if err := inner.Upsert(ctx, vectorRecordFor("fact-b", irac.NodeFact, "case-b")); err != nil {
		t.Fatalf("seed upsert fact-b: %v", err)
	}

	caseBStore, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	results, err := caseBStore.Query(ctx, vectorindex.QueryRequest{
		Vector: embedding.EmbeddingVector{1, 0, 0},
		TopK:   10,
		CaseID: "case-a", // adversarial: try to widen the search to case-a.
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	for _, r := range results {
		if r.Record.CaseID != "case-b" {
			t.Fatalf("Query leaked record from case %q into case-b's results: %+v", r.Record.CaseID, r.Record)
		}
	}

	attempts := caseBStore.AccessAttempts()
	if len(attempts) == 0 {
		t.Fatalf("expected the CaseID override attempt to be audited")
	}
}

// TestLeakage_VectorUpsert_RejectsForeignCaseRecord proves a case-B
// scoped vector store cannot be used to write a case-A record.
func TestLeakage_VectorUpsert_RejectsForeignCaseRecord(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	caseBStore, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	err = caseBStore.Upsert(context.Background(), vectorRecordFor("fact-a", irac.NodeFact, "case-a"))
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess upserting case-a record via case-b store, got %v", err)
	}
}

// TestLeakage_VectorQuery_SharedLawRecordsFlowThroughAuthorizedCase
// confirms shared-law vector records are still recallable from either
// case's own scoped query (proving the vector guard does not
// overcorrect either).
func TestLeakage_VectorQuery_SharedLawRecordsFlowThroughAuthorizedCase(t *testing.T) {
	t.Parallel()

	inner := vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{})
	ctx := context.Background()

	sharedRule := vectorRecordFor("rule-shared", irac.NodeRule, "case-a")
	if err := inner.Upsert(ctx, sharedRule); err != nil {
		t.Fatalf("seed upsert shared rule: %v", err)
	}

	caseBStore, err := knowledgeisolation.NewCaseScopedVectorStore(inner, "case-b", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedVectorStore: %v", err)
	}

	// Upserting the shared-law record via case-b's store must succeed
	// even though it is attributed to case-a.
	if err := caseBStore.Upsert(ctx, sharedRule); err != nil {
		t.Fatalf("expected shared-law record upsert to succeed from case-b's store, got %v", err)
	}
}
