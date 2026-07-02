package firstpartyagent

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/firstpartyagent/templates"
	"github.com/YASSERRMD/verdex/packages/prompts"
)

// resolveTemplate selects the best argument-construction prompt template
// for the requested locale and legal family, via
// prompts.VariantSelector{}.SelectBest's tiered fallback (exact ->
// locale-only -> family-only -> universal), mirroring
// packages/issueagent/prompt.go's resolveTemplate exactly.
//
// Returns ErrNoTemplate wrapping the underlying lookup failure if no
// variant is registered at any fallback tier.
func resolveTemplate(registry *prompts.Registry, locale, legalFamily string) (*prompts.PromptTemplate, error) {
	tmpl, err := (prompts.VariantSelector{}).SelectBest(registry, templates.ArgumentConstructionTemplateID, locale, legalFamily)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoTemplate, err)
	}
	return tmpl, nil
}

// renderIssuesBlock renders every issueEvidence into the plain-text block
// the argument-construction template's issues_block variable expects:
// one numbered entry per issue, its governing question(s), and the
// exact, exhaustive set of fact/rule IDs the model is permitted to cite
// as evidence for it.
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

// buildArgumentPrompt renders tmpl with the case's per-issue evidence and
// jurisdiction/legal-family/party context.
func buildArgumentPrompt(tmpl *prompts.PromptTemplate, evidence []issueEvidence, jurisdictionName, legalFamily, partyLabel string) (string, error) {
	vars := map[string]string{
		"issues_block":      renderIssuesBlock(evidence),
		"jurisdiction_name": jurisdictionName,
		"legal_family":      legalFamily,
		"party_label":       partyLabel,
	}
	return prompts.Render(tmpl, vars)
}
