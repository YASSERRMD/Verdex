package citation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/graph"
)

func TestDetectBrokenDeleted(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	// Node "rule-1" was previously created and later deleted (simulated
	// by DeleteTree), but known still records it as having existed.
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text"))
	if err := store.DeleteTree(ctx, "case-1"); err != nil {
		t.Fatalf("DeleteTree() error = %v", err)
	}

	known := citation.KnownNodeIDs{"rule-1": {}}
	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1"}

	result, err := citation.DetectBroken(ctx, store, known, unit)
	if err != nil {
		t.Fatalf("DetectBroken() error = %v", err)
	}
	if result.Reason != citation.BrokenReasonDeleted {
		t.Errorf("Reason = %q, want %q", result.Reason, citation.BrokenReasonDeleted)
	}
	if !result.Broken() {
		t.Error("Broken() = false, want true")
	}
}

func TestDetectBrokenNeverExistedIsNotBroken(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()

	unit := citation.CitedUnit{NodeID: "ghost", CaseID: "case-1"}
	result, err := citation.DetectBroken(ctx, store, nil, unit)
	if err != nil {
		t.Fatalf("DetectBroken() error = %v", err)
	}
	if result.Reason != citation.BrokenReasonNone {
		t.Errorf("Reason = %q, want %q (never existed is hallucinated, not broken)", result.Reason, citation.BrokenReasonNone)
	}
	if result.Broken() {
		t.Error("Broken() = true, want false")
	}
}

func TestDetectBrokenStale(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "updated text"))

	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1", Text: "original text"}
	result, err := citation.DetectBroken(ctx, store, nil, unit)
	if err != nil {
		t.Fatalf("DetectBroken() error = %v", err)
	}
	if result.Reason != citation.BrokenReasonStale {
		t.Errorf("Reason = %q, want %q", result.Reason, citation.BrokenReasonStale)
	}
}

func TestDetectBrokenUnchangedTextIsNotBroken(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "same text"))

	unit := citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1", Text: "same text"}
	result, err := citation.DetectBroken(ctx, store, nil, unit)
	if err != nil {
		t.Fatalf("DetectBroken() error = %v", err)
	}
	if result.Reason != citation.BrokenReasonNone {
		t.Errorf("Reason = %q, want %q", result.Reason, citation.BrokenReasonNone)
	}
}

func TestDetectBrokenNilStore(t *testing.T) {
	_, err := citation.DetectBroken(context.Background(), nil, nil, citation.CitedUnit{NodeID: "n", CaseID: "c"})
	if !errors.Is(err, citation.ErrNilGraphStore) {
		t.Errorf("DetectBroken(nil store) error = %v, want ErrNilGraphStore", err)
	}
}

func TestDetectBrokenAll(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text"))

	units := []citation.CitedUnit{
		{NodeID: "rule-1", CaseID: "case-1", Text: "text"},
		{NodeID: "rule-1", CaseID: "case-1", Text: "stale text"},
	}
	results, err := citation.DetectBrokenAll(ctx, store, nil, units)
	if err != nil {
		t.Fatalf("DetectBrokenAll() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Reason != citation.BrokenReasonNone {
		t.Errorf("results[0].Reason = %q, want none", results[0].Reason)
	}
	if results[1].Reason != citation.BrokenReasonStale {
		t.Errorf("results[1].Reason = %q, want stale", results[1].Reason)
	}
}

func TestFindingsFromBroken(t *testing.T) {
	cases := []struct {
		name     string
		result   citation.BrokenResult
		wantLen  int
		wantSev  citation.Severity
		wantCode string
	}{
		{
			name:    "none produces no finding",
			result:  citation.BrokenResult{Reason: citation.BrokenReasonNone},
			wantLen: 0,
		},
		{
			name:     "deleted is critical",
			result:   citation.BrokenResult{Reason: citation.BrokenReasonDeleted},
			wantLen:  1,
			wantSev:  citation.SeverityCritical,
			wantCode: citation.CodeBrokenDeleted,
		},
		{
			name:     "stale is warning",
			result:   citation.BrokenResult{Reason: citation.BrokenReasonStale},
			wantLen:  1,
			wantSev:  citation.SeverityWarning,
			wantCode: citation.CodeBrokenStale,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := citation.FindingsFromBroken(tc.result)
			if len(findings) != tc.wantLen {
				t.Fatalf("len(findings) = %d, want %d", len(findings), tc.wantLen)
			}
			if tc.wantLen == 0 {
				return
			}
			if findings[0].Severity != tc.wantSev {
				t.Errorf("Severity = %q, want %q", findings[0].Severity, tc.wantSev)
			}
			if findings[0].Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", findings[0].Code, tc.wantCode)
			}
		})
	}
}

func TestKnownNodeIDsContains(t *testing.T) {
	var nilSet citation.KnownNodeIDs
	if nilSet.Contains("x") {
		t.Error("nil KnownNodeIDs.Contains() = true, want false")
	}

	known := citation.KnownNodeIDs{"a": {}}
	if !known.Contains("a") {
		t.Error("Contains(a) = false, want true")
	}
	if known.Contains("b") {
		t.Error("Contains(b) = true, want false")
	}
}
