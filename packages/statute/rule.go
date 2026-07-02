package statute

import (
	"fmt"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// Citation identifies exactly where a rule's text was drawn from within
// an act's hierarchy: the act, the section, and (when the rule was built
// at clause granularity) the clause.
type Citation struct {
	// Act is the act number the rule derives from (StatuteNode.Number at
	// LevelAct).
	Act string `json:"act"`

	// Section is the section number the rule derives from
	// (StatuteNode.Number at LevelSection). Empty if the rule was built
	// directly from an act with no sections.
	Section string `json:"section,omitempty"`

	// Clause is the clause identifier the rule derives from
	// (StatuteNode.Number at LevelClause). Empty unless the rule was
	// built at clause granularity.
	Clause string `json:"clause,omitempty"`
}

// String formats the citation as "Act <act>, s.<section>(<clause>)",
// omitting the section/clause segments that are empty. A bare act
// citation with no section renders as "Act <act>".
func (c Citation) String() string {
	var b strings.Builder
	b.WriteString("Act ")
	b.WriteString(c.Act)
	if c.Section == "" {
		return b.String()
	}
	fmt.Fprintf(&b, ", s.%s", c.Section)
	if c.Clause != "" {
		fmt.Fprintf(&b, "(%s)", c.Clause)
	}
	return b.String()
}

// RuleGranularity controls which tier of a StatuteNode tree
// BuildRuleNodes converts into individual irac.RuleNodes.
type RuleGranularity string

const (
	// GranularitySection builds one rule per Section node, using the
	// section's own text (its clauses' text is not separately emitted).
	// If a section has clause children but no text of its own, the
	// clauses' text is concatenated to form the rule's text so the
	// section-level rule is never empty.
	GranularitySection RuleGranularity = "section"

	// GranularityClause builds one rule per leaf node (a Clause when
	// clauses exist, otherwise the Section itself). This is the default
	// used by BuildRuleNodes when Granularity is left unset.
	GranularityClause RuleGranularity = "clause"
)

// RuleBuildOptions configures BuildRuleNodes.
type RuleBuildOptions struct {
	// Granularity selects which StatuteNode tier becomes one
	// irac.RuleNode. Defaults to GranularityClause.
	Granularity RuleGranularity

	// CaseID is stamped on every produced irac.RuleNode. Statute rules
	// are not case-scoped the way reasoning-tree nodes are, so callers
	// conventionally pass a corpus-scoped pseudo case id such as
	// "statute:<jurisdiction-code>" (see Citation and
	// StatuteIngestionService for the convention this package follows).
	// Required.
	CaseID string

	// JurisdictionCode is stamped on every produced irac.RuleNode's
	// JurisdictionCode field directly (tagging.go may overwrite/refine
	// this per-rule later in the pipeline).
	JurisdictionCode string

	// LegalFamily is stamped on every produced irac.RuleNode's
	// LegalFamily field directly.
	LegalFamily string

	// IDPrefix prefixes every generated irac.RuleNode ID. If empty,
	// "rule" is used.
	IDPrefix string

	// GeneratedBy stamps irac.Provenance.GeneratedBy on every produced
	// rule. If empty, "statute-rule-builder-v1" is used.
	GeneratedBy string

	// CreatedAt stamps every produced rule's CreatedAt and
	// Provenance.GeneratedAt. If zero, time.Now() is used.
	CreatedAt time.Time

	// Confidence is stamped on every produced rule. Statute text is
	// authoritative source material rather than an extraction, so a
	// zero value here (the default for callers that never set it) is
	// treated by BuildRuleNodes as "use full confidence" (1.0). Callers
	// that genuinely want a zero confidence should pass a tiny non-zero
	// placeholder instead, since irac.ValidConfidence treats 0 as a
	// legal but unusual value for a rule sourced directly from statute
	// text.
	Confidence float64
}

// BuiltRule bundles a single irac.RuleNode with the Citation and source
// StatuteNode it was built from, so downstream pipeline stages
// (tagging.go, amendment.go, xref.go, embed.go, persist.go) can carry
// that context forward without re-deriving it.
type BuiltRule struct {
	Node     irac.RuleNode
	Citation Citation
	Source   *StatuteNode
}

// BuildRuleNodes converts act (the root StatuteNode returned by
// ParseHierarchy) into one irac.RuleNode per node at the configured
// Granularity, each carrying a formatted Citation back to its position
// in the act's hierarchy.
//
// Returns ErrEmptyInput if act is nil or opts.CaseID is blank.
func BuildRuleNodes(act *StatuteNode, opts RuleBuildOptions) ([]BuiltRule, error) {
	if act == nil {
		return nil, ErrEmptyInput
	}
	if strings.TrimSpace(opts.CaseID) == "" {
		return nil, ErrEmptyInput
	}

	granularity := opts.Granularity
	if granularity == "" {
		granularity = GranularityClause
	}
	prefix := opts.IDPrefix
	if prefix == "" {
		prefix = "rule"
	}
	generatedBy := opts.GeneratedBy
	if generatedBy == "" {
		generatedBy = "statute-rule-builder-v1"
	}
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	confidence := opts.Confidence
	if confidence == 0 {
		confidence = 1.0
	}

	var targets []targetNode
	switch granularity {
	case GranularitySection:
		for _, s := range act.Children {
			targets = append(targets, targetNode{section: s, node: s})
		}
		if len(act.Children) == 0 {
			targets = append(targets, targetNode{node: act})
		}
	default: // GranularityClause
		for _, s := range act.Children {
			if s.IsLeaf() {
				targets = append(targets, targetNode{section: s, node: s})
				continue
			}
			for _, c := range s.Children {
				targets = append(targets, targetNode{section: s, clause: c, node: c})
			}
		}
		if len(act.Children) == 0 {
			targets = append(targets, targetNode{node: act})
		}
	}

	built := make([]BuiltRule, 0, len(targets))
	for i, t := range targets {
		text := ruleText(t.node, granularity)
		citation := Citation{Act: act.Number}
		if t.section != nil {
			citation.Section = t.section.Number
		}
		if t.clause != nil {
			citation.Clause = t.clause.Number
		}

		id := fmt.Sprintf("%s-%d", prefix, i)
		provenance := irac.Provenance{
			GeneratedBy: generatedBy,
			GeneratedAt: createdAt,
		}
		node := irac.NewRuleNode(id, opts.CaseID, text, opts.JurisdictionCode, opts.LegalFamily, createdAt, confidence, provenance)

		built = append(built, BuiltRule{
			Node:     node,
			Citation: citation,
			Source:   t.node,
		})
	}
	return built, nil
}

// targetNode bundles the node BuildRuleNodes is about to convert into a
// rule with its enclosing section/clause context (for Citation
// derivation).
type targetNode struct {
	section *StatuteNode
	clause  *StatuteNode
	node    *StatuteNode
}

// ruleText returns the text to use for a rule built from node. For
// section-granularity rules with no text of their own but with clause
// children, the clauses' text is concatenated so the rule is never
// built with empty text.
func ruleText(node *StatuteNode, granularity RuleGranularity) string {
	if node.Text != "" {
		return node.Text
	}
	if granularity != GranularitySection || len(node.Children) == 0 {
		return node.Text
	}
	parts := make([]string, 0, len(node.Children))
	for _, c := range node.Children {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, " ")
}
