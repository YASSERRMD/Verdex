package synthesisagent

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent/templates"
)

// resolveTemplate selects the best opinion-synthesis prompt template for
// the requested locale and legal family, via
// prompts.VariantSelector{}.SelectBest's tiered fallback, mirroring
// packages/firstpartyagent/prompt.go's resolveTemplate exactly.
//
// Returns ErrNoTemplate wrapping the underlying lookup failure if no
// variant is registered at any fallback tier.
func resolveTemplate(registry *prompts.Registry, locale, legalFamily string) (*prompts.PromptTemplate, error) {
	tmpl, err := (prompts.VariantSelector{}).SelectBest(registry, templates.OpinionSynthesisTemplateID, locale, legalFamily)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoTemplate, err)
	}
	return tmpl, nil
}

// renderIssuesBlock renders every issueSynthesisInput into the plain-text
// block the synthesis template's issues_block variable expects: one
// numbered entry per issue, its governing question(s), both parties'
// arguments, the issue's applied law, and the exact, exhaustive set of
// fact/rule IDs the model is permitted to cite.
func renderIssuesBlock(inputs []issueSynthesisInput) string {
	var b strings.Builder
	for i, in := range inputs {
		fmt.Fprintf(&b, "%d. ISSUE [%s]\n", i+1, in.Issue.SourceIssueNodeID)
		if len(in.Issue.GoverningQuestions) == 0 {
			fmt.Fprintf(&b, "   question: %s\n", in.Issue.Question)
		} else {
			b.WriteString("   governing questions:\n")
			for _, q := range in.Issue.GoverningQuestions {
				fmt.Fprintf(&b, "     - %s\n", q)
			}
		}

		renderArguments(&b, "first-party arguments", argumentClaims(in.FirstArguments))
		renderArguments(&b, "second-party arguments", secondPartyClaims(in.SecondArguments))

		if in.HasApplication {
			fmt.Fprintf(&b, "   controlling rules: %s\n", strings.Join(in.Application.ControllingRuleIDs, ", "))
			if len(in.Application.Conflicts) > 0 {
				b.WriteString("   conflicting authority detected:\n")
				for _, c := range in.Application.Conflicts {
					fmt.Fprintf(&b, "     - %s vs %s: %s\n", c.FirstRuleID, c.SecondRuleID, c.Rationale)
				}
			}
			fmt.Fprintf(&b, "   law-application confidence: %.2f\n", in.Application.Confidence)
		} else {
			b.WriteString("   controlling rules: none found\n")
		}

		if len(in.Facts) == 0 {
			b.WriteString("   available facts: none\n")
		} else {
			b.WriteString("   available facts:\n")
			for _, f := range in.Facts {
				weight, ok := in.FactWeights[f.ID]
				if ok {
					fmt.Fprintf(&b, "     - [%s] %s (weight=%.2f, contradicted=%t)\n", f.ID, f.Text, weight.Weight, weight.Contradicted)
				} else {
					fmt.Fprintf(&b, "     - [%s] %s (weight=unknown)\n", f.ID, f.Text)
				}
			}
		}

		if len(in.Rules) == 0 {
			b.WriteString("   available rules: none\n")
		} else {
			b.WriteString("   available rules:\n")
			for _, r := range in.Rules {
				fmt.Fprintf(&b, "     - [%s] %s\n", r.ID, r.Text)
			}
		}
	}
	return b.String()
}

// renderArguments writes label's claims block. Callers pass pre-extracted
// argumentClaims (via argumentClaims/secondPartyClaims) rather than a
// firstpartyagent.Argument/secondpartyagent.Argument slice directly,
// since those are independent, structurally similar types this function
// need not choose between.
func renderArguments(b *strings.Builder, label string, claims []argumentClaim) {
	if len(claims) == 0 {
		fmt.Fprintf(b, "   %s: none\n", label)
		return
	}
	fmt.Fprintf(b, "   %s:\n", label)
	for _, c := range claims {
		fmt.Fprintf(b, "     - [%s] (party=%s, strength=%.2f) %s\n", c.id, c.partyID, c.strength, c.claim)
	}
}

// argumentClaim is a party-agnostic view of a single Argument's claim,
// used only to render the synthesis prompt's arguments blocks without
// depending on which of firstpartyagent.Argument/secondpartyagent.Argument
// produced it.
type argumentClaim struct {
	id       string
	partyID  string
	claim    string
	strength float64
}

// argumentClaims converts firstpartyagent.Arguments into argumentClaims.
func argumentClaims(args []firstpartyagent.Argument) []argumentClaim {
	out := make([]argumentClaim, 0, len(args))
	for _, a := range args {
		out = append(out, argumentClaim{id: a.ID, partyID: string(a.PartyID), claim: a.Claim, strength: a.Strength})
	}
	return out
}

// secondPartyClaims converts secondpartyagent.Arguments into
// argumentClaims.
func secondPartyClaims(args []secondpartyagent.Argument) []argumentClaim {
	out := make([]argumentClaim, 0, len(args))
	for _, a := range args {
		out = append(out, argumentClaim{id: a.ID, partyID: string(a.PartyID), claim: a.Claim, strength: a.Strength})
	}
	return out
}

// buildSynthesisPrompt renders tmpl with the case's per-issue synthesis
// inputs and jurisdiction/legal-family context.
func buildSynthesisPrompt(tmpl *prompts.PromptTemplate, inputs []issueSynthesisInput, jurisdictionName, legalFamily string) (string, error) {
	vars := map[string]string{
		"issues_block":      renderIssuesBlock(inputs),
		"jurisdiction_name": jurisdictionName,
		"legal_family":      legalFamily,
	}
	return prompts.Render(tmpl, vars)
}
