package knowledgeisolation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

func TestNewCrossCaseReader_Validation(t *testing.T) {
	t.Parallel()

	if _, err := knowledgeisolation.NewCrossCaseReader(nil, nil); !errors.Is(err, knowledgeisolation.ErrNilStore) {
		t.Fatalf("expected ErrNilStore, got %v", err)
	}
}

func TestCrossCaseReader_GetNode_RejectsMissingAuthorization(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	_, err = reader.GetNodeAcrossCases(context.Background(), "node-1", knowledgeisolation.CrossCaseAuthorization{})
	if !errors.Is(err, knowledgeisolation.ErrMissingAuthorization) {
		t.Fatalf("expected ErrMissingAuthorization, got %v", err)
	}
}

func TestCrossCaseReader_GetNode_RejectsExpiredAuthorization(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{
		Cases:     []string{"case-a"},
		Reason:    "test",
		ExpiresAt: time.Now().Add(-time.Hour),
	}
	_, err = reader.GetNodeAcrossCases(context.Background(), "node-1", auth)
	if !errors.Is(err, knowledgeisolation.ErrAuthorizationExpired) {
		t.Fatalf("expected ErrAuthorizationExpired, got %v", err)
	}
}

func TestCrossCaseReader_GetNode_RejectsUncoveredCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	if err := inner.CreateNode(ctx, irac.Node{ID: "fact-a", Type: irac.NodeFact, CaseID: "case-a"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{Cases: []string{"case-b", "case-c"}, Reason: "dashboard"}
	_, err = reader.GetNodeAcrossCases(ctx, "fact-a", auth)
	if !errors.Is(err, knowledgeisolation.ErrCaseNotAuthorized) {
		t.Fatalf("expected ErrCaseNotAuthorized, got %v", err)
	}
}

func TestCrossCaseReader_GetNode_AllowsCoveredCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	if err := inner.CreateNode(ctx, irac.Node{ID: "fact-a", Type: irac.NodeFact, CaseID: "case-a"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := inner.CreateNode(ctx, irac.Node{ID: "fact-b", Type: irac.NodeFact, CaseID: "case-b"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	var recorded []knowledgeisolation.AccessAttempt
	sink := knowledgeisolation.FuncAlertSink(func(a knowledgeisolation.AccessAttempt) { recorded = append(recorded, a) })
	reader, err := knowledgeisolation.NewCrossCaseReader(inner, sink)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{
		Cases:     []string{"case-a", "case-b"},
		Reason:    "cross-case-analytics-dashboard",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	nodeA, err := reader.GetNodeAcrossCases(ctx, "fact-a", auth)
	if err != nil {
		t.Fatalf("expected authorized cross-case read to succeed, got %v", err)
	}
	if nodeA.ID != "fact-a" {
		t.Fatalf("unexpected node: %+v", nodeA)
	}

	nodeB, err := reader.GetNodeAcrossCases(ctx, "fact-b", auth)
	if err != nil {
		t.Fatalf("expected authorized cross-case read to succeed, got %v", err)
	}
	if nodeB.ID != "fact-b" {
		t.Fatalf("unexpected node: %+v", nodeB)
	}

	if len(recorded) != 2 {
		t.Fatalf("expected 2 audited authorized reads, got %d: %+v", len(recorded), recorded)
	}
	for _, a := range recorded {
		if a.Kind != knowledgeisolation.ViolationCrossCaseAnalysis {
			t.Fatalf("expected ViolationCrossCaseAnalysis, got %v", a.Kind)
		}
	}
}

func TestCrossCaseReader_GetNode_SharedLawAlwaysReadable(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	if err := inner.CreateNode(ctx, irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-a"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	// Authorization covers neither case-a nor any other case, yet the
	// shared-law node must still be readable.
	auth := knowledgeisolation.CrossCaseAuthorization{Cases: []string{"case-z"}, Reason: "dashboard"}
	node, err := reader.GetNodeAcrossCases(ctx, "rule-1", auth)
	if err != nil {
		t.Fatalf("expected shared-law read to succeed regardless of case coverage, got %v", err)
	}
	if node.ID != "rule-1" {
		t.Fatalf("unexpected node: %+v", node)
	}
}

func TestCrossCaseReader_Traverse_RejectsUncoveredCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{Cases: []string{"case-a"}, Reason: "dashboard"}
	_, err = reader.TraverseAcrossCases(context.Background(), graph.TraversalQuery{CaseID: "case-b"}, auth)
	if !errors.Is(err, knowledgeisolation.ErrCaseNotAuthorized) {
		t.Fatalf("expected ErrCaseNotAuthorized, got %v", err)
	}
}

func TestCrossCaseReader_Traverse_AllowsCoveredCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	ctx := context.Background()
	if err := inner.CreateNode(ctx, irac.Node{ID: "fact-a", Type: irac.NodeFact, CaseID: "case-a"}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{Cases: []string{"case-a"}, Reason: "dashboard"}
	nodes, err := reader.TraverseAcrossCases(ctx, graph.TraversalQuery{CaseID: "case-a"}, auth)
	if err != nil {
		t.Fatalf("TraverseAcrossCases: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "fact-a" {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}

	attempts := reader.AccessAttempts()
	if len(attempts) != 1 {
		t.Fatalf("expected 1 audited attempt, got %d", len(attempts))
	}
}

func TestCrossCaseReader_Traverse_EmptyCaseIDAllowedAcrossAllAuthorizedCases(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	reader, err := knowledgeisolation.NewCrossCaseReader(inner, nil)
	if err != nil {
		t.Fatalf("NewCrossCaseReader: %v", err)
	}

	auth := knowledgeisolation.CrossCaseAuthorization{Cases: []string{"case-a"}, Reason: "dashboard"}
	// An empty query.CaseID is not itself an uncovered-case attempt (it
	// is validated as "no explicit case requested"); the inner store
	// will reject it independently since graph.TraversalQuery requires
	// a CaseID.
	_, err = reader.TraverseAcrossCases(context.Background(), graph.TraversalQuery{}, auth)
	if !errors.Is(err, graph.ErrEmptyCaseID) {
		t.Fatalf("expected graph.ErrEmptyCaseID, got %v", err)
	}
}
