package ontology

// RelationType classifies how two Concepts relate to one another. This
// mirrors packages/irac's NodeType convention of a small string-backed
// enum with one constant per recognized kind.
type RelationType string

const (
	// RelIsA denotes a specialization relationship: the FromConceptID is
	// a kind of the ToConceptID (e.g. "gross-negligence" RelIsA
	// "negligence").
	RelIsA RelationType = "is_a"

	// RelPartOf denotes a compositional relationship: the FromConceptID
	// is a component of the ToConceptID (e.g. "offer" RelPartOf
	// "contract-formation").
	RelPartOf RelationType = "part_of"

	// RelRelatedTo denotes a loose, non-hierarchical association between
	// two concepts.
	RelRelatedTo RelationType = "related_to"

	// RelContradicts denotes that the two concepts represent conflicting
	// or mutually exclusive legal positions (e.g. "consent" RelContradicts
	// "coercion").
	RelContradicts RelationType = "contradicts"
)

// allRelationTypes is the exhaustive set of recognized RelationType
// values, used by IsValid.
var allRelationTypes = map[RelationType]struct{}{
	RelIsA:         {},
	RelPartOf:      {},
	RelRelatedTo:   {},
	RelContradicts: {},
}

// IsValid reports whether t is one of the recognized RelationType
// constants.
func (t RelationType) IsValid() bool {
	_, ok := allRelationTypes[t]
	return ok
}

// AllRelationTypes returns every recognized RelationType, in the fixed
// order declared above.
func AllRelationTypes() []RelationType {
	return []RelationType{
		RelIsA,
		RelPartOf,
		RelRelatedTo,
		RelContradicts,
	}
}

// Relation links two Concepts (by ID) with a RelationType, e.g.
// "gross-negligence" RelIsA "negligence".
type Relation struct {
	// FromConceptID is the source concept's ID.
	FromConceptID string `json:"from_concept_id"`

	// ToConceptID is the target concept's ID.
	ToConceptID string `json:"to_concept_id"`

	// Type classifies how FromConceptID relates to ToConceptID.
	Type RelationType `json:"type"`
}

// NewRelation constructs a Relation from fromConceptID to toConceptID of
// the given RelationType.
func NewRelation(fromConceptID, toConceptID string, relType RelationType) Relation {
	return Relation{
		FromConceptID: fromConceptID,
		ToConceptID:   toConceptID,
		Type:          relType,
	}
}
