package category

import (
	"errors"
	"testing"
)

func TestValidateCategory(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	if err := tax.AddCategory("IN", Category{Code: "civil-contract", Name: "Civil - Contract", ParentCode: CodeCivil}); err != nil {
		t.Fatalf("AddCategory() error = %v", err)
	}

	civil, _ := tax.Lookup("IN", CodeCivil)
	sub, _ := tax.Lookup("IN", "civil-contract")

	tests := []struct {
		name             string
		jurisdictionCode string
		category         Category
		wantErr          error
	}{
		{
			name:             "valid top-level category",
			jurisdictionCode: "IN",
			category:         civil,
			wantErr:          nil,
		},
		{
			name:             "valid sub-category",
			jurisdictionCode: "IN",
			category:         sub,
			wantErr:          nil,
		},
		{
			name:             "unknown jurisdiction",
			jurisdictionCode: "ZZ",
			category:         civil,
			wantErr:          ErrUnknownJurisdiction,
		},
		{
			name:             "category not in jurisdiction",
			jurisdictionCode: "IN",
			category:         Category{Code: "not-real", Name: "Not Real"},
			wantErr:          ErrCategoryNotInJurisdiction,
		},
		{
			name:             "category mismatched name from registered definition",
			jurisdictionCode: "IN",
			category:         Category{Code: CodeCivil, Name: "Wrong Name"},
			wantErr:          ErrCategoryNotInJurisdiction,
		},
		{
			name:             "sub-category with unresolvable parent",
			jurisdictionCode: "IN",
			category:         Category{Code: "orphan-sub", Name: "Orphan", ParentCode: "does-not-exist"},
			wantErr:          ErrCategoryNotInJurisdiction, // fails the "not registered" check first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCategory(tt.jurisdictionCode, tt.category, tax)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateCategory() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateCategory() error = nil, want wrapping %v", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateCategory() error = %v, want wrapping %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCategory_RegisteredButUnresolvableParent(t *testing.T) {
	// Build a taxonomy where a sub-category is registered directly (bypassing
	// AddCategory's own parent check) with a parent that is not present, to
	// exercise ValidateCategory's ValidateSubCategory branch specifically.
	tax := make(Taxonomy)
	tax["IN"] = map[CategoryCode]Category{
		CodeCivil: {Code: CodeCivil, Name: "Civil"},
		"orphan":  {Code: "orphan", Name: "Orphan", ParentCode: "does-not-exist"},
	}

	orphan := tax["IN"]["orphan"]
	err := ValidateCategory("IN", orphan, tax)
	if !errors.Is(err, ErrUnknownParent) {
		t.Errorf("ValidateCategory() error = %v, want wrapping %v", err, ErrUnknownParent)
	}
}
