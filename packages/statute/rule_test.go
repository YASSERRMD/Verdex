package statute

import "testing"

func syntheticAct(t *testing.T) *StatuteNode {
	t.Helper()
	act, err := ParseHierarchy(RawStatute{
		ActNumber: "12",
		ActTitle:  "Contracts Act",
		Body: `Section 1. Definitions
(a) "party" means a natural or legal person.
(b) "contract" means an agreement enforceable by law.
Section 2. Formation
A contract is formed when an offer is accepted.
`,
	})
	if err != nil {
		t.Fatalf("ParseHierarchy() error = %v", err)
	}
	return act
}

func TestCitation_String(t *testing.T) {
	tests := []struct {
		name string
		c    Citation
		want string
	}{
		{"act only", Citation{Act: "12"}, "Act 12"},
		{"act and section", Citation{Act: "12", Section: "5"}, "Act 12, s.5"},
		{"act section clause", Citation{Act: "12", Section: "5", Clause: "a"}, "Act 12, s.5(a)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRuleNodes_ClauseGranularity(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{
		CaseID:           "statute:AE",
		JurisdictionCode: "AE",
		LegalFamily:      "civil_law",
	})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	// Section 1 has 2 clauses; Section 2 has no clauses (leaf itself) -> 3 rules total.
	if len(built) != 3 {
		t.Fatalf("len(built) = %d, want 3", len(built))
	}

	if built[0].Citation.String() != "Act 12, s.1(a)" {
		t.Errorf("built[0].Citation = %q, want Act 12, s.1(a)", built[0].Citation.String())
	}
	if built[1].Citation.String() != "Act 12, s.1(b)" {
		t.Errorf("built[1].Citation = %q, want Act 12, s.1(b)", built[1].Citation.String())
	}
	if built[2].Citation.String() != "Act 12, s.2" {
		t.Errorf("built[2].Citation = %q, want Act 12, s.2", built[2].Citation.String())
	}

	for _, b := range built {
		if b.Node.JurisdictionCode != "AE" {
			t.Errorf("JurisdictionCode = %q, want AE", b.Node.JurisdictionCode)
		}
		if b.Node.LegalFamily != "civil_law" {
			t.Errorf("LegalFamily = %q, want civil_law", b.Node.LegalFamily)
		}
		if b.Node.Confidence != 1.0 {
			t.Errorf("Confidence = %v, want 1.0", b.Node.Confidence)
		}
		if b.Node.Text == "" {
			t.Error("rule text should not be empty")
		}
		if b.Node.CaseID != "statute:AE" {
			t.Errorf("CaseID = %q, want statute:AE", b.Node.CaseID)
		}
	}
}

func TestBuildRuleNodes_SectionGranularity(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{
		Granularity: GranularitySection,
		CaseID:      "statute:AE",
	})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	if len(built) != 2 {
		t.Fatalf("len(built) = %d, want 2", len(built))
	}
	if built[0].Citation.Clause != "" {
		t.Errorf("section-granularity rule should have no clause citation, got %q", built[0].Citation.Clause)
	}
	// Section 1 has no direct text but has clause children -> text should be
	// derived from concatenating clause text.
	if built[0].Node.Text == "" {
		t.Error("section 1 rule text should be derived from its clauses")
	}
}

func TestBuildRuleNodes_Errors(t *testing.T) {
	if _, err := BuildRuleNodes(nil, RuleBuildOptions{CaseID: "x"}); err == nil {
		t.Error("BuildRuleNodes(nil, ...) error = nil, want error")
	}
	act := syntheticAct(t)
	if _, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: ""}); err == nil {
		t.Error("BuildRuleNodes(act, empty CaseID) error = nil, want error")
	}
}

func TestBuildRuleNodes_UniqueIDs(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE", IDPrefix: "r"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	seen := make(map[string]bool)
	for _, b := range built {
		if seen[b.Node.ID] {
			t.Errorf("duplicate rule ID: %q", b.Node.ID)
		}
		seen[b.Node.ID] = true
	}
}
