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

func TestNewCaseScopedStore_Validation(t *testing.T) {
	t.Parallel()

	if _, err := knowledgeisolation.NewCaseScopedStore(nil, "case-a", nil); !errors.Is(err, knowledgeisolation.ErrNilStore) {
		t.Fatalf("expected ErrNilStore, got %v", err)
	}

	inner := graph.NewInMemoryGraphStore()
	if _, err := knowledgeisolation.NewCaseScopedStore(inner, "", nil); !errors.Is(err, knowledgeisolation.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestCaseScopedStore_CreateNode_RejectsForeignCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	foreignFact := irac.Node{ID: "fact-x", Type: irac.NodeFact, CaseID: "case-b"}
	err = store.CreateNode(context.Background(), foreignFact)
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}
}

func TestCaseScopedStore_CreateNode_AllowsOwnCaseAndSharedLaw(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	ownFact := irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a"}
	if err := store.CreateNode(context.Background(), ownFact); err != nil {
		t.Fatalf("expected own-case CreateNode to succeed, got %v", err)
	}

	// A shared-law node attributed to a *different* case must still be
	// creatable through this store, since RuleNodes are never
	// case-exclusive.
	sharedRule := irac.Node{ID: "rule-1", Type: irac.NodeRule, CaseID: "case-b"}
	if err := store.CreateNode(context.Background(), sharedRule); err != nil {
		t.Fatalf("expected shared-law CreateNode to succeed regardless of CaseID, got %v", err)
	}
}

func TestCaseScopedStore_GetNode_NotFoundPropagates(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	_, err = store.GetNode(context.Background(), "does-not-exist")
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound to propagate, got %v", err)
	}
}

func TestCaseScopedStore_CreateEdge_PropagatesInnerNotFound(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	err = store.CreateEdge(context.Background(), irac.Edge{FromID: "missing-1", ToID: "missing-2", Type: irac.EdgeGoverns})
	if !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestCaseScopedStore_DeleteTree_RejectsForeignCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	err = store.DeleteTree(context.Background(), "case-b")
	if !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}
}

func TestCaseScopedStore_DeleteTree_OwnCase(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	ctx := context.Background()
	if err := store.CreateNode(ctx, irac.Node{ID: "fact-1", Type: irac.NodeFact, CaseID: "case-a", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := store.DeleteTree(ctx, "case-a"); err != nil {
		t.Fatalf("expected own-case DeleteTree to succeed, got %v", err)
	}
}

func TestCaseScopedStore_AuditRecordsCapturedByCustomSink(t *testing.T) {
	t.Parallel()

	var captured []knowledgeisolation.AccessAttempt
	sink := knowledgeisolation.FuncAlertSink(func(a knowledgeisolation.AccessAttempt) {
		captured = append(captured, a)
	})

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", sink)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}

	foreignFact := irac.Node{ID: "fact-x", Type: irac.NodeFact, CaseID: "case-b"}
	if err := store.CreateNode(context.Background(), foreignFact); !errors.Is(err, knowledgeisolation.ErrCrossCaseAccess) {
		t.Fatalf("expected ErrCrossCaseAccess, got %v", err)
	}

	if len(captured) != 1 {
		t.Fatalf("expected 1 captured attempt, got %d", len(captured))
	}
	if captured[0].AttemptedCase != "case-b" {
		t.Fatalf("expected AttemptedCase case-b, got %q", captured[0].AttemptedCase)
	}
	if captured[0].OccurredAt.IsZero() {
		t.Fatalf("expected OccurredAt to be set")
	}

	// The store's own AccessAttempts() must reflect the same event.
	stored := store.AccessAttempts()
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored attempt, got %d", len(stored))
	}
}

func TestCaseScopedStore_CaseID(t *testing.T) {
	t.Parallel()

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, "case-a", nil)
	if err != nil {
		t.Fatalf("NewCaseScopedStore: %v", err)
	}
	if got := store.CaseID(); got != "case-a" {
		t.Fatalf("CaseID() = %q, want case-a", got)
	}
}
