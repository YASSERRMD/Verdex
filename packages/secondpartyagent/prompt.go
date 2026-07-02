package secondpartyagent

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent/templates"
)

// resolveTemplate selects the best argument-rebuttal prompt template for
// the requested locale and legal family, via
// prompts.VariantSelector{}.SelectBest's tiered fallback (exact ->
// locale-only -> family-only -> universal), mirroring
// packages/firstpartyagent/prompt.go's resolveTemplate exactly.
//
// Returns ErrNoTemplate wrapping the underlying lookup failure if no
// variant is registered at any fallback tier.
func resolveTemplate(registry *prompts.Registry, locale, legalFamily string) (*prompts.PromptTemplate, error) {
	tmpl, err := (prompts.VariantSelector{}).SelectBest(registry, templates.ArgumentRebuttalTemplateID, locale, legalFamily)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoTemplate, err)
	}
	return tmpl, nil
}

// renderIssuesBlock renders every issueEvidence into the plain-text block
// the argument-rebuttal template's issues_block variable expects: one
// numbered entry per issue, its governing question(s), and the exact,
// exhaustive set of fact/rule IDs the model is permitted to cite as
// evidence for it. Mirrors
// packages/firstpartyagent/prompt.go's renderIssuesBlock exactly.
func renderIssuesBlock(evidence []issueEvidence) string {
	var b strings.Builder
	for i, ev := range evidence {
		fmt.Fprintf(&b, "%d. ISSUE [%s]\n", i+1, ev.Issue.SourceIssueNodeID)
		if len(ev.Issue.GoverningQuestions) == 0 {
			fmt.Fprintf(&b, "   question: %s\n", ev.Issue.Question)
		} else {
			b.WriteString("   governing questions:\n")
			for _, q := range ev.Issue.GoverningQuestions {
				fmt.Fprintf(&b, "     - %s\n", q)
			}
		}

		if len(ev.Facts) == 0 {
			b.WriteString("   available facts: none\n")
		} else {
			b.WriteString("   available facts:\n")
			for _, f := range ev.Facts {
				fmt.Fprintf(&b, "     - [%s] %s (confidence=%.2f)\n", f.ID, f.Text, f.Confidence)
			}
		}

		if len(ev.Rules) == 0 {
			b.WriteString("   available rules: none\n")
		} else {
			b.WriteString("   available rules:\n")
			for _, r := range ev.Rules {
				fmt.Fprintf(&b, "     - [%s] %s (confidence=%.2f)\n", r.ID, r.Text, r.Confidence)
			}
		}
	}
	return b.String()
}

// renderOpposingArgumentsBlock renders every first-party Argument in
// opposing into the plain-text block the argument-rebuttal template's
// opposing_arguments_block variable expects: one numbered entry per
// opposing argument, its issue, claim, and the counterarguments the first
// party itself already anticipated — fed in as a starting point for this
// agent's own targeted rebuttal, per the plan's rebuttal requirement.
//
// Only opposing.ID, opposing.IssueNodeID, opposing.Claim, and
// opposing.Counterarguments are read; this package never re-derives or
// mutates firstpartyagent's own grounding/scoring of these arguments.
func renderOpposingArgumentsBlock(opposing []firstpartyagent.Argument) string {
	if len(opposing) == 0 {
		return "none"
	}
	var b strings.Builder
	for i, arg := range opposing {
		fmt.Fprintf(&b, "%d. OPPOSING ARGUMENT [opposing_argument_id=%s] (issue %s)\n", i+1, arg.ID, arg.IssueNodeID)
		fmt.Fprintf(&b, "   claim: %s\n", arg.Claim)
		if len(arg.Counterarguments) == 0 {
			b.WriteString("   anticipated rebuttals: none\n")
		} else {
			b.WriteString("   anticipated rebuttals (starting point for your own rebuttal):\n")
			for _, c := range arg.Counterarguments {
				fmt.Fprintf(&b, "     - %s\n", c)
			}
		}
	}
	return b.String()
}

// buildArgumentPrompt renders tmpl with the case's per-issue evidence,
// the first party's opposing arguments to rebut, and
// jurisdiction/legal-family/party context.
func buildArgumentPrompt(tmpl *prompts.PromptTemplate, evidence []issueEvidence, opposing []firstpartyagent.Argument, jurisdictionName, legalFamily, partyLabel string) (string, error) {
	vars := map[string]string{
		"issues_block":             renderIssuesBlock(evidence),
		"opposing_arguments_block": renderOpposingArgumentsBlock(opposing),
		"jurisdiction_name":        jurisdictionName,
		"legal_family":             legalFamily,
		"party_label":              partyLabel,
	}
	return prompts.Render(tmpl, vars)
}
