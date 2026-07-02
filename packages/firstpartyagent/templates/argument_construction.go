// Package templates registers firstpartyagent's own prompt template(s)
// into prompts.DefaultRegistry, mirroring
// packages/issueagent/templates's registration style exactly.
//
// Import this package for its side effects:
//
//	import _ "github.com/YASSERRMD/verdex/packages/firstpartyagent/templates"
package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

// ArgumentConstructionTemplateID is the prompts.PromptTemplate.ID
// registered by this package's init(). firstpartyagent's agent.go
// resolves this ID via prompts.VariantSelector{}.SelectBest for
// jurisdiction-aware argument construction.
const ArgumentConstructionTemplateID = "firstpartyagent.argument.construction"

func init() {
	t := prompts.PromptTemplate{
		ID:          ArgumentConstructionTemplateID,
		Name:        "First-Party Agent — Argument Construction",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are an expert advocate constructing the strongest good-faith case for {{index . "party_label"}} in {{index . "legal_family"}} legal systems.

JURISDICTION: {{index . "jurisdiction_name"}}
LEGAL FAMILY:  {{index . "legal_family"}}
PARTY:         {{index . "party_label"}}

TASK — ARGUMENT CONSTRUCTION
=============================
You are given a list of already-framed legal issues for this case, each
with its governing question(s) and the facts/rules from the case's
reasoning tree available to support an argument. Your task is to
construct, for each issue, one or more arguments in {{index . "party_label"}}'s favor:

1. State a CLAIM: the core assertion this argument makes in
   {{index . "party_label"}}'s favor, answering the issue's governing question.
2. Cite SUPPORTING FACTS AND RULES: reference ONLY the fact/rule IDs
   listed below for that issue. Do NOT invent, guess, or reference any ID
   that is not explicitly listed — any argument citing an ID outside the
   provided list will be rejected.
3. Anticipate COUNTERARGUMENTS: list the most likely rebuttals an
   opposing party would raise against this specific argument.
4. Report your own CONFIDENCE in this argument's supporting-fact strength,
   in [0.0, 1.0] (distinct from citation verification, which is handled
   separately).

ISSUES AND AVAILABLE EVIDENCE
==============================
{{index . "issues_block"}}

Respond in valid JSON conforming to the schema:
{
  "arguments": [
    {
      "issue_node_id": "<issue node id>",
      "claim": "<the argument's claim>",
      "supporting_fact_ids": ["<fact node id>"],
      "supporting_rule_ids": ["<rule node id>"],
      "counterarguments": ["<likely rebuttal>"],
      "confidence": 0.0
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "issues_block", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "jurisdiction_name", Required: false, Sanitize: true, MaxLen: 256},
			{Name: "legal_family", Required: false, Sanitize: true, MaxLen: 128},
			{Name: "party_label", Required: true, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("firstpartyagent/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
