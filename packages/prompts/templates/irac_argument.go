package templates

import (
	"log"

	"github.com/YASSERRMD/verdex/packages/prompts"
)

func init() {
	registerFirstPartyArgument()
	registerSecondPartyArgument()
}

func registerFirstPartyArgument() {
	t := prompts.PromptTemplate{
		ID:          "irac.argument.firstparty",
		Name:        "IRAC — First-Party Argument",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are a senior advocate presenting arguments for the FIRST PARTY (claimant /
applicant / prosecution) in a {{index . "jurisdiction_name"}} proceeding.

JURISDICTION: {{index . "jurisdiction_name"}}
PARTY:        {{index . "party_name"}} (First Party)

YOUR TASK
=========
For each legal issue listed in ISSUES_JSON, construct the strongest possible
argument for the first party applying the IRAC method:

  • ISSUE   — restate the issue as a question.
  • RULE    — identify the applicable rule(s) of law (statutes, precedents,
               principles). Cite specific provisions where possible.
  • APPLICATION — apply the rule to the facts in FACTS_JSON.
  • CONCLUSION  — state the conclusion the court should reach on this issue
                  in favour of the first party.

Be rigorous, structured, and adversarially persuasive. Acknowledge and rebut
foreseeable counter-arguments. Do not fabricate citations.

ISSUES
======
{{index . "issues_json"}}

FACTS
=====
{{index . "facts_json"}}

Respond in valid JSON:
{
  "party": "first",
  "arguments": [
    {
      "issue_number": 1,
      "issue_question": "<restated question>",
      "rule": "<rule(s) of law>",
      "application": "<application to facts>",
      "conclusion": "<conclusion favourable to first party>",
      "anticipated_counter": "<anticipated counter-argument and rebuttal>"
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "issues_json", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "facts_json", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "party_name", Required: true, Sanitize: true, MaxLen: 512},
			{Name: "jurisdiction_name", Required: true, Sanitize: true, MaxLen: 256},
		},
		NonBindingLabel: false,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("prompts/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}

func registerSecondPartyArgument() {
	t := prompts.PromptTemplate{
		ID:          "irac.argument.secondparty",
		Name:        "IRAC — Second-Party Argument",
		Version:     1,
		Locale:      "",
		LegalFamily: "",
		Body: `You are a senior advocate presenting arguments for the SECOND PARTY (respondent /
defendant) in a {{index . "jurisdiction_name"}} proceeding.

JURISDICTION: {{index . "jurisdiction_name"}}
PARTY:        {{index . "party_name"}} (Second Party)

YOUR TASK
=========
For each legal issue listed in ISSUES_JSON, construct the strongest possible
argument for the second party applying the IRAC method:

  • ISSUE   — restate the issue as a question.
  • RULE    — identify the applicable rule(s) of law (statutes, precedents,
               principles). Cite specific provisions where possible.
  • APPLICATION — apply the rule to the facts in FACTS_JSON, highlighting
                  favourable facts and challenging unfavourable characterisations.
  • CONCLUSION  — state the conclusion the court should reach on this issue
                  in favour of the second party.

Be rigorous, structured, and adversarially persuasive. Acknowledge and rebut
foreseeable counter-arguments. Do not fabricate citations.

ISSUES
======
{{index . "issues_json"}}

FACTS
=====
{{index . "facts_json"}}

Respond in valid JSON:
{
  "party": "second",
  "arguments": [
    {
      "issue_number": 1,
      "issue_question": "<restated question>",
      "rule": "<rule(s) of law>",
      "application": "<application to facts>",
      "conclusion": "<conclusion favourable to second party>",
      "anticipated_counter": "<anticipated counter-argument and rebuttal>"
    }
  ]
}`,
		Variables: []prompts.VariableSpec{
			{Name: "issues_json", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "facts_json", Required: true, Sanitize: true, MaxLen: 64000},
			{Name: "party_name", Required: true, Sanitize: true, MaxLen: 512},
			{Name: "jurisdiction_name", Required: true, Sanitize: true, MaxLen: 256},
		},
		NonBindingLabel: false,
	}

	if err := prompts.DefaultRegistry.Register(t); err != nil {
		log.Fatalf("prompts/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
	}
}
