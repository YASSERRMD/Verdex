package precedent

import "testing"

func syntheticPrecedentRule(t *testing.T) PrecedentRule {
	t.Helper()
	rule, err := BuildPrecedentRule("precedent-0", syntheticRawPrecedent(), RuleBuildOptions{CaseID: "precedent:UK"})
	if err != nil {
		t.Fatalf("BuildPrecedentRule() error = %v", err)
	}
	return rule
}

func TestTagPrecedents(t *testing.T) {
	rule := syntheticPrecedentRule(t)
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{
		CategoryCode:     "tort",
		JurisdictionCode: "UK",
		LegalFamily:      "common_law",
	})
	if len(tagged) != 1 {
		t.Fatalf("len(tagged) = %d, want 1", len(tagged))
	}
	tr := tagged[0]
	if tr.CategoryCode != "tort" {
		t.Errorf("CategoryCode = %q, want tort", tr.CategoryCode)
	}
	if tr.JurisdictionCode != "UK" {
		t.Errorf("JurisdictionCode = %q, want UK", tr.JurisdictionCode)
	}
	if tr.LegalFamily != "common_law" {
		t.Errorf("LegalFamily = %q, want common_law", tr.LegalFamily)
	}
	if len(tr.IssueKeywords) == 0 {
		t.Error("IssueKeywords should not be empty")
	}
}

func TestTagPrecedents_PreservesExistingWhenOptEmpty(t *testing.T) {
	rule, err := BuildPrecedentRule("precedent-0", syntheticRawPrecedent(), RuleBuildOptions{
		CaseID:           "precedent:UK",
		JurisdictionCode: "PK",
		LegalFamily:      "common_law",
	})
	if err != nil {
		t.Fatalf("BuildPrecedentRule() error = %v", err)
	}
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "criminal"})
	if tagged[0].JurisdictionCode != "PK" {
		t.Errorf("JurisdictionCode = %q, want PK (unchanged)", tagged[0].JurisdictionCode)
	}
	if tagged[0].LegalFamily != "common_law" {
		t.Errorf("LegalFamily = %q, want common_law (unchanged)", tagged[0].LegalFamily)
	}
}

func TestExtractIssueKeywords(t *testing.T) {
	text := "A manufacturer of products owes a duty of care to the ultimate consumer of those products."
	keywords := ExtractIssueKeywords(text, 5)
	if len(keywords) == 0 {
		t.Fatal("ExtractIssueKeywords() returned no keywords")
	}
	if len(keywords) > 5 {
		t.Errorf("len(keywords) = %d, want <= 5", len(keywords))
	}
	for _, kw := range keywords {
		if _, stop := stopWords[kw]; stop {
			t.Errorf("keyword %q is a stop word and should have been excluded", kw)
		}
	}
}

func TestExtractIssueKeywords_NoDuplicates(t *testing.T) {
	text := "duty duty duty care care manufacturer"
	keywords := ExtractIssueKeywords(text, 10)
	seen := make(map[string]bool)
	for _, kw := range keywords {
		if seen[kw] {
			t.Errorf("duplicate keyword %q", kw)
		}
		seen[kw] = true
	}
}

func TestExtractIssueKeywords_EmptyText(t *testing.T) {
	keywords := ExtractIssueKeywords("", 5)
	if len(keywords) != 0 {
		t.Errorf("keywords = %v, want empty", keywords)
	}
}

func TestExtractIssueKeywords_DefaultMax(t *testing.T) {
	text := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"
	keywords := ExtractIssueKeywords(text, 0)
	if len(keywords) != defaultMaxKeywords {
		t.Errorf("len(keywords) = %d, want %d (default cap)", len(keywords), defaultMaxKeywords)
	}
}
