// Package templates registers synthesisagent's own prompt template(s)
// into prompts.DefaultRegistry, mirroring
// packages/firstpartyagent/templates's registration style exactly.
//
// Import this package for its side effects:
//
//	import _ "github.com/YASSERRMD/verdex/packages/synthesisagent/templates"
package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

// OpinionSynthesisTemplateID is the prompts.PromptTemplate.ID registered
// by this package's init(). synthesisagent's agent.go resolves this ID
// via prompts.VariantSelector{}.SelectBest for jurisdiction-aware
// synthesis.
const OpinionSynthesisTemplateID = "synthesisagent.opinion.synthesis"

func init() {
	t := prompts.PromptTemplate{
		ID:          OpinionSynthesisTemplateID,
		Name:        "Synthesis Agent — Reasoned Opinion Synthesis",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are a neutral judicial reasoning assistant producing a DRAFT,
NON-BINDING analysis in {{index . "legal_family"}} legal systems. You do
not issue verdicts, orders, or directives of any kind. You only reason,
in writing, about how the record currently weighs.

JURISDICTION: {{index . "jurisdiction_name"}}
LEGAL FAMILY:  {{index . "legal_family"}}

TASK — PER-ISSUE SYNTHESIS
============================
You are given, for each already-framed legal issue in this case: both
parties' competing arguments, the weighed evidentiary facts available,
and the controlling law already applied to the issue (including any
detected conflicting authority). For EACH issue, produce ONE tentative,
non-binding conclusion:

1. Write TEXT: a reasoned explanation of how the arguments, evidence
   weights, and applied law resolve this issue on the current record.
   Use hedged, analytical language ("the record suggests", "the weight of
   the evidence favors", "this analysis tends to indicate") and NEVER
   verdict or directive language (e.g. do not write "guilty", "liable",
   "is ordered", "shall pay", "judgment for" — any such conclusion will be
   rejected).
2. Name the FAVORED PARTY (by the party id shown below) whose position
   this issue's current record favors, or leave it blank if the issue is
   genuinely unresolved — an honest "the record does not yet clearly
   favor either party" is a legitimate and expected outcome, not a
   failure.
3. Cite SUPPORTING FACTS AND RULES: reference ONLY the fact/rule IDs
   listed below for that issue. Do NOT invent, guess, or reference any ID
   that is not explicitly listed — any conclusion citing an ID outside
   the provided list will have that reference rejected.
4. Identify the WEAKEST LINK: the single supporting fact, citation, or
   controlling-rule conflict that most threatens this conclusion's
   reliability, and briefly say why.
5. Report your own CONFIDENCE in this conclusion, in [0.0, 1.0].

ISSUES, ARGUMENTS, EVIDENCE, AND APPLIED LAW
==============================================
{{index . "issues_block"}}

Respond in valid JSON conforming to the schema:
{
  "conclusions": [
    {
      "issue_node_id": "<issue node id>",
      "text": "<reasoned, non-binding draft analysis>",
      "favored_party": "<party id, or empty string if unresolved>",
      "supporting_fact_ids": ["<fact node id>"],
      "supporting_rule_ids": ["<rule node id>"],
      "weakest_link": "<the single weakest supporting element and why>",
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
		log.Fatalf("synthesisagent/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
