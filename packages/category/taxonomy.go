package category

// CategoryCode is a short machine-readable identifier for a case category
// (e.g. "civil", "criminal", "domestic-violence"). It mirrors
// packages/evidence's EvidenceType convention of a small string-backed enum,
// except the category set is not a fixed Go constant list: it is data,
// keyed per jurisdiction, because which categories (and sub-categories) are
// recognized varies by legal system.
type CategoryCode string

// Top-level category codes recognized across jurisdictions. A Taxonomy is
// free to include only a subset of these for a given jurisdiction, and to
// add jurisdiction-specific sub-categories beneath them (see subcategory.go).
const (
	CodeCivil            CategoryCode = "civil"
	CodeCriminal         CategoryCode = "criminal"
	CodeDomesticViolence CategoryCode = "domestic-violence"
	CodeConsumer         CategoryCode = "consumer"
	CodeFamily           CategoryCode = "family"
	CodeCommercial       CategoryCode = "commercial"
	CodeLabor            CategoryCode = "labor"
	CodeOther            CategoryCode = "other"
)

// Category describes a single case category: its machine-readable Code, a
// human-readable Name, and an optional ParentCode identifying the
// top-level category this is a sub-category of.
//
// A Category with an empty ParentCode is a top-level category. A non-empty
// ParentCode must reference another Category.Code present in the same
// Taxonomy (see subcategory.go for chain resolution and validation).
type Category struct {
	// Code is the short machine-readable identifier for this category
	// (e.g. "civil", "civil-contract").
	Code CategoryCode `json:"code"`

	// Name is the human-readable name of the category.
	Name string `json:"name"`

	// ParentCode identifies the parent category's Code when this Category is
	// a sub-category. Empty for top-level categories.
	ParentCode CategoryCode `json:"parent_code,omitempty"`
}

// IsTopLevel reports whether c has no parent, i.e. it is one of the
// top-level categories rather than a sub-category.
func (c Category) IsTopLevel() bool {
	return c.ParentCode == ""
}

// Taxonomy holds the set of valid Categories for one or more jurisdictions,
// keyed by jurisdiction code (mirroring packages/jurisdiction's CountryCode
// convention — a short jurisdiction identifier such as an ISO 3166-1
// alpha-2 country code, or a more specific court/jurisdiction key when a
// deployment needs sub-national granularity).
//
// Each jurisdiction maps to the set of Categories valid for cases in that
// jurisdiction, keyed by CategoryCode for O(1) lookup.
type Taxonomy map[string]map[CategoryCode]Category

// DefaultTopLevelCategories returns the standard set of top-level
// Categories shared by every jurisdiction seeded via NewDefaultTaxonomy:
// civil, criminal, domestic-violence, consumer, family, commercial, labor,
// and other. Callers building a jurisdiction-specific Taxonomy from scratch
// can start from this list and add/remove entries as needed.
func DefaultTopLevelCategories() []Category {
	return []Category{
		{Code: CodeCivil, Name: "Civil"},
		{Code: CodeCriminal, Name: "Criminal"},
		{Code: CodeDomesticViolence, Name: "Domestic Violence"},
		{Code: CodeConsumer, Name: "Consumer"},
		{Code: CodeFamily, Name: "Family"},
		{Code: CodeCommercial, Name: "Commercial"},
		{Code: CodeLabor, Name: "Labor"},
		{Code: CodeOther, Name: "Other"},
	}
}

// NewDefaultTaxonomy builds a Taxonomy that seeds the given jurisdiction
// codes with DefaultTopLevelCategories(). Duplicate jurisdiction codes are
// ignored (the first occurrence wins). Useful for tests and as a starting
// point before layering jurisdiction-specific sub-categories via
// AddCategory.
func NewDefaultTaxonomy(jurisdictionCodes ...string) Taxonomy {
	t := make(Taxonomy, len(jurisdictionCodes))
	for _, jc := range jurisdictionCodes {
		if _, ok := t[jc]; ok {
			continue
		}
		cats := make(map[CategoryCode]Category)
		for _, c := range DefaultTopLevelCategories() {
			cats[c.Code] = c
		}
		t[jc] = cats
	}
	return t
}

// AddCategory registers cat as valid for jurisdictionCode, creating the
// jurisdiction's entry if it does not already exist. Returns
// ErrUnknownParent if cat has a non-empty ParentCode that is not already
// present in that jurisdiction's taxonomy.
func (t Taxonomy) AddCategory(jurisdictionCode string, cat Category) error {
	cats, ok := t[jurisdictionCode]
	if !ok {
		cats = make(map[CategoryCode]Category)
		t[jurisdictionCode] = cats
	}
	if cat.ParentCode != "" {
		if _, ok := cats[cat.ParentCode]; !ok {
			return ErrUnknownParent
		}
	}
	cats[cat.Code] = cat
	return nil
}

// Categories returns every Category registered for jurisdictionCode. The
// returned slice has no guaranteed order. Returns nil if the jurisdiction
// has no entries in t.
func (t Taxonomy) Categories(jurisdictionCode string) []Category {
	cats, ok := t[jurisdictionCode]
	if !ok {
		return nil
	}
	out := make([]Category, 0, len(cats))
	for _, c := range cats {
		out = append(out, c)
	}
	return out
}

// Lookup returns the Category registered under code for jurisdictionCode.
// The second return value is false if the jurisdiction is unknown to t or
// the code is not registered for it.
func (t Taxonomy) Lookup(jurisdictionCode string, code CategoryCode) (Category, bool) {
	cats, ok := t[jurisdictionCode]
	if !ok {
		return Category{}, false
	}
	c, ok := cats[code]
	return c, ok
}

// HasJurisdiction reports whether jurisdictionCode has any categories
// registered in t.
func (t Taxonomy) HasJurisdiction(jurisdictionCode string) bool {
	_, ok := t[jurisdictionCode]
	return ok
}
