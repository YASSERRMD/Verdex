package category

import "testing"

func TestProceduralRules_RegisterAndLookup(t *testing.T) {
	p := NewProceduralRules()

	cpc := ProceduralRuleRef{Code: "CPC", Name: "Code of Civil Procedure"}
	crpc := ProceduralRuleRef{Code: "CrPC", Name: "Code of Criminal Procedure"}

	p.Register("IN", CodeCivil, cpc)
	p.Register("IN", CodeCriminal, crpc)

	tests := []struct {
		name             string
		jurisdictionCode string
		categoryCode     CategoryCode
		wantCodes        []string
	}{
		{"civil in IN", "IN", CodeCivil, []string{"CPC"}},
		{"criminal in IN", "IN", CodeCriminal, []string{"CrPC"}},
		{"unregistered category", "IN", CodeLabor, nil},
		{"unregistered jurisdiction", "AE", CodeCivil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Lookup(tt.jurisdictionCode, tt.categoryCode)
			if len(got) != len(tt.wantCodes) {
				t.Fatalf("got %d refs, want %d", len(got), len(tt.wantCodes))
			}
			for i, ref := range got {
				if ref.Code != tt.wantCodes[i] {
					t.Errorf("ref[%d].Code = %q, want %q", i, ref.Code, tt.wantCodes[i])
				}
			}
		})
	}
}

func TestProceduralRules_RegisterAppends(t *testing.T) {
	p := NewProceduralRules()
	p.Register("IN", CodeCivil, ProceduralRuleRef{Code: "CPC"})
	p.Register("IN", CodeCivil, ProceduralRuleRef{Code: "Evidence-Act"})

	got := p.Lookup("IN", CodeCivil)
	if len(got) != 2 {
		t.Fatalf("got %d refs, want 2 (register should append, not replace)", len(got))
	}
}

func TestProceduralRules_LookupCategory(t *testing.T) {
	p := NewProceduralRules()
	p.Register("IN", CodeCivil, ProceduralRuleRef{Code: "CPC"})

	civil := Category{Code: CodeCivil, Name: "Civil"}
	got := p.LookupCategory("IN", civil)
	if len(got) != 1 || got[0].Code != "CPC" {
		t.Errorf("LookupCategory() = %v, want [{CPC ...}]", got)
	}
}

func TestProceduralRules_ZeroValue(t *testing.T) {
	var p ProceduralRules
	got := p.Lookup("IN", CodeCivil)
	if got != nil {
		t.Errorf("zero-value Lookup() = %v, want nil", got)
	}

	// Register on the zero value must not panic and must lazily init the
	// underlying table.
	p.Register("IN", CodeCivil, ProceduralRuleRef{Code: "CPC"})
	got = p.Lookup("IN", CodeCivil)
	if len(got) != 1 {
		t.Errorf("got %d refs after Register on zero value, want 1", len(got))
	}
}
