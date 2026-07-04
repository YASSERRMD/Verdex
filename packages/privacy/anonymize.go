package privacy

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/pii"
)

// dataCategoryToPIICategory maps a DataCategory whose content is
// fundamentally free text (case content, transcripts, and similar) to
// the packages/pii.PIICategory AnonymizeForAnalytics should treat it
// as when no more specific per-span Category was already assigned by
// detection. This is deliberately a narrow, best-effort mapping, not a
// claim that every DataCategory has a PIICategory equivalent --
// CategoryBehavioral and CategoryOther fall back to pii.CategoryOther,
// which is exactly how packages/pii itself classifies anything it
// cannot place more specifically.
var dataCategoryToPIICategory = map[DataCategory]pii.PIICategory{
	CategoryIdentity:   pii.CategoryName,
	CategoryCaseParty:  pii.CategoryName,
	CategoryContact:    pii.CategoryContact,
	CategoryIdentifier: pii.CategoryIdentifier,
	CategoryFinancial:  pii.CategoryFinancial,
	CategoryTranscript: pii.CategoryOther,
	CategoryBehavioral: pii.CategoryOther,
	CategoryOther:      pii.CategoryOther,
}

// AnalyticsRecord is a single free-text field submitted to
// AnonymizeForAnalytics, identified by FieldName so the caller can
// reassemble AnonymizedRecord.Fields against the original record
// shape.
type AnalyticsRecord struct {
	// FieldName identifies which field of the source record Text came
	// from (e.g. "party_name", "transcript_segment").
	FieldName string

	// Text is the raw field content that may contain personal data.
	Text string
}

// AnonymizedField is one field of the anonymized projection
// AnonymizeForAnalytics produces.
type AnonymizedField struct {
	// FieldName mirrors the originating AnalyticsRecord.FieldName.
	FieldName string

	// Text is Text with every detected PII span redacted according to
	// mode.
	Text string

	// MatchCount is how many PII spans were detected (and redacted) in
	// this field, reported so an analytics consumer can distinguish "no
	// personal data was present" from "personal data was present and
	// removed" without inspecting Text itself.
	MatchCount int
}

// AnonymizedRecord is the aggregated/pseudonymized projection task 8
// asks for: a record suitable for analytics use, with every field's
// personal-data content redacted or pseudonymized via
// packages/pii.Redactor rather than this package reimplementing
// NER/detection.
type AnonymizedRecord struct {
	// SubjectID identifies the data subject the source record
	// concerned. Left as-is (not itself redacted) since an analytics
	// pipeline keyed by subject needs a stable join key; if the
	// analytics use case requires subject-anonymity too, the caller
	// should hash/tokenize SubjectID separately before storing
	// AnonymizedRecord -- that is outside packages/pii's redaction
	// concern (text spans within a document), so it is not folded into
	// this function.
	SubjectID string

	// Category is the DataCategory the source record was classified
	// under.
	Category DataCategory

	// Fields holds one AnonymizedField per input AnalyticsRecord, in
	// the same order supplied.
	Fields []AnonymizedField

	// TotalMatches is the sum of every field's MatchCount.
	TotalMatches int

	// AnonymizedAt is when this projection was produced.
	AnonymizedAt time.Time
}

// AnonymizeForAnalytics detects and redacts personal data within
// records using packages/pii's existing detection and redaction
// pipeline (task 8): pii.NewRuleBasedDetector for detection,
// pii.ClassifyMatches to categorize each span, and a pii.Redactor
// (mode defaults to pii.ModeIrreversibleRedact when unset, since an
// analytics projection has no legitimate need to ever reverse a
// redaction back to the original value) to apply the transform. This
// package does not reimplement detection or redaction logic -- every
// span-level decision is delegated to packages/pii.
func AnonymizeForAnalytics(ctx context.Context, subjectID string, category DataCategory, records []AnalyticsRecord, mode pii.RedactionMode) (AnonymizedRecord, error) {
	if mode == "" {
		mode = pii.ModeIrreversibleRedact
	}

	detector := pii.NewRuleBasedDetector()
	var pseudonyms *pii.PseudonymMap
	if mode == pii.ModePseudonymize {
		pseudonyms = pii.NewPseudonymMap(pii.DenyAllAccessPolicy{})
	}
	redactor := pii.NewRedactor(mode, pseudonyms)

	out := AnonymizedRecord{
		SubjectID:    subjectID,
		Category:     category,
		Fields:       make([]AnonymizedField, 0, len(records)),
		AnonymizedAt: time.Now().UTC(),
	}

	for _, rec := range records {
		matches, err := detector.Detect(ctx, rec.Text)
		if err != nil {
			return AnonymizedRecord{}, wrapf("AnonymizeForAnalytics", err)
		}
		matches = pii.ClassifyMatches(matches)
		for i := range matches {
			if matches[i].Category == "" {
				if cat, ok := dataCategoryToPIICategory[category]; ok {
					matches[i].Category = cat
				}
			}
		}

		result, err := redactor.Redact(rec.Text, matches)
		if err != nil {
			return AnonymizedRecord{}, wrapf("AnonymizeForAnalytics", err)
		}

		field := AnonymizedField{
			FieldName:  rec.FieldName,
			Text:       result.Text,
			MatchCount: len(matches),
		}
		out.Fields = append(out.Fields, field)
		out.TotalMatches += field.MatchCount
	}

	return out, nil
}
