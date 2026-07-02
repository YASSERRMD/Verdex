package ontology

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrConceptNotFound is returned when a lookup by Concept.ID finds no
	// matching record.
	ErrConceptNotFound = errors.New("ontology: concept not found")

	// ErrDuplicateAlias is returned when AddAlias is called with an alias
	// that already resolves to a different concept ID.
	ErrDuplicateAlias = errors.New("ontology: alias already registered to a different concept")

	// ErrAliasNotFound is returned when ResolveAlias (or a store-backed
	// equivalent) finds no concept registered for the given alias.
	ErrAliasNotFound = errors.New("ontology: alias not found")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required input.
	ErrEmptyInput = errors.New("ontology: input is empty")

	// ErrRelationNotFound is returned when a lookup for a Relation finds
	// no matching record.
	ErrRelationNotFound = errors.New("ontology: relation not found")

	// ErrJurisdictionNotFound is returned when a lookup by jurisdiction
	// code finds no registered JurisdictionOverlay.
	ErrJurisdictionNotFound = errors.New("ontology: jurisdiction overlay not found")

	// ErrVersionNotFound is returned when a lookup by OntologyVersion
	// number finds no matching record.
	ErrVersionNotFound = errors.New("ontology: version not found")

	// ErrInvalidRelationType is returned when a Relation is constructed
	// or validated with a RelationType that is not one of the recognized
	// constants.
	ErrInvalidRelationType = errors.New("ontology: invalid relation type")
)
