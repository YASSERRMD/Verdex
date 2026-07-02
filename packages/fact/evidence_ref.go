package fact

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/evidence"
)

// EvidenceRef links a fact node back to the evidence.Classification (and,
// through it, the evidence.EvidenceType — testimony, exhibit, statutory
// citation, etc.) it was built from, so every fact traces to why it is
// believed. This mirrors packages/irac.Provenance's "trace back to how a
// node came to exist" convention, but at the evidentiary-basis level
// rather than the generating-process level: Provenance says which
// pipeline produced the node, EvidenceRef says which underlying evidence
// backs its truth.
type EvidenceRef struct {
	// FactID is the irac.FactNode.ID this reference belongs to.
	FactID string

	// SegmentID identifies the segmentation.Segment the classification
	// was derived from (see evidence.Classification.SegmentID).
	SegmentID string

	// ClassificationID is a stable identifier for the originating
	// evidence.Classification. evidence.Classification has no ID field
	// of its own (it is keyed by SegmentID within a batch), so callers
	// typically pass the SegmentID itself, or a caller-assigned batch
	// index/ID when multiple classifications can share a SegmentID.
	ClassificationID string

	// EvidenceType is the evidentiary role the originating segment plays
	// (testimony, documentary exhibit, statutory citation, argument,
	// physical exhibit, or other) — see evidence.EvidenceType.
	EvidenceType evidence.EvidenceType

	// PartyRole is the party the originating segment was attributed to
	// by the evidence classifier, when known (see evidence.PartyRole).
	PartyRole evidence.PartyRole

	// Confidence is the originating classification's own confidence
	// score, in the closed interval [0, 1], carried forward for
	// reliability scoring (see reliability.go).
	Confidence float64
}

// NewEvidenceRef builds an EvidenceRef linking factID to classification,
// using classificationID as the stable classification identifier (see
// EvidenceRef.ClassificationID's doc comment for when to pass something
// other than classification.SegmentID).
//
// Returns ErrClassificationInvalid if factID or classification.SegmentID
// is empty.
func NewEvidenceRef(factID string, classification evidence.Classification, classificationID string) (EvidenceRef, error) {
	if strings.TrimSpace(factID) == "" {
		return EvidenceRef{}, ErrClassificationInvalid
	}
	if strings.TrimSpace(classification.SegmentID) == "" {
		return EvidenceRef{}, ErrClassificationInvalid
	}

	id := classificationID
	if strings.TrimSpace(id) == "" {
		id = classification.SegmentID
	}

	return EvidenceRef{
		FactID:           factID,
		SegmentID:        classification.SegmentID,
		ClassificationID: id,
		EvidenceType:     classification.Type,
		PartyRole:        classification.Party,
		Confidence:       classification.Confidence,
	}, nil
}

// IsTestimonial reports whether ref's EvidenceType is testimonial
// (witness statement) evidence.
func (ref EvidenceRef) IsTestimonial() bool {
	return ref.EvidenceType == evidence.TypeWitnessStatement
}

// IsExhibit reports whether ref's EvidenceType is a documentary or
// physical exhibit.
func (ref EvidenceRef) IsExhibit() bool {
	return ref.EvidenceType == evidence.TypeDocumentaryEvidence || ref.EvidenceType == evidence.TypePhysicalExhibit
}
