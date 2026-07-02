package precedent

import (
	"fmt"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// PrecedentRule wraps an irac.RuleNode (see packages/irac/node.go's
// RuleNode doc comment: "a legal rule, statute, or precedent invoked to
// resolve an Issue" — there is no separate PrecedentNode type in the fixed
// IRAC schema) with precedent-specific metadata that RuleNode itself does
// not carry: the extracted Holding and RatioDecidendi (holding.go), and a
// formatted case Citation.
//
// This mirrors packages/statute's BuiltRule/Citation convention: the
// schema-owning package (packages/irac) is never modified, and
// domain-specific enrichment lives in a local sibling struct that embeds
// the RuleNode.
type PrecedentRule struct {
	// RuleNode is the underlying IRAC rule node built via
	// irac.NewRuleNode. Embedded so callers can use a PrecedentRule
	// anywhere an irac.RuleNode (or irac.NodeLike) is expected via
	// promoted fields/methods.
	irac.RuleNode

	// Holding is the court's core determination, as extracted by
	// ExtractHoldingAndRatio (or a caller-supplied ExtractorFunc).
	Holding string `json:"holding"`

	// RatioDecidendi is the reasoning behind the Holding.
	RatioDecidendi string `json:"ratio_decidendi"`

	// Citation is this precedent's formatted case citation (e.g.
	// "Donoghue v Stevenson [1932] AC 562").
	Citation string `json:"citation"`

	// Source is the RawPrecedent this rule was built from, retained so
	// downstream pipeline stages (tagging.go, hierarchy.go, authority.go)
	// can read fields (Court, DecidedDate) not carried on irac.RuleNode
	// without re-deriving them.
	Source RawPrecedent `json:"-"`
}

// FormatCitation formats a precedent's case name and raw citation into a
// single human-readable citation string, e.g.
// "Donoghue v Stevenson [1932] AC 562". If either component is blank, the
// other is returned alone (trimmed); if both are blank, the empty string
// is returned.
func FormatCitation(caseName, rawCitation string) string {
	caseName = strings.TrimSpace(caseName)
	rawCitation = strings.TrimSpace(rawCitation)
	switch {
	case caseName == "" && rawCitation == "":
		return ""
	case caseName == "":
		return rawCitation
	case rawCitation == "":
		return caseName
	default:
		return fmt.Sprintf("%s %s", caseName, rawCitation)
	}
}

// RuleBuildOptions configures BuildPrecedentRule/BuildPrecedentRules.
type RuleBuildOptions struct {
	// CaseID is stamped on every produced irac.RuleNode. Precedent rules
	// are not case-scoped the way reasoning-tree nodes are, so callers
	// conventionally pass a corpus-scoped pseudo case id such as
	// "precedent:<jurisdiction-code>", mirroring packages/statute's own
	// "statute:<jurisdiction-code>" convention. Required.
	CaseID string

	// JurisdictionCode is stamped on every produced irac.RuleNode's
	// JurisdictionCode field directly (tagging.go may overwrite/refine
	// this per-rule later in the pipeline).
	JurisdictionCode string

	// LegalFamily is stamped on every produced irac.RuleNode's
	// LegalFamily field directly.
	LegalFamily string

	// IDPrefix prefixes every generated irac.RuleNode ID. If empty,
	// "precedent" is used.
	IDPrefix string

	// GeneratedBy stamps irac.Provenance.GeneratedBy on every produced
	// rule. If empty, "precedent-rule-builder-v1" is used.
	GeneratedBy string

	// CreatedAt stamps every produced rule's CreatedAt and
	// Provenance.GeneratedAt. If zero, time.Now() is used.
	CreatedAt time.Time

	// Confidence is stamped on every produced rule. A precedent's holding
	// text is drawn from an authoritative judgment rather than a
	// heuristic extraction over unstructured evidence, so a zero value
	// here (the default for callers that never set it) is treated as
	// "use full confidence" (1.0), mirroring packages/statute's
	// RuleBuildOptions.Confidence convention.
	Confidence float64

	// Extractor is the holding/ratio extraction function used when a
	// RawPrecedent is built into a PrecedentRule. If nil,
	// ExtractHoldingAndRatio is used.
	Extractor ExtractorFunc
}

// BuildPrecedentRule converts a single RawPrecedent into a PrecedentRule:
// it extracts the holding/ratio (via opts.Extractor, defaulting to
// ExtractHoldingAndRatio), formats the citation, and constructs the
// underlying irac.RuleNode via irac.NewRuleNode using the extracted
// Holding+RatioDecidendi as the rule's Text (falling back to FullText when
// extraction finds nothing, so a rule is never built with empty text).
//
// id is the caller-supplied irac.RuleNode ID (see BuildPrecedentRules for
// the batch convenience that generates IDs automatically).
//
// Returns ErrEmptyInput if opts.CaseID is blank.
func BuildPrecedentRule(id string, raw RawPrecedent, opts RuleBuildOptions) (PrecedentRule, error) {
	if strings.TrimSpace(opts.CaseID) == "" {
		return PrecedentRule{}, ErrEmptyInput
	}

	extractor := opts.Extractor
	if extractor == nil {
		extractor = ExtractHoldingAndRatio
	}
	generatedBy := opts.GeneratedBy
	if generatedBy == "" {
		generatedBy = "precedent-rule-builder-v1"
	}
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	confidence := opts.Confidence
	if confidence == 0 {
		confidence = 1.0
	}

	result, extractErr := extractor(raw.FullText)

	text := strings.TrimSpace(result.Holding + " " + result.RatioDecidendi)
	if text == "" {
		text = strings.TrimSpace(raw.FullText)
	}

	provenance := irac.Provenance{
		GeneratedBy: generatedBy,
		GeneratedAt: createdAt,
	}
	node := irac.NewRuleNode(id, opts.CaseID, text, opts.JurisdictionCode, opts.LegalFamily, createdAt, confidence, provenance)

	rule := PrecedentRule{
		RuleNode:       node,
		Holding:        result.Holding,
		RatioDecidendi: result.RatioDecidendi,
		Citation:       FormatCitation(raw.CaseName, raw.Citation),
		Source:         raw,
	}

	if extractErr != nil {
		return rule, extractErr
	}
	return rule, nil
}

// BuildPrecedentRules converts every RawPrecedent in raws into a
// PrecedentRule via BuildPrecedentRule, auto-generating each rule's
// irac.RuleNode ID as "<prefix>-<index>" (prefix defaults to "precedent").
//
// Extraction failures (ErrHoldingNotFound) for individual precedents do
// not abort the batch: the corresponding PrecedentRule is still included,
// built from raw.FullText directly (see BuildPrecedentRule), and its ID is
// collected in the returned failedIDs slice so callers can inspect which
// precedents lacked a recognizable holding marker.
//
// Returns ErrEmptyInput if opts.CaseID is blank.
func BuildPrecedentRules(raws []RawPrecedent, opts RuleBuildOptions) (rules []PrecedentRule, failedIDs []string, err error) {
	if strings.TrimSpace(opts.CaseID) == "" {
		return nil, nil, ErrEmptyInput
	}

	prefix := opts.IDPrefix
	if prefix == "" {
		prefix = "precedent"
	}

	rules = make([]PrecedentRule, 0, len(raws))
	for i, raw := range raws {
		id := fmt.Sprintf("%s-%d", prefix, i)
		rule, buildErr := BuildPrecedentRule(id, raw, RuleBuildOptions{
			CaseID:           opts.CaseID,
			JurisdictionCode: opts.JurisdictionCode,
			LegalFamily:      opts.LegalFamily,
			GeneratedBy:      opts.GeneratedBy,
			CreatedAt:        opts.CreatedAt,
			Confidence:       opts.Confidence,
			Extractor:        opts.Extractor,
		})
		rules = append(rules, rule)
		if buildErr != nil {
			failedIDs = append(failedIDs, id)
		}
	}
	return rules, failedIDs, nil
}
