package issueagent

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/issueagent/templates"
	"github.com/YASSERRMD/verdex/packages/prompts"
)

// resolveTemplate selects the best issue-framing prompt template for the
// requested locale and legal family, via
// prompts.VariantSelector{}.SelectBest's tiered fallback (exact ->
// locale-only -> family-only -> universal). This is the agent's
// jurisdiction-aware framing hook: a future locale- or
// legal-family-specific template variant is picked up automatically once
// registered, with no change to agent.go.
//
// Returns ErrNoTemplate wrapping the underlying lookup failure if no
// variant is registered at any fallback tier.
func resolveTemplate(registry *prompts.Registry, locale, legalFamily string) (*prompts.PromptTemplate, error) {
	tmpl, err := (prompts.VariantSelector{}).SelectBest(registry, templates.IssueFramingTemplateID, locale, legalFamily)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoTemplate, err)
	}
	return tmpl, nil
}

// renderIssuesBlock renders every issueContext into the plain-text block
// the issue-framing template's issues_block variable expects: one
// numbered entry per issue, its ID, question text, extraction confidence,
// and governing rules (or an explicit "none" marker).
func renderIssuesBlock(contexts []issueContext) string {
	var b strings.Builder
	for i, ic := range contexts {
		fmt.Fprintf(&b, "%d. [%s] %s (extraction_confidence=%.2f)\n", i+1, ic.Node.ID, ic.Node.Text, ic.Node.Confidence)
		if len(ic.GoverningRule) == 0 {
			b.WriteString("   governing rules: none\n")
			continue
		}
		b.WriteString("   governing rules:\n")
		for _, r := range ic.GoverningRule {
			fmt.Fprintf(&b, "     - [%s] %s\n", r.ID, r.Text)
		}
	}
	return b.String()
}

// buildFramingPrompt renders tmpl with the case's issue contexts and
// jurisdiction/legal-family context.
func buildFramingPrompt(tmpl *prompts.PromptTemplate, contexts []issueContext, jurisdictionName, legalFamily string) (string, error) {
	vars := map[string]string{
		"issues_block":      renderIssuesBlock(contexts),
		"jurisdiction_name": jurisdictionName,
		"legal_family":      legalFamily,
	}
	return prompts.Render(tmpl, vars)
}
