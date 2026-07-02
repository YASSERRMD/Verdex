package ontology

import "sort"

// JurisdictionOverlay holds additional or modified Concepts for a single
// jurisdiction, keyed by an opaque jurisdiction code string (mirroring
// packages/category's Taxonomy convention of keying by jurisdiction code
// rather than importing packages/jurisdiction's CountryCode type
// directly).
//
// An overlay's Concepts are merged on top of the core seed set produced
// by SeedCoreConcepts: a Concept in the overlay whose ID matches a core
// concept's ID replaces (modifies) it, while a Concept with a new ID adds
// to the set.
type JurisdictionOverlay struct {
	// JurisdictionCode identifies the jurisdiction this overlay applies
	// to (e.g. an ISO 3166-1 alpha-2 country code, or a more specific
	// court/jurisdiction key).
	JurisdictionCode string `json:"jurisdiction_code"`

	// Concepts is the set of additional or modified Concepts this overlay
	// contributes, keyed by Concept.ID for O(1) merge lookup.
	Concepts map[string]Concept `json:"concepts"`
}

// NewJurisdictionOverlay constructs an empty JurisdictionOverlay for
// jurisdictionCode.
func NewJurisdictionOverlay(jurisdictionCode string) JurisdictionOverlay {
	return JurisdictionOverlay{
		JurisdictionCode: jurisdictionCode,
		Concepts:         make(map[string]Concept),
	}
}

// AddConcept registers concept in the overlay, keyed by concept.ID. If a
// Concept with the same ID is already present in the overlay, it is
// replaced.
func (o *JurisdictionOverlay) AddConcept(concept Concept) {
	if o.Concepts == nil {
		o.Concepts = make(map[string]Concept)
	}
	o.Concepts[concept.ID] = concept
}

// MergeOverlay applies overlay on top of core: every Concept in core is
// included in the result, and every Concept in overlay.Concepts either
// replaces the core Concept sharing its ID or is added as a new entry.
// core is not mutated. Concepts not touched by the overlay are returned
// unchanged ("doesn't clobber core concepts unexpectedly").
//
// The result is returned in a stable order: core concepts first (in
// their original order, overlaid in place when replaced), then
// overlay-only concepts appended in a deterministic (sorted by ID) order.
func MergeOverlay(core []Concept, overlay JurisdictionOverlay) []Concept {
	out := make([]Concept, len(core))
	copy(out, core)

	seen := make(map[string]int, len(core))
	for i, c := range out {
		seen[c.ID] = i
	}

	var newIDs []string
	for id := range overlay.Concepts {
		if _, ok := seen[id]; !ok {
			newIDs = append(newIDs, id)
		}
	}
	sort.Strings(newIDs)

	for i, c := range out {
		if replacement, ok := overlay.Concepts[c.ID]; ok {
			out[i] = replacement
		}
	}
	for _, id := range newIDs {
		out = append(out, overlay.Concepts[id])
	}
	return out
}
