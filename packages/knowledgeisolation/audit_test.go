package knowledgeisolation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

func TestNoOpAlertSink_DoesNothing(t *testing.T) {
	t.Parallel()

	// Must not panic.
	knowledgeisolation.NoOpAlertSink{}.Notify(knowledgeisolation.AccessAttempt{})
}

func TestFuncAlertSink_NilSafe(t *testing.T) {
	t.Parallel()

	var f knowledgeisolation.FuncAlertSink
	// Must not panic when the underlying func is nil.
	f.Notify(knowledgeisolation.AccessAttempt{})
}

func TestFuncAlertSink_Invokes(t *testing.T) {
	t.Parallel()

	var got knowledgeisolation.AccessAttempt
	called := false
	f := knowledgeisolation.FuncAlertSink(func(a knowledgeisolation.AccessAttempt) {
		called = true
		got = a
	})

	attempt := knowledgeisolation.AccessAttempt{Kind: knowledgeisolation.ViolationGetNode, NodeID: "n1"}
	f.Notify(attempt)

	if !called {
		t.Fatalf("expected FuncAlertSink to invoke the underlying function")
	}
	if got.NodeID != "n1" {
		t.Fatalf("got %+v", got)
	}
}

func TestMultiAlertSink_FansOutToEverySink(t *testing.T) {
	t.Parallel()

	var calls int
	sink := knowledgeisolation.FuncAlertSink(func(knowledgeisolation.AccessAttempt) { calls++ })

	multi := knowledgeisolation.MultiAlertSink{Sinks: []knowledgeisolation.AlertSink{sink, sink, knowledgeisolation.NoOpAlertSink{}}}
	multi.Notify(knowledgeisolation.AccessAttempt{})

	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestMultiAlertSink_TolerantOfNilEntries(t *testing.T) {
	t.Parallel()

	multi := knowledgeisolation.MultiAlertSink{Sinks: []knowledgeisolation.AlertSink{nil}}
	// Must not panic.
	multi.Notify(knowledgeisolation.AccessAttempt{})
}

func TestViolationKind_Values(t *testing.T) {
	t.Parallel()

	// Basic sanity: every constant is a distinct, non-empty string, so
	// audit consumers can rely on Kind for filtering/grouping.
	kinds := []knowledgeisolation.ViolationKind{
		knowledgeisolation.ViolationGetNode,
		knowledgeisolation.ViolationCreateEdge,
		knowledgeisolation.ViolationTraverse,
		knowledgeisolation.ViolationVectorQuery,
		knowledgeisolation.ViolationVectorUpsert,
		knowledgeisolation.ViolationCrossCaseAnalysis,
		knowledgeisolation.ViolationDeleteTree,
		knowledgeisolation.ViolationVectorDeleteCase,
	}
	seen := map[knowledgeisolation.ViolationKind]bool{}
	for _, k := range kinds {
		if k == "" {
			t.Fatalf("expected non-empty ViolationKind")
		}
		if seen[k] {
			t.Fatalf("duplicate ViolationKind value %q", k)
		}
		seen[k] = true
	}
}
