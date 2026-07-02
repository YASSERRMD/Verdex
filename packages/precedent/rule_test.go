package precedent

import (
	"testing"
	"time"
)

func syntheticRawPrecedent() RawPrecedent {
	return RawPrecedent{
		CaseName:    "Donoghue v Stevenson",
		Citation:    "[1932] AC 562",
		Court:       "House of Lords",
		DecidedDate: time.Date(1932, 5, 26, 0, 0, 0, 0, time.UTC),
		FullText: `HELD: A manufacturer of products owes a duty of care to the
ultimate consumer.
RATIO: The neighbour principle requires reasonable care to avoid harm
that is reasonably foreseeable.`,
	}
}

func TestFormatCitation(t *testing.T) {
	tests := []struct {
		name     string
		caseName string
		citation string
		want     string
	}{
		{"both present", "Donoghue v Stevenson", "[1932] AC 562", "Donoghue v Stevenson [1932] AC 562"},
		{"citation only", "", "[1932] AC 562", "[1932] AC 562"},
		{"name only", "Donoghue v Stevenson", "", "Donoghue v Stevenson"},
		{"both blank", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCitation(tt.caseName, tt.citation); got != tt.want {
				t.Errorf("FormatCitation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildPrecedentRule(t *testing.T) {
	raw := syntheticRawPrecedent()
	rule, err := BuildPrecedentRule("precedent-0", raw, RuleBuildOptions{
		CaseID:           "precedent:UK",
		JurisdictionCode: "UK",
		LegalFamily:      "common_law",
	})
	if err != nil {
		t.Fatalf("BuildPrecedentRule() error = %v", err)
	}
	if rule.Citation != "Donoghue v Stevenson [1932] AC 562" {
		t.Errorf("Citation = %q, want Donoghue v Stevenson [1932] AC 562", rule.Citation)
	}
	if rule.Holding == "" {
		t.Error("Holding should not be empty")
	}
	if rule.RatioDecidendi == "" {
		t.Error("RatioDecidendi should not be empty")
	}
	if rule.JurisdictionCode != "UK" {
		t.Errorf("JurisdictionCode = %q, want UK", rule.JurisdictionCode)
	}
	if rule.LegalFamily != "common_law" {
		t.Errorf("LegalFamily = %q, want common_law", rule.LegalFamily)
	}
	if rule.CaseID != "precedent:UK" {
		t.Errorf("CaseID = %q, want precedent:UK", rule.CaseID)
	}
	if rule.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", rule.Confidence)
	}
	if rule.Text == "" {
		t.Error("underlying RuleNode.Text should not be empty")
	}
	if rule.Source.CaseName != raw.CaseName {
		t.Errorf("Source.CaseName = %q, want %q", rule.Source.CaseName, raw.CaseName)
	}
}

func TestBuildPrecedentRule_NoHoldingFallsBackToFullText(t *testing.T) {
	raw := RawPrecedent{CaseName: "No Marker Case", Citation: "[2001] 1 X 1", FullText: "The court discussed the matter at length without a clear determination."}
	rule, err := BuildPrecedentRule("precedent-0", raw, RuleBuildOptions{CaseID: "precedent:UK"})
	if err == nil {
		t.Fatal("BuildPrecedentRule() error = nil, want ErrHoldingNotFound")
	}
	if rule.Text == "" {
		t.Error("Text should fall back to FullText when no holding is found")
	}
	if rule.Holding != "" {
		t.Errorf("Holding = %q, want empty", rule.Holding)
	}
}

func TestBuildPrecedentRule_EmptyCaseID(t *testing.T) {
	_, err := BuildPrecedentRule("precedent-0", syntheticRawPrecedent(), RuleBuildOptions{CaseID: ""})
	if err == nil {
		t.Fatal("BuildPrecedentRule() error = nil, want error for empty CaseID")
	}
}

func TestBuildPrecedentRules_UniqueIDsAndFailedTracking(t *testing.T) {
	raws := []RawPrecedent{
		syntheticRawPrecedent(),
		{CaseName: "No Holding Case", FullText: "No recognizable marker here."},
	}
	rules, failedIDs, err := BuildPrecedentRules(raws, RuleBuildOptions{CaseID: "precedent:UK", IDPrefix: "p"})
	if err != nil {
		t.Fatalf("BuildPrecedentRules() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	if rules[0].ID != "p-0" || rules[1].ID != "p-1" {
		t.Errorf("IDs = %q, %q, want p-0, p-1", rules[0].ID, rules[1].ID)
	}
	if len(failedIDs) != 1 || failedIDs[0] != "p-1" {
		t.Errorf("failedIDs = %v, want [p-1]", failedIDs)
	}
}

func TestBuildPrecedentRules_EmptyCaseID(t *testing.T) {
	_, _, err := BuildPrecedentRules([]RawPrecedent{syntheticRawPrecedent()}, RuleBuildOptions{CaseID: ""})
	if err == nil {
		t.Fatal("BuildPrecedentRules() error = nil, want error for empty CaseID")
	}
}
