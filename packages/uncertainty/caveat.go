package uncertainty

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

// lowConfidenceCaveat generates the human-readable Caveat text for a
// low-confidence finding at the given source, wording it in terms a
// reviewing judge understands rather than exposing the raw Source enum
// value.
func lowConfidenceCaveat(source Source, confidence float64) string {
	switch source {
	case SourceIssueFraming:
		return fmt.Sprintf("This issue was framed with low confidence (%.2f); the governing question or its rule linkage may be unclear.", confidence)
	case SourceLawApplication:
		return fmt.Sprintf("The application of law to this issue carries low confidence (%.2f); the controlling rule selection may be weak.", confidence)
	case SourceConclusion:
		return fmt.Sprintf("This conclusion was reached with low confidence (%.2f) in the underlying synthesis.", confidence)
	case SourceEvidence:
		return fmt.Sprintf("This finding carries low confidence (%.2f).", confidence)
	default:
		return fmt.Sprintf("This finding carries low confidence (%.2f).", confidence)
	}
}

// thinEvidenceCaveat generates the Caveat text for a single thin or
// disputed FactWeight.
func thinEvidenceCaveat(fw evidenceweighing.FactWeight) string {
	if fw.Contradicted {
		return "This conclusion relies on a fact contradicted by opposing evidence."
	}
	return fmt.Sprintf("This conclusion relies on a thinly supported fact (evidentiary weight %.2f).", fw.Weight)
}

// contradictionCaveat generates the Caveat text for an
// evidenceweighing.Contradiction.
func contradictionCaveat(c evidenceweighing.Contradiction) string {
	return fmt.Sprintf(
		"A fact central to this issue (%s) is cited by both parties in support of mutually exclusive claims.",
		c.FactNodeID,
	)
}

// gapCaveat generates the Caveat text for an evidenceweighing.Gap,
// wording it differently depending on GapKind.
func gapCaveat(g evidenceweighing.Gap) string {
	switch g.Kind {
	case evidenceweighing.GapKindUncitedIssue:
		return "This issue is argued without any party citing supporting evidence for it."
	case evidenceweighing.GapKindMissingFact:
		return "An argument for this issue cites a fact that could not be found in the case record."
	default:
		if g.Description != "" {
			return g.Description
		}
		return "A gap was found in the evidentiary record for this issue."
	}
}

// conflictingAuthorityCaveat generates the Caveat text for a
// lawapplication.ConflictingAuthority.
func conflictingAuthorityCaveat(c lawapplication.ConflictingAuthority) string {
	return fmt.Sprintf(
		"The controlling authority for this issue is contested between two conflicting rules (%s and %s), invoked by opposing parties.",
		c.FirstRuleID, c.SecondRuleID,
	)
}
