package category

import "fmt"

// ValidateCategory checks that category is present in jurisdictionCode's
// entry within taxonomy, and — when category is a sub-category — that its
// full ParentCode chain resolves within that same jurisdiction.
//
// Returns ErrUnknownJurisdiction if jurisdictionCode has no entry in
// taxonomy at all. Returns ErrCategoryNotInJurisdiction (wrapped with
// details via fmt.Errorf) if the jurisdiction is known but does not
// recognize category.Code, or if the registered Category for that code
// does not match category exactly (code, name, and parent must all agree
// with the taxonomy's record). Returns ErrUnknownParent if category's
// parent chain does not fully resolve.
func ValidateCategory(jurisdictionCode string, category Category, taxonomy Taxonomy) error {
	cats, ok := taxonomy[jurisdictionCode]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownJurisdiction, jurisdictionCode)
	}

	registered, ok := cats[category.Code]
	if !ok {
		return fmt.Errorf("%w: category %q not registered for jurisdiction %q", ErrCategoryNotInJurisdiction, category.Code, jurisdictionCode)
	}

	if registered != category {
		return fmt.Errorf("%w: category %q does not match jurisdiction %q's registered definition", ErrCategoryNotInJurisdiction, category.Code, jurisdictionCode)
	}

	if category.ParentCode != "" {
		if err := ValidateSubCategory(taxonomy, jurisdictionCode, category); err != nil {
			return err
		}
	}

	return nil
}
