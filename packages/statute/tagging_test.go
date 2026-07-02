package statute

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

func TestTagRules(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}

	tagged := TagRules(built, TagOptions{
		CategoryCode:     "civil",
		JurisdictionCode: "AE",
		LegalFamily:      jurisdiction.LegalFamilyCivilLaw,
	})

	if len(tagged) != len(built) {
		t.Fatalf("len(tagged) = %d, want %d", len(tagged), len(built))
	}
	for _, tr := range tagged {
		if tr.CategoryCode != "civil" {
			t.Errorf("CategoryCode = %q, want civil", tr.CategoryCode)
		}
		if tr.Node.JurisdictionCode != "AE" {
			t.Errorf("JurisdictionCode = %q, want AE", tr.Node.JurisdictionCode)
		}
		if tr.Node.LegalFamily != string(jurisdiction.LegalFamilyCivilLaw) {
			t.Errorf("LegalFamily = %q, want %q", tr.Node.LegalFamily, jurisdiction.LegalFamilyCivilLaw)
		}
	}
}

func TestTagRules_PreservesExistingWhenOptEmpty(t *testing.T) {
	act := syntheticAct(t)
	built, err := BuildRuleNodes(act, RuleBuildOptions{CaseID: "statute:AE", JurisdictionCode: "PK", LegalFamily: "common_law"})
	if err != nil {
		t.Fatalf("BuildRuleNodes() error = %v", err)
	}
	tagged := TagRules(built, TagOptions{CategoryCode: "criminal"})
	for _, tr := range tagged {
		if tr.Node.JurisdictionCode != "PK" {
			t.Errorf("JurisdictionCode = %q, want PK (unchanged)", tr.Node.JurisdictionCode)
		}
		if tr.Node.LegalFamily != "common_law" {
			t.Errorf("LegalFamily = %q, want common_law (unchanged)", tr.Node.LegalFamily)
		}
	}
}

func TestTagOptions_IsValidLegalFamily(t *testing.T) {
	tests := []struct {
		name string
		opts TagOptions
		want bool
	}{
		{"empty is valid", TagOptions{}, true},
		{"known family is valid", TagOptions{LegalFamily: jurisdiction.LegalFamilyIslamicLaw}, true},
		{"unknown family is invalid", TagOptions{LegalFamily: jurisdiction.LegalFamily("not-a-family")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.IsValidLegalFamily(); got != tt.want {
				t.Errorf("IsValidLegalFamily() = %v, want %v", got, tt.want)
			}
		})
	}
}
