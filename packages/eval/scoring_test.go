package eval_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/eval"
)

func TestExactMatchScorer_Match(t *testing.T) {
	c := eval.ExactMatchScorer(1.0)
	got := c.Fn("hello world", "hello world")
	if got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestExactMatchScorer_CaseInsensitiveMatch(t *testing.T) {
	c := eval.ExactMatchScorer(1.0)
	got := c.Fn("Hello World", "hello world")
	if got != 1.0 {
		t.Errorf("expected 1.0 for case-insensitive match, got %f", got)
	}
}

func TestExactMatchScorer_NoMatch(t *testing.T) {
	c := eval.ExactMatchScorer(1.0)
	got := c.Fn("something else", "expected text")
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestContainsKeywordsScorer_AllPresent(t *testing.T) {
	c := eval.ContainsKeywordsScorer([]string{"duty", "breach", "causation"}, 1.0)
	got := c.Fn("The duty was breached, causation was established.", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 when all keywords present, got %f", got)
	}
}

func TestContainsKeywordsScorer_PartialMatch(t *testing.T) {
	c := eval.ContainsKeywordsScorer([]string{"duty", "breach", "causation"}, 1.0)
	got := c.Fn("duty was breached", "")
	want := 2.0 / 3.0
	if got < want-0.001 || got > want+0.001 {
		t.Errorf("expected ~%.4f, got %f", want, got)
	}
}

func TestContainsKeywordsScorer_NonePresent(t *testing.T) {
	c := eval.ContainsKeywordsScorer([]string{"duty", "breach"}, 1.0)
	got := c.Fn("the model said something irrelevant", "")
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestContainsKeywordsScorer_EmptyKeywords(t *testing.T) {
	c := eval.ContainsKeywordsScorer([]string{}, 1.0)
	got := c.Fn("any output", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for empty keywords, got %f", got)
	}
}

func TestContainsKeywordsScorer_CaseInsensitive(t *testing.T) {
	c := eval.ContainsKeywordsScorer([]string{"DUTY"}, 1.0)
	got := c.Fn("duty was established", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for case-insensitive keyword match, got %f", got)
	}
}

func TestCitationPresenceScorer_AllPresent(t *testing.T) {
	c := eval.CitationPresenceScorer([]string{"jones v. smith", "42 u.s.c. § 1983"}, 1.0)
	got := c.Fn("See Jones v. Smith and 42 U.S.C. § 1983.", "")
	if got != 1.0 {
		t.Errorf("expected 1.0, got %f", got)
	}
}

func TestCitationPresenceScorer_NonePresent(t *testing.T) {
	c := eval.CitationPresenceScorer([]string{"jones v. smith"}, 1.0)
	got := c.Fn("The analysis does not mention any cases.", "")
	if got != 0.0 {
		t.Errorf("expected 0.0, got %f", got)
	}
}

func TestCitationPresenceScorer_EmptyCitations(t *testing.T) {
	c := eval.CitationPresenceScorer([]string{}, 1.0)
	got := c.Fn("any output", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for empty citations, got %f", got)
	}
}

func TestNonBindingComplianceScorer_Compliant(t *testing.T) {
	c := eval.NonBindingComplianceScorer(1.0)
	got := c.Fn("The analysis suggests the plaintiff may have a viable claim.", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for compliant output, got %f", got)
	}
}

func TestNonBindingComplianceScorer_VerdictDetected(t *testing.T) {
	c := eval.NonBindingComplianceScorer(1.0)
	got := c.Fn("verdict: the defendant is liable.", "")
	if got != 0.0 {
		t.Errorf("expected 0.0 when 'verdict' detected, got %f", got)
	}
}

func TestNonBindingComplianceScorer_OrderDetected(t *testing.T) {
	c := eval.NonBindingComplianceScorer(1.0)
	got := c.Fn("It is ordered that damages be paid.", "")
	if got != 0.0 {
		t.Errorf("expected 0.0 when 'it is ordered' detected, got %f", got)
	}
}

func TestNonBindingComplianceScorer_SoOrderedDetected(t *testing.T) {
	c := eval.NonBindingComplianceScorer(1.0)
	got := c.Fn("So ordered.", "")
	if got != 0.0 {
		t.Errorf("expected 0.0 when 'so ordered' detected, got %f", got)
	}
}

func TestJurisdictionAccuracyScorer_Match(t *testing.T) {
	c := eval.JurisdictionAccuracyScorer("New York", 1.0)
	got := c.Fn("Under New York law, the claim is timely.", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 when jurisdiction present, got %f", got)
	}
}

func TestJurisdictionAccuracyScorer_NoMatch(t *testing.T) {
	c := eval.JurisdictionAccuracyScorer("New York", 1.0)
	got := c.Fn("Under California law, this would be different.", "")
	if got != 0.0 {
		t.Errorf("expected 0.0 when jurisdiction absent, got %f", got)
	}
}

func TestJurisdictionAccuracyScorer_CaseInsensitive(t *testing.T) {
	c := eval.JurisdictionAccuracyScorer("new york", 1.0)
	got := c.Fn("Under NEW YORK law...", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for case-insensitive jurisdiction match, got %f", got)
	}
}

func TestJurisdictionAccuracyScorer_EmptyExpected(t *testing.T) {
	c := eval.JurisdictionAccuracyScorer("", 1.0)
	got := c.Fn("any output", "")
	if got != 1.0 {
		t.Errorf("expected 1.0 for empty expected jurisdiction, got %f", got)
	}
}

// --- SideBySide comparison tests ---

func TestSideBySide_WinnerIsHigherScore(t *testing.T) {
	a := eval.EvalResult{TaskID: "t1", ProviderID: "alpha", Score: 0.9}
	b := eval.EvalResult{TaskID: "t1", ProviderID: "beta", Score: 0.6}
	cmp := eval.SideBySide(a, b)
	if cmp.WinnerID != "alpha" {
		t.Errorf("expected winner 'alpha', got %q", cmp.WinnerID)
	}
	diff := cmp.ScoreDiff
	if diff < 0.299 || diff > 0.301 {
		t.Errorf("expected ScoreDiff ~0.3, got %f", diff)
	}
}

func TestSideBySide_TieHasEmptyWinner(t *testing.T) {
	a := eval.EvalResult{TaskID: "t1", ProviderID: "alpha", Score: 0.7}
	b := eval.EvalResult{TaskID: "t1", ProviderID: "beta", Score: 0.7}
	cmp := eval.SideBySide(a, b)
	if cmp.WinnerID != "" {
		t.Errorf("expected empty winner for tie, got %q", cmp.WinnerID)
	}
}

func TestSideBySide_MismatchedTaskIDs(t *testing.T) {
	a := eval.EvalResult{TaskID: "t1", ProviderID: "alpha", Score: 0.9}
	b := eval.EvalResult{TaskID: "t2", ProviderID: "beta", Score: 0.4}
	cmp := eval.SideBySide(a, b)
	if cmp.WinnerID != "mismatch" {
		t.Errorf("expected 'mismatch', got %q", cmp.WinnerID)
	}
}

// --- InMemoryGoldenStore tests ---

func TestInMemoryGoldenStore_SaveAndLoad(t *testing.T) {
	store := eval.NewInMemoryGoldenStore()
	gs := eval.GoldenSet{Version: "v1", Tasks: []eval.EvalTask{{ID: "x"}}}
	ctx := t.Context()
	if err := store.Save(ctx, gs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load(ctx, "v1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != "v1" {
		t.Errorf("Version: got %q, want %q", got.Version, "v1")
	}
}

func TestInMemoryGoldenStore_LoadNonExistent(t *testing.T) {
	store := eval.NewInMemoryGoldenStore()
	_, err := store.Load(t.Context(), "missing")
	if err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestInMemoryGoldenStore_LatestReturnsLastSaved(t *testing.T) {
	store := eval.NewInMemoryGoldenStore()
	ctx := t.Context()
	_ = store.Save(ctx, eval.GoldenSet{Version: "v1"})
	_ = store.Save(ctx, eval.GoldenSet{Version: "v2"})
	got, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.Version != "v2" {
		t.Errorf("expected latest to be v2, got %q", got.Version)
	}
}

// --- InMemoryResultStore tests ---

func TestInMemoryResultStore_SaveAndList(t *testing.T) {
	store := eval.NewInMemoryResultStore()
	ctx := t.Context()
	r1 := eval.EvalReport{GoldenVersion: "v1"}
	r2 := eval.EvalReport{GoldenVersion: "v2"}
	if err := store.SaveReport(ctx, r1); err != nil {
		t.Fatalf("SaveReport r1: %v", err)
	}
	if err := store.SaveReport(ctx, r2); err != nil {
		t.Fatalf("SaveReport r2: %v", err)
	}
	list, err := store.ListReports(ctx, 0)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 reports, got %d", len(list))
	}
}

func TestInMemoryResultStore_ListLimit(t *testing.T) {
	store := eval.NewInMemoryResultStore()
	ctx := t.Context()
	for i := 0; i < 5; i++ {
		_ = store.SaveReport(ctx, eval.EvalReport{GoldenVersion: "v"})
	}
	list, err := store.ListReports(ctx, 3)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 reports (limit), got %d", len(list))
	}
}
