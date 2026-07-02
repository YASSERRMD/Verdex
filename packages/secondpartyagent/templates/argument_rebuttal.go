// Package templates registers secondpartyagent's own prompt template(s)
// into prompts.DefaultRegistry, mirroring
// packages/firstpartyagent/templates's registration style exactly.
//
// Import this package for its side effects:
//
//	import _ "github.com/YASSERRMD/verdex/packages/secondpartyagent/templates"
package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

// ArgumentRebuttalTemplateID is the prompts.PromptTemplate.ID registered
// by this package's init(). secondpartyagent's agent.go resolves this ID
// via prompts.VariantSelector{}.SelectBest for jurisdiction-aware
// second-party argument construction and rebuttal.
const ArgumentRebuttalTemplateID = "secondpartyagent.argument.rebuttal"

func init() {
	t := prompts.PromptTemplate{
		ID:          ArgumentRebuttalTemplateID,
		Name:        "Second-Party Agent — Argument Construction & Rebuttal",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are an expert advocate constructing the strongest good-faith case for {{index . "party_label"}} in {{index . "legal_family"}} legal systems, and rebutting the opposing party's arguments.

JURISDICTION: {{index . "jurisdiction_name"}}
LEGAL FAMILY:  {{index . "legal_family"}}
PARTY:         {{index . "party_label"}}

TASK — ARGUMENT CONSTRUCTION AND REBUTTAL
===========================================
You are given a list of already-framed legal issues for this case, each
with its governing question(s) and the facts/rules from the case's
reasoning tree available to support an argument. You are also given the
opposing (first) party's arguments already constructed for these same
issues, including the rebuttals that party itself anticipated. Your task
is to construct, for each issue, one or more arguments in
{{index . "party_label"}}'s favor that both advance {{index . "party_label"}}'s
own affirmative case AND directly rebut the opposing party's arguments:

1. State a CLAIM: the core assertion this argument makes in
   {{index . "party_label"}}'s favor, answering the issue's governing question.
2. Cite SUPPORTING FACTS AND RULES: reference ONLY the fact/rule IDs
   listed below for that issue. Do NOT invent, guess, or reference any ID
   that is not explicitly listed — any argument citing an ID outside the
   provided list will be rejected.
3. REBUT specific opposing arguments: for each opposing argument this
   claim undermines, reference its exact opposing_argument_id from the
   list below in rebuts_argument_ids. Use the opposing party's own listed
   claim and anticipated counterarguments as your starting point for
   constructing a targeted rebuttal rather than a generic denial. Do NOT
   invent an opposing_argument_id that is not explicitly listed.
4. Anticipate COUNTERARGUMENTS: list the most likely rebuttals the
   opposing party could still raise against this specific argument.
5. Report your own CONFIDENCE in this argument's supporting-fact strength,
   in [0.0, 1.0] (distinct from citation verification, which is handled
   separately).

ISSUES AND AVAILABLE EVIDENCE
==============================
{{index . "issues_block"}}

OPPOSING PARTY'S ARGUMENTS TO REBUT
=====================================
{{index . "opposing_arguments_block"}}

Respond in valid JSON conforming to the schema:
{
  "arguments": [
    {
      "issue_node_id": "<issue node id>",
      "claim": "<the argument's claim>",
      "supporting_fact_ids": ["<fact node id>"],
      "supporting_rule_ids": ["<rule node id>"],
      "rebuts_argument_ids": ["<opposing_argument_id>"],
      "counterarguments": ["<likely rebuttal>"],
      "confidence": 0.0
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "issues_block", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "opposing_arguments_block", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "jurisdiction_name", Required: false, Sanitize: true, MaxLen: 256},
			{Name: "legal_family", Required: false, Sanitize: true, MaxLen: 128},
			{Name: "party_label", Required: true, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("secondpartyagent/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
