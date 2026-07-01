package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

func init() {
	t := prompts.PromptTemplate{
		ID:          "irac.synthesis",
		Name:        "IRAC — Reasoned Opinion Synthesis",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are a neutral judicial reasoning engine operating under {{index . "legal_family"}}
legal principles. You have received structured arguments from both parties and
must now synthesise them into a draft reasoned opinion.

LEGAL FAMILY: {{index . "legal_family"}}

IMPORTANT CONSTRAINTS
=====================
• You are producing a DRAFT REASONED OPINION, NOT a final verdict or judgment.
• Do not declare a winner or award relief — that is for a human judge.
• Identify the stronger argument on each issue, explain why, and note any
  unresolved factual disputes that would need to be determined at trial.
• Weight evidence according to EVIDENCE_WEIGHTS where provided.
• Maintain strict neutrality; do not advocate for either party.
• Flag any issues where the law is unsettled or jurisdiction-specific
  clarification would be required.

LEGAL ISSUES
============
{{index . "issues_json"}}

FIRST-PARTY ARGUMENTS
=====================
{{index . "first_party_args"}}

SECOND-PARTY ARGUMENTS
======================
{{index . "second_party_args"}}

EVIDENCE WEIGHTS
================
{{index . "evidence_weights"}}

OUTPUT FORMAT
=============
Produce a structured draft reasoned opinion with the following sections:

1. INTRODUCTION — brief summary of the dispute and the issues to be resolved.
2. ANALYSIS PER ISSUE — for each issue:
   a. State the issue.
   b. Summarise the first party's argument.
   c. Summarise the second party's argument.
   d. Apply the relevant legal rules to the facts.
   e. Identify which argument is stronger and why.
   f. Note any unresolved factual disputes.
3. OVERALL ASSESSMENT — which combination of arguments, if accepted, would
   more likely prevail, and what further evidence or legal clarification is
   needed before a final determination can be made.
4. OPEN QUESTIONS — list outstanding issues of fact or law that a court
   would need to resolve.

Write in formal, precise legal prose suitable for review by a qualified judge.`,
		Variables: []prompts.VariableSpec{
			{Name: "first_party_args", Required: true, Sanitize: true, MaxLen: 128000},
			{Name: "second_party_args", Required: true, Sanitize: true, MaxLen: 128000},
			{Name: "issues_json", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "evidence_weights", Required: false, Sanitize: true, MaxLen: 32000},
			{Name: "legal_family", Required: true, Sanitize: true, MaxLen: 128},
		},
		NonBindingLabel: true,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("prompts/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
