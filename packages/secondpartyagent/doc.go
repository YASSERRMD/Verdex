// Package secondpartyagent is a tool-using reasoning agent, built on
// packages/agentframework, that constructs the strongest good-faith case
// for a single party ("the second party") across every issue an upstream
// packages/issueagent run has already framed, and rebuts the first
// party's already-constructed arguments (packages/firstpartyagent).
//
// # Scope
//
// This is the direct adversarial counterpart to packages/firstpartyagent
// (Phase 051): it consumes the same packages/issueagent (Phase 050)
// IssueAnalysisResult as its input, plus the first party's
// firstpartyagent.ArgumentSet. For each FramedIssue, it retrieves
// FactNodes and RuleNodes from the case's already-assembled reasoning
// tree — via packages/knowledgeapi, never directly — favorable to the
// configured PartyID, constructs one or more Argument chains grounded
// exclusively in node IDs that actually exist in the tree, attaches
// verified citations to each argument's supporting rules, anticipates
// likely counterarguments, targets and rebuts specific first-party
// arguments by ID, and scores each argument's strength. The result is a
// structured ArgumentSet, the input Phase 053's evidence-weighing module
// and Phase 055's synthesis agent consume alongside the first party's own
// output.
//
// # Composes with, does not duplicate
//
// packages/issueagent frames and ranks issues already present in a
// case's tree; it never constructs arguments. packages/firstpartyagent
// constructs the first party's own affirmative case; this package never
// re-derives or duplicates that logic — it imports firstpartyagent only
// for its exported ArgumentSet/Argument/CitationRef types, to read the
// first party's Claim and Counterarguments as a rebuttal starting point.
// Like firstpartyagent, this package never constructs, mutates, or
// persists any irac tree node or edge — its own output (Argument /
// ArgumentSet) references existing FactNode/RuleNode IDs, and the first
// party's Argument IDs, by string only.
//
// # Anti-fabrication grounding
//
// Every FactNode/RuleNode ID an Argument cites is cross-checked against
// the case's actual tree, exactly as packages/firstpartyagent does. This
// package additionally cross-checks every RebutsArgumentIDs entry against
// the real set of firstpartyagent.Argument.ID values supplied as input —
// a model inventing either a node ID or an opposing-argument ID that does
// not exist is rejected/flagged rather than trusted. See ground.go and
// doc/second-party-agent.md for the full design.
//
// # Model-agnostic by construction
//
// Every model call is routed through a caller-supplied *router.Router,
// via a packages/agentframework Runner — this package never imports
// packages/adapters or talks to a provider.LLMProvider directly.
//
// See doc/second-party-agent.md for the prompt template, the exact
// output shape, the strength-scoring formula, the rebuttal-grounding
// addition, and the explicit boundary against packages/firstpartyagent,
// packages/issueagent, and packages/timeline.
package secondpartyagent
