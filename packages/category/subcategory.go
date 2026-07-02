package category

// maxParentChainDepth bounds ParentChain's walk so a taxonomy with an
// accidental cycle in ParentCode links fails fast (ErrUnknownParent)
// instead of looping forever. No legitimate taxonomy needs more than a
// handful of levels between a top-level category and its most specific
// sub-category.
const maxParentChainDepth = 32

// SubCategories returns every Category registered for jurisdictionCode
// whose ParentCode equals parent.Code, i.e. the direct children of parent
// in that jurisdiction's Taxonomy. The returned slice has no guaranteed
// order.
func SubCategories(taxonomy Taxonomy, jurisdictionCode string, parent Category) []Category {
	cats, ok := taxonomy[jurisdictionCode]
	if !ok {
		return nil
	}
	var children []Category
	for _, c := range cats {
		if c.ParentCode == parent.Code {
			children = append(children, c)
		}
	}
	return children
}

// ParentChain resolves the full ancestry of cat within jurisdictionCode's
// Taxonomy, starting with cat itself and walking up through each
// ParentCode until a top-level category (empty ParentCode) is reached. The
// returned slice is ordered from cat to its root ancestor.
//
// Returns ErrUnknownParent if any ParentCode in the chain does not resolve
// to a registered Category in that jurisdiction, or if the chain exceeds
// maxParentChainDepth (indicating a cycle).
func ParentChain(taxonomy Taxonomy, jurisdictionCode string, cat Category) ([]Category, error) {
	cats, ok := taxonomy[jurisdictionCode]
	if !ok {
		return nil, ErrUnknownJurisdiction
	}

	chain := []Category{cat}
	current := cat
	for i := 0; current.ParentCode != ""; i++ {
		if i >= maxParentChainDepth {
			return nil, ErrUnknownParent
		}
		parent, ok := cats[current.ParentCode]
		if !ok {
			return nil, ErrUnknownParent
		}
		chain = append(chain, parent)
		current = parent
	}
	return chain, nil
}

// ResolveParent returns the immediate parent Category of cat within
// jurisdictionCode's Taxonomy. Returns ok=false (with no error) if cat is a
// top-level category (empty ParentCode). Returns ErrUnknownParent if
// cat.ParentCode is set but does not resolve to a registered Category.
func ResolveParent(taxonomy Taxonomy, jurisdictionCode string, cat Category) (parent Category, ok bool, err error) {
	if cat.ParentCode == "" {
		return Category{}, false, nil
	}
	cats, exists := taxonomy[jurisdictionCode]
	if !exists {
		return Category{}, false, ErrUnknownJurisdiction
	}
	parent, found := cats[cat.ParentCode]
	if !found {
		return Category{}, false, ErrUnknownParent
	}
	return parent, true, nil
}

// ValidateSubCategory checks that cat's ParentCode chain fully resolves
// within jurisdictionCode's Taxonomy, i.e. ParentChain succeeds. This is a
// thin, intention-revealing wrapper around ParentChain for callers that
// only need a pass/fail result.
func ValidateSubCategory(taxonomy Taxonomy, jurisdictionCode string, cat Category) error {
	_, err := ParentChain(taxonomy, jurisdictionCode, cat)
	return err
}
