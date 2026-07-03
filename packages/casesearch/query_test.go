package casesearch_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/casesearch"
)

func TestQuery_NewQuery_DefaultsToModeAuto(t *testing.T) {
	q := casesearch.NewQuery("breach of contract")
	if q.Mode != casesearch.ModeAuto {
		t.Fatalf("expected zero-value Mode (ModeAuto), got %q", q.Mode)
	}
	if q.Text != "breach of contract" {
		t.Fatalf("expected Text preserved, got %q", q.Text)
	}
}

func TestMode_IsValid(t *testing.T) {
	cases := []struct {
		mode casesearch.Mode
		want bool
	}{
		{casesearch.ModeAuto, true},
		{casesearch.ModeKeyword, true},
		{casesearch.ModeSemantic, true},
		{casesearch.ModeIssueRule, true},
		{casesearch.Mode("bogus"), false},
	}
	for _, tc := range cases {
		if got := tc.mode.IsValid(); got != tc.want {
			t.Errorf("Mode(%q).IsValid() = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestFilter_IsZero(t *testing.T) {
	if !(casesearch.Filter{}).IsZero() {
		t.Fatal("expected zero-value Filter to be IsZero")
	}
	if (casesearch.Filter{CategoryCode: "civil"}).IsZero() {
		t.Fatal("expected non-empty CategoryCode to make Filter non-zero")
	}
}

func TestQuery_WithBuilders_ChainWithoutMutatingOriginal(t *testing.T) {
	base := casesearch.NewQuery("original")
	derived := base.
		WithMode(casesearch.ModeKeyword).
		WithIssueOrRule("rule-1").
		WithTopKPerCase(3).
		WithPage(casesearch.Page{Number: 2, Size: 10})

	if base.Mode != casesearch.ModeAuto {
		t.Fatalf("expected base.Mode unchanged, got %q", base.Mode)
	}
	if derived.Mode != casesearch.ModeKeyword {
		t.Fatalf("expected derived.Mode = keyword, got %q", derived.Mode)
	}
	if derived.IssueOrRuleID != "rule-1" {
		t.Fatalf("expected derived.IssueOrRuleID = rule-1, got %q", derived.IssueOrRuleID)
	}
	if derived.TopKPerCase != 3 {
		t.Fatalf("expected derived.TopKPerCase = 3, got %d", derived.TopKPerCase)
	}
	if derived.Page.Number != 2 || derived.Page.Size != 10 {
		t.Fatalf("expected derived.Page = {2 10}, got %+v", derived.Page)
	}
}
