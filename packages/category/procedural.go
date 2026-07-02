package category

// ProceduralRuleRef is a reference to a procedural code or rule that
// governs practice for cases of a given category. It deliberately mirrors
// packages/jurisdiction's ProceduralRule shape (Code/Name/Description)
// rather than importing that package, so packages/category has no
// compile-time dependency on packages/jurisdiction — jurisdiction is
// referenced here only by its string CountryCode/jurisdiction-code key, the
// same loose-coupling convention used by Taxonomy (see taxonomy.go).
type ProceduralRuleRef struct {
	// Code is a short machine-readable identifier (e.g. "CPC", "CrPC").
	Code string `json:"code"`

	// Name is the human-readable name of the procedural instrument.
	Name string `json:"name"`

	// Description provides a brief overview of the rule's scope and
	// applicability to the category it is mapped from.
	Description string `json:"description"`
}

// proceduralRuleTable maps jurisdictionCode -> CategoryCode -> the set of
// ProceduralRuleRefs that govern cases of that category in that
// jurisdiction. Populated via RegisterProceduralRules; empty by default so
// a fresh ProceduralRules value never returns stale data.
type proceduralRuleTable map[string]map[CategoryCode][]ProceduralRuleRef

// ProceduralRules is a lookup table mapping a case Category to the
// procedural rules that govern it, per jurisdiction. The zero value is
// ready to use (an empty table).
type ProceduralRules struct {
	table proceduralRuleTable
}

// NewProceduralRules constructs an empty ProceduralRules lookup table.
func NewProceduralRules() *ProceduralRules {
	return &ProceduralRules{table: make(proceduralRuleTable)}
}

// Register associates refs with categoryCode for jurisdictionCode,
// appending to (not replacing) any refs already registered for that pair.
func (p *ProceduralRules) Register(jurisdictionCode string, categoryCode CategoryCode, refs ...ProceduralRuleRef) {
	if p.table == nil {
		p.table = make(proceduralRuleTable)
	}
	byCategory, ok := p.table[jurisdictionCode]
	if !ok {
		byCategory = make(map[CategoryCode][]ProceduralRuleRef)
		p.table[jurisdictionCode] = byCategory
	}
	byCategory[categoryCode] = append(byCategory[categoryCode], refs...)
}

// Lookup returns every ProceduralRuleRef registered for categoryCode within
// jurisdictionCode. Returns nil if nothing is registered for that pair.
func (p *ProceduralRules) Lookup(jurisdictionCode string, categoryCode CategoryCode) []ProceduralRuleRef {
	if p.table == nil {
		return nil
	}
	byCategory, ok := p.table[jurisdictionCode]
	if !ok {
		return nil
	}
	return byCategory[categoryCode]
}

// LookupCategory is a convenience wrapper around Lookup that takes a full
// Category value rather than a bare CategoryCode.
func (p *ProceduralRules) LookupCategory(jurisdictionCode string, cat Category) []ProceduralRuleRef {
	return p.Lookup(jurisdictionCode, cat.Code)
}
