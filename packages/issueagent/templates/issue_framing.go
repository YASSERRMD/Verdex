// Package templates registers issueagent's own prompt template(s) into
// prompts.DefaultRegistry.
//
// Design choice: registration target. packages/prompts/templates is the
// home for templates shared across the whole platform (e.g.
// "irac.issue.extraction", used by packages/issue at ingestion time).
// This package's template is specific to issueagent's own reasoning task
// — framing issues that already exist in the tree, not extracting them —
// so it lives package-locally under packages/issueagent/templates rather
// than being added to packages/prompts/templates. It still registers into
// the single shared prompts.DefaultRegistry (rather than a package-local
// *prompts.Registry) so that prompts.VariantSelector{}.SelectBest and
// prompts.DefaultRegistry.Latest work uniformly across every agent's
// templates for any future prompt-management tooling (e.g. a template
// admin UI) that lists "every registered template" from one place.
//
// Import this package for its side effects:
//
//	import _ "github.com/YASSERRMD/verdex/packages/issueagent/templates"
package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

// IssueFramingTemplateID is the prompts.PromptTemplate.ID registered by
// this package's init(). issueagent's agent.go resolves this ID via
// prompts.VariantSelector{}.SelectBest for jurisdiction-aware framing.
const IssueFramingTemplateID = "issueagent.issue.framing"

func init() {
	t := prompts.PromptTemplate{
		ID:          IssueFramingTemplateID,
		Name:        "Issue Agent — Issue Framing",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are an expert legal analyst framing the issues of a case for adjudication in {{index . "legal_family"}} legal systems.

JURISDICTION: {{index . "jurisdiction_name"}}
LEGAL FAMILY:  {{index . "legal_family"}}

TASK — ISSUE FRAMING
=====================
You are given a list of legal issues already identified for this case,
each with its ID, question text, and any governing rules already linked
to it in the case's reasoning tree. Your task is NOT to identify new
issues — it is to FRAME the existing issues for adjudication:

1. Rank every issue by MATERIALITY: how determinative it is of the case's
   overall outcome, on a scale from 0.0 (immaterial) to 1.0 (fully
   determinative). The most material issue should usually be a threshold
   question or the one on which the broadest relief depends.
2. For each issue, state the precise GOVERNING LEGAL QUESTION(S) it
   raises, refining the linked rule(s) into a specific question the
   adjudicator must answer (not a restatement of the rule's text).
3. Flag AMBIGUITIES: note when an issue has thin or missing rule linkage,
   when the facts underlying it are contradictory, or when its scope is
   unclear. Leave the list empty if none apply.
4. Report your own CONFIDENCE in this framing, in [0.0, 1.0].

ISSUES
======
{{index . "issues_block"}}

Respond in valid JSON conforming to the schema:
{
  "framed_issues": [
    {
      "source_issue_node_id": "<issue node id>",
      "materiality_score": 0.0,
      "governing_questions": ["<question>"],
      "ambiguities": ["<ambiguity>"],
      "confidence": 0.0
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "issues_block", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "jurisdiction_name", Required: false, Sanitize: true, MaxLen: 256},
			{Name: "legal_family", Required: false, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("issueagent/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
