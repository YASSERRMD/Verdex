// Package templates registers all built-in Verdex prompt templates into
// prompts.DefaultRegistry. Import this package for its side effects:
//
//	import _ "github.com/YASSERRMD/verdex/packages/prompts/templates"
package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

func init() {
	t := prompts.PromptTemplate{
		ID:          "irac.issue.extraction",
		Name:        "IRAC — Issue Extraction",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are an expert legal analyst specialising in {{index . "legal_family"}} legal systems.

JURISDICTION: {{index . "jurisdiction_name"}}
LEGAL FAMILY:  {{index . "legal_family"}}

TASK — ISSUE EXTRACTION
=======================
Read the case summary below and identify every distinct legal issue that must
be resolved in order to decide the case. For each issue:

1. State the issue as a precise legal question (e.g. "Whether the defendant
   owed a duty of care to the claimant under the circumstances.").
2. Identify the primary area of law (e.g. contract, tort, criminal, family,
   administrative, constitutional).
3. Note any sub-issues or threshold questions that must be resolved first.
4. Flag whether the issue is one of fact, one of law, or mixed.

Output the issues as a numbered list. Do NOT reach any conclusions or cite
authorities at this stage — issue identification only.

CASE SUMMARY
============
{{index . "case_summary"}}

Respond in valid JSON conforming to the schema:
{
  "issues": [
    {
      "number": 1,
      "question": "<legal question>",
      "area_of_law": "<area>",
      "sub_issues": ["<sub-issue>"],
      "issue_type": "fact | law | mixed"
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "case_summary", Required: true, Sanitize: true, MaxLen: 32000},
			{Name: "jurisdiction_name", Required: true, Sanitize: true, MaxLen: 256},
			{Name: "legal_family", Required: true, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("prompts/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
