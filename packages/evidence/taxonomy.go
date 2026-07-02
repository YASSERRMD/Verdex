package evidence

// EvidenceType classifies the evidentiary role a segment of case text plays,
// so downstream IRAC (Issue/Rule/Application/Conclusion) reasoning can
// distinguish testimony from documentary proof, statutory authority, and
// argument, rather than treating every segment as undifferentiated text.
//
// This mirrors packages/pii's PIICategory and packages/segmentation's
// SegmentType conventions: a small string-backed enum with one constant per
// recognized kind, plus an "other" catch-all for text that does not fit a
// more specific type.
type EvidenceType string

const (
	// TypeWitnessStatement covers first-person testimonial language: sworn
	// statements, depositions, and speaker-attributed segments recounting
	// what a witness observed, said, or did (see witness.go).
	TypeWitnessStatement EvidenceType = "witness_statement"

	// TypeDocumentaryEvidence covers references to documents, exhibits, and
	// other tangible records introduced as proof (see documentary.go).
	TypeDocumentaryEvidence EvidenceType = "documentary_evidence"

	// TypeStatutoryCitation covers statute, code, and case-law citations
	// invoked as legal authority (see statute_citation.go).
	TypeStatutoryCitation EvidenceType = "statutory_citation"

	// TypeArgument covers advocacy or reasoning text — a party's or
	// counsel's contention, submission, or inference — that is neither
	// testimony, a document reference, nor a bare citation.
	TypeArgument EvidenceType = "argument"

	// TypePhysicalExhibit covers references to tangible, non-documentary
	// physical evidence (e.g. a weapon, a garment, a sample) as distinct
	// from paper/documentary exhibits.
	TypePhysicalExhibit EvidenceType = "physical_exhibit"

	// TypeOther covers segments that do not fit any of the above categories
	// (e.g. procedural headings, filler text).
	TypeOther EvidenceType = "other"
)

// evidenceTypeDescriptions holds a short human-readable description for
// each recognized EvidenceType, used by Describe and by
// doc/evidence-taxonomy.md as the source of truth for the taxonomy.
var evidenceTypeDescriptions = map[EvidenceType]string{
	TypeWitnessStatement:    "First-person testimonial language: sworn statements, depositions, and speaker-attributed accounts of what a witness observed, said, or did.",
	TypeDocumentaryEvidence: "References to documents, exhibits, or records introduced as proof (contracts, letters, reports, Exhibit/Annexure/Schedule markers).",
	TypeStatutoryCitation:   "Statute, code section, or case-law citations invoked as legal authority (e.g. \"Section 302 IPC\", \"42 U.S.C. § 1983\", \"Smith v. Jones\").",
	TypeArgument:            "Advocacy or reasoning text: a party's or counsel's contention, submission, or inference that is not itself testimony, a document, or a bare citation.",
	TypePhysicalExhibit:     "References to tangible, non-documentary physical evidence (e.g. a weapon, a garment, a sample) as distinct from paper/documentary exhibits.",
	TypeOther:               "Text that does not fit a more specific evidence type (e.g. procedural headings, filler text).",
}

// Describe returns the short human-readable description registered for t.
// Returns the empty string for an unrecognized EvidenceType value.
func Describe(t EvidenceType) string {
	return evidenceTypeDescriptions[t]
}

// AllEvidenceTypes returns every recognized EvidenceType, in the fixed
// order they are declared above (TypeWitnessStatement first, TypeOther
// last). Useful for tests and for building documentation/UI enumerations
// from a single source of truth.
func AllEvidenceTypes() []EvidenceType {
	return []EvidenceType{
		TypeWitnessStatement,
		TypeDocumentaryEvidence,
		TypeStatutoryCitation,
		TypeArgument,
		TypePhysicalExhibit,
		TypeOther,
	}
}
