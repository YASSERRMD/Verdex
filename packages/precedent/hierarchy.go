package precedent

import "strings"

// CourtLevel classifies a deciding court's position in a common-law
// court hierarchy, for binding-authority weighting (see Weight).
type CourtLevel string

const (
	// CourtSupreme is the highest court in a jurisdiction (e.g. a Supreme
	// Court, House of Lords, or final Court of Appeal). Its decisions
	// carry the strongest binding authority.
	CourtSupreme CourtLevel = "supreme"

	// CourtAppellate is an intermediate appellate court (e.g. a Court of
	// Appeal below the jurisdiction's highest court). Its decisions bind
	// lower courts but are themselves subject to further appeal.
	CourtAppellate CourtLevel = "appellate"

	// CourtTrial is a first-instance/trial-level court. Its decisions
	// carry the weakest binding authority (typically persuasive only
	// outside the immediate case) among the three tiers.
	CourtTrial CourtLevel = "trial"

	// CourtUnknown is used when a precedent's court could not be
	// classified into one of the known tiers. It weighs the lowest of
	// all levels, reflecting the absence of a confirmed hierarchy
	// position rather than an affirmative "trial court" classification.
	CourtUnknown CourtLevel = "unknown"
)

// courtWeights assigns each CourtLevel a binding-authority weight in the
// half-open interval (0, 1], with higher courts weighted higher.
// CourtUnknown is weighted below CourtTrial to reflect the additional
// uncertainty of an unclassified court.
var courtWeights = map[CourtLevel]float64{
	CourtSupreme:   1.0,
	CourtAppellate: 0.7,
	CourtTrial:     0.4,
	CourtUnknown:   0.2,
}

// Weight returns the binding-authority strength of l, in (0, 1], with
// higher courts weighted higher: CourtSupreme > CourtAppellate >
// CourtTrial > CourtUnknown.
func (l CourtLevel) Weight() float64 {
	if w, ok := courtWeights[l]; ok {
		return w
	}
	return courtWeights[CourtUnknown]
}

// IsValid reports whether l is one of the recognized CourtLevel
// constants.
func (l CourtLevel) IsValid() bool {
	_, ok := courtWeights[l]
	return ok
}

// supremeCourtNames lists lowercase substrings recognized as identifying
// a jurisdiction's highest court.
var supremeCourtNames = []string{
	"supreme court",
	"house of lords",
	"privy council",
	"constitutional court",
	"federal supreme court",
	"final court of appeal",
}

// appellateCourtNames lists lowercase substrings recognized as
// identifying an intermediate appellate court.
var appellateCourtNames = []string{
	"court of appeal",
	"appellate",
	"circuit court of appeals",
	"court of appeals",
}

// trialCourtNames lists lowercase substrings recognized as identifying a
// first-instance/trial-level court.
var trialCourtNames = []string{
	"district court",
	"high court",
	"trial court",
	"magistrate",
	"sessions court",
	"crown court",
	"first instance",
}

// ClassifyCourtLevel is a deterministic heuristic that maps a free-text
// court name (as loaded into RawPrecedent.Court) to a CourtLevel, by
// case-insensitive substring matching against known court-name fragments.
// Returns CourtUnknown when courtName does not match any recognized
// fragment (including when courtName is blank).
func ClassifyCourtLevel(courtName string) CourtLevel {
	lower := strings.ToLower(strings.TrimSpace(courtName))
	if lower == "" {
		return CourtUnknown
	}
	for _, name := range supremeCourtNames {
		if strings.Contains(lower, name) {
			return CourtSupreme
		}
	}
	for _, name := range appellateCourtNames {
		if strings.Contains(lower, name) {
			return CourtAppellate
		}
	}
	for _, name := range trialCourtNames {
		if strings.Contains(lower, name) {
			return CourtTrial
		}
	}
	return CourtUnknown
}

// HierarchyRule bundles a TaggedPrecedent with its classified CourtLevel,
// so downstream stages (embed.go, authority.go, persist.go) can read the
// hierarchy weighting without re-deriving it.
type HierarchyRule struct {
	TaggedPrecedent

	// CourtLevel is this precedent's classified position in the
	// common-law court hierarchy.
	CourtLevel CourtLevel
}

// ApplyCourtHierarchy classifies every rule's Source.Court via
// ClassifyCourtLevel (or uses overrideLevel when non-empty, applied
// uniformly to every rule) and returns the resulting HierarchyRules.
//
// overrideLevel is useful for corpora where every precedent is already
// known to come from a single court tier (e.g. an appellate-only corpus)
// and per-rule classification is unnecessary; pass "" to classify each
// rule independently from its Source.Court.
func ApplyCourtHierarchy(rules []TaggedPrecedent, overrideLevel CourtLevel) []HierarchyRule {
	out := make([]HierarchyRule, 0, len(rules))
	for _, r := range rules {
		level := overrideLevel
		if level == "" {
			level = ClassifyCourtLevel(r.Source.Court)
		}
		out = append(out, HierarchyRule{
			TaggedPrecedent: r,
			CourtLevel:      level,
		})
	}
	return out
}
