package evidence

import (
	"context"
	"strings"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// Classification is the output of classifying a single segmentation.Segment:
// its evidentiary type, the party attributed to it (see party.go), and a
// confidence score, with room for a human ManualOverride to take precedence
// over the classifier's own determination (see override.go).
type Classification struct {
	// SegmentID identifies the segmentation.Segment this classification
	// describes.
	SegmentID string

	// Type is the evidentiary type assigned to the segment.
	Type EvidenceType

	// Party is the party attributed to the segment, when attribution
	// heuristics found one (see party.go).
	Party PartyRole

	// Confidence is this classification's confidence score, in the closed
	// interval [0, 1]. See override.go for confidence-bounds helpers.
	Confidence float64

	// Override, when non-nil, records a human correction that takes
	// precedence over the classifier's own Type/Party/Confidence — see
	// override.go. The classifier's original determination is preserved
	// distinctly on Override.Previous rather than being overwritten.
	Override *ManualOverride
}

// Classifier assigns an evidentiary Classification to a single
// segmentation.Segment.
//
// This interface is the pluggable extension point for evidence
// classification: the default implementation in this file
// (RuleBasedClassifier) is a deterministic function of segment type and
// text patterns, mirroring packages/pii's Detector and
// packages/segmentation's "no ML models, rule based" design principle. A
// future phase can swap in a real classifier model by implementing this
// same interface — no caller of Classifier needs to change.
type Classifier interface {
	// Classify inspects seg and returns its Classification. ctx allows
	// implementations that call out to an external model or service to
	// respect cancellation/deadlines.
	//
	// Returns ErrEmptyInput if seg.Text is empty or whitespace-only.
	Classify(ctx context.Context, seg segmentation.Segment) (Classification, error)
}

// RuleBasedClassifier is the default, deterministic Classifier
// implementation. It combines segmentation.SegmentType with lexical
// heuristics (see witness.go, documentary.go, statute_citation.go) to
// assign an EvidenceType, then attributes a party (see party.go).
//
// RuleBasedClassifier performs no machine learning and calls out to no
// external service, so its output is fully reproducible given the same
// input segment.
type RuleBasedClassifier struct{}

// NewRuleBasedClassifier constructs a RuleBasedClassifier. It has no
// configuration today; the constructor exists so call sites can be updated
// uniformly if configuration is added later.
func NewRuleBasedClassifier() *RuleBasedClassifier {
	return &RuleBasedClassifier{}
}

// Classify implements Classifier. It applies, in order, the
// statutory-citation, witness-statement, and documentary-evidence
// heuristics (each scoped to the segment.Type it is most specific to, plus
// a text-pattern fallback for paragraph segments), falling back to
// TypeArgument for opinion/contention language and TypeOther when nothing
// matches.
func (c *RuleBasedClassifier) Classify(_ context.Context, seg segmentation.Segment) (Classification, error) {
	if strings.TrimSpace(seg.Text) == "" {
		return Classification{}, ErrEmptyInput
	}

	evType, confidence := classifyType(seg)
	party := AttributeParty(seg)

	return Classification{
		SegmentID:  seg.ID,
		Type:       evType,
		Party:      party,
		Confidence: confidence,
	}, nil
}

// classifyType determines the EvidenceType and confidence for seg by
// checking each subtype heuristic in priority order: statutory citation,
// witness statement, documentary evidence, physical exhibit, then a
// lexical argument fallback, defaulting to TypeOther.
//
// Statutory citation is checked first because citation shapes (e.g.
// "Section 302 IPC") are highly specific lexical patterns unlikely to
// co-occur ambiguously with the looser witness/documentary heuristics.
func classifyType(seg segmentation.Segment) (EvidenceType, float64) {
	if ok, conf := IsStatutoryCitation(seg); ok {
		return TypeStatutoryCitation, conf
	}
	if ok, conf := IsWitnessStatement(seg); ok {
		return TypeWitnessStatement, conf
	}
	if ok, conf := IsPhysicalExhibit(seg); ok {
		return TypePhysicalExhibit, conf
	}
	if ok, conf := IsDocumentaryEvidence(seg); ok {
		return TypeDocumentaryEvidence, conf
	}
	if isArgumentLanguage(seg.Text) {
		return TypeArgument, 0.55
	}
	return TypeOther, 0.3
}

// argumentMarkers are lexical cues that a segment is advocacy/reasoning
// text rather than testimony, a document reference, or a citation.
var argumentMarkers = []string{
	"we submit", "it is submitted", "the defense contends", "the prosecution contends",
	"we contend", "counsel argues", "it is argued", "the plaintiff argues",
	"the defendant argues", "we respectfully submit", "therefore, it follows",
	"the appellant submits", "the respondent submits",
}

// isArgumentLanguage reports whether text contains a lexical marker
// typical of advocacy/reasoning language.
func isArgumentLanguage(text string) bool {
	lower := strings.ToLower(text)
	for _, m := range argumentMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
