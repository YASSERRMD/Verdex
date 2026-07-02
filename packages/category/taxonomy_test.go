package category

import "testing"

func TestDefaultTopLevelCategories(t *testing.T) {
	cats := DefaultTopLevelCategories()
	wantCodes := []CategoryCode{
		CodeCivil, CodeCriminal, CodeDomesticViolence, CodeConsumer,
		CodeFamily, CodeCommercial, CodeLabor, CodeOther,
	}
	if len(cats) != len(wantCodes) {
		t.Fatalf("got %d categories, want %d", len(cats), len(wantCodes))
	}
	for i, c := range cats {
		if c.Code != wantCodes[i] {
			t.Errorf("category %d: got code %q, want %q", i, c.Code, wantCodes[i])
		}
		if c.Name == "" {
			t.Errorf("category %d (%s): empty name", i, c.Code)
		}
		if !c.IsTopLevel() {
			t.Errorf("category %d (%s): expected top-level (empty ParentCode)", i, c.Code)
		}
	}
}

func TestNewDefaultTaxonomy(t *testing.T) {
	tests := []struct {
		name          string
		jurisdictions []string
	}{
		{"single jurisdiction", []string{"IN"}},
		{"multiple jurisdictions", []string{"IN", "AE", "PK"}},
		{"duplicate jurisdiction codes", []string{"IN", "IN"}},
		{"no jurisdictions", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tax := NewDefaultTaxonomy(tt.jurisdictions...)

			distinct := map[string]bool{}
			for _, j := range tt.jurisdictions {
				distinct[j] = true
			}

			if len(tax) != len(distinct) {
				t.Fatalf("got %d jurisdictions in taxonomy, want %d", len(tax), len(distinct))
			}
			for j := range distinct {
				cats := tax.Categories(j)
				if len(cats) != len(DefaultTopLevelCategories()) {
					t.Errorf("jurisdiction %q: got %d categories, want %d", j, len(cats), len(DefaultTopLevelCategories()))
				}
			}
		})
	}
}

func TestTaxonomy_AddCategory(t *testing.T) {
	t.Run("valid top-level category into new jurisdiction", func(t *testing.T) {
		tax := make(Taxonomy)
		err := tax.AddCategory("IN", Category{Code: CodeCivil, Name: "Civil"})
		if err != nil {
			t.Fatalf("AddCategory() error = %v, want nil", err)
		}
		got, ok := tax.Lookup("IN", CodeCivil)
		if !ok {
			t.Fatal("Lookup() ok = false, want true")
		}
		if got.Name != "Civil" {
			t.Errorf("got name %q, want %q", got.Name, "Civil")
		}
	})

	t.Run("valid sub-category with existing parent", func(t *testing.T) {
		tax := NewDefaultTaxonomy("IN")
		err := tax.AddCategory("IN", Category{Code: "civil-contract", Name: "Civil - Contract", ParentCode: CodeCivil})
		if err != nil {
			t.Fatalf("AddCategory() error = %v, want nil", err)
		}
		got, ok := tax.Lookup("IN", "civil-contract")
		if !ok || got.ParentCode != CodeCivil {
			t.Errorf("sub-category not registered correctly: %+v (ok=%v)", got, ok)
		}
	})

	t.Run("sub-category with unknown parent is rejected", func(t *testing.T) {
		tax := make(Taxonomy)
		err := tax.AddCategory("IN", Category{Code: "civil-contract", Name: "Civil - Contract", ParentCode: CodeCivil})
		if err != ErrUnknownParent {
			t.Fatalf("AddCategory() error = %v, want %v", err, ErrUnknownParent)
		}
	})
}

func TestTaxonomy_Lookup(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")

	t.Run("known jurisdiction and code", func(t *testing.T) {
		got, ok := tax.Lookup("IN", CodeCriminal)
		if !ok {
			t.Fatal("Lookup() ok = false, want true")
		}
		if got.Code != CodeCriminal {
			t.Errorf("got code %q, want %q", got.Code, CodeCriminal)
		}
	})

	t.Run("unknown jurisdiction", func(t *testing.T) {
		_, ok := tax.Lookup("ZZ", CodeCriminal)
		if ok {
			t.Fatal("Lookup() ok = true, want false")
		}
	})

	t.Run("unknown code in known jurisdiction", func(t *testing.T) {
		_, ok := tax.Lookup("IN", "not-a-real-code")
		if ok {
			t.Fatal("Lookup() ok = true, want false")
		}
	})
}

func TestTaxonomy_HasJurisdiction(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	if !tax.HasJurisdiction("IN") {
		t.Error("HasJurisdiction(\"IN\") = false, want true")
	}
	if tax.HasJurisdiction("ZZ") {
		t.Error("HasJurisdiction(\"ZZ\") = true, want false")
	}
}

func TestCategory_IsTopLevel(t *testing.T) {
	tests := []struct {
		name string
		cat  Category
		want bool
	}{
		{"empty parent code", Category{Code: CodeCivil}, true},
		{"non-empty parent code", Category{Code: "civil-contract", ParentCode: CodeCivil}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cat.IsTopLevel(); got != tt.want {
				t.Errorf("IsTopLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
