package ontology

// Concept is a single node in the legal ontology: a classification unit
// such as "negligence", "breach of contract", or "consent" that rules and
// issues can link into (see link.go). This mirrors packages/category's
// Category convention of a small, data-driven struct rather than a fixed
// Go constant list, because the set of legal concepts is not fixed at
// compile time and varies by jurisdiction (see overlay.go).
type Concept struct {
	// ID uniquely identifies this concept within an OntologyStore.
	ID string `json:"id"`

	// Name is the canonical human-readable name of this concept (e.g.
	// "Negligence"). This is the fallback value returned by Label when no
	// language-specific label is available (see label.go).
	Name string `json:"name"`

	// Description is a short prose explanation of what this concept
	// covers.
	Description string `json:"description"`

	// CategoryCodes lists the packages/category taxonomy CategoryCodes
	// this concept is associated with (e.g. "civil", "criminal"), as
	// plain strings so this package does not need to import
	// packages/category's CategoryCode type directly for storage (it is
	// string-backed, so any packages/category.CategoryCode value can be
	// stored here via a simple string conversion).
	CategoryCodes []string `json:"category_codes,omitempty"`

	// Labels holds multilingual display labels for this concept, keyed
	// by language code (e.g. "en", "ar", "ur", "ta"). See label.go for
	// the Label helper and fallback behavior.
	Labels map[string]string `json:"labels,omitempty"`
}

// HasCategory reports whether c is associated with categoryCode.
func (c Concept) HasCategory(categoryCode string) bool {
	for _, code := range c.CategoryCodes {
		if code == categoryCode {
			return true
		}
	}
	return false
}
