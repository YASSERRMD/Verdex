// Package evidence classifies packages/segmentation's Segments by
// evidentiary role — witness testimony, documentary evidence, statutory
// citation, argument, physical exhibit, or other — and attributes each to
// a party (first, second, or unattributed), so downstream IRAC
// (Issue/Rule/Application/Conclusion) reasoning can distinguish what kind
// of evidence it is looking at and whose evidence it is.
//
// Core concepts:
//
//   - EvidenceType / Describe / AllEvidenceTypes: the evidence-type
//     taxonomy — TypeWitnessStatement, TypeDocumentaryEvidence,
//     TypeStatutoryCitation, TypeArgument, TypePhysicalExhibit, TypeOther —
//     with a short description per type (taxonomy.go).
//   - Classification / Classifier / RuleBasedClassifier: the pluggable
//     classification interface and its default, deterministic
//     implementation, which combines segmentation.SegmentType with lexical
//     heuristics to assign an EvidenceType and PartyRole (classifier.go).
//   - IsWitnessStatement: heuristics for first-person testimonial language
//     and speaker-attributed segmentation.SegmentStatement segments
//     (witness.go).
//   - IsDocumentaryEvidence / IsPhysicalExhibit: heuristics for document
//     and exhibit references, including segmentation.SegmentExhibit
//     segments (documentary.go).
//   - IsStatutoryCitation: heuristics for statute/section citation
//     patterns, including segmentation.SegmentCitation segments
//     (statute_citation.go).
//   - PartyRole / AttributeParty: first/second-party attribution based on
//     speaker label or explicit textual markers (party.go).
//   - ManualOverride / ApplyOverride: human-correction support that lets a
//     reviewer's judgment take precedence over the classifier, with the
//     original determination preserved distinctly on
//     Classification.Override.Previous rather than being discarded
//     (override.go).
//   - ClassificationStore / InMemoryClassificationStore: the persistence
//     contract for Classification records, keyed by segment ID, with an
//     in-memory implementation requiring no real database dependency
//     (store.go).
//   - EvidenceService: orchestrates the full pipeline — classify -> detect
//     witness/documentary/statutory subtype -> attribute party -> apply any
//     override -> persist -> return []Classification (service.go).
//
// Design principles:
//
//   - No ML models. Every heuristic in this package is a deterministic
//     function of segmentation.SegmentType and regular-expression/lexical
//     pattern matching — mirroring packages/segmentation and packages/pii's
//     "no ML models, rule based" design principle. A future phase can swap
//     in a real classifier model by implementing the Classifier interface;
//     no caller needs to change.
//   - Confidence, not certainty. Every Classification carries a Confidence
//     score in the closed interval [0, 1], reflecting how specific the
//     matched signal was (a structurally boundary-tagged
//     segmentation.SegmentExhibit/SegmentCitation scores higher than a bare
//     lexical match on an untagged paragraph).
//   - Human correction is first-class, not a patch. ManualOverride and
//     ApplyOverride are dedicated types, not a mutation of Classification
//     in place — the classifier's original determination is always
//     recoverable from Classification.Override.Previous.
//
// See doc/evidence-taxonomy.md for a detailed taxonomy write-up.
package evidence
