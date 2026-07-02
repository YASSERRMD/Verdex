// Package firstpartyagent is a tool-using reasoning agent, built on
// packages/agentframework, that constructs the strongest good-faith case
// for a single party ("the first party") across every issue an upstream
// packages/issueagent run has already framed.
//
// # Scope
//
// This is the second concrete agent built on packages/agentframework
// (Phase 049), consuming packages/issueagent's (Phase 050)
// IssueAnalysisResult as its input. For each FramedIssue, it retrieves
// FactNodes and RuleNodes from the case's already-assembled reasoning
// tree — via packages/knowledgeapi, never directly — favorable to the
// configured PartyID, constructs one or more Argument chains grounded
// exclusively in node IDs that actually exist in the tree, attaches
// verified citations to each argument's supporting rules, anticipates
// likely counterarguments, and scores each argument's strength. The
// result is a structured ArgumentSet, the input Phase 052's second-party
// (rebuttal) agent argues against and Phase 055's synthesis agent
// reconciles alongside every other agent's output.
//
// # Composes with, does not duplicate
//
// packages/issueagent frames and ranks issues already present in a case's
// tree; it never constructs arguments. This package is downstream: it
// takes a FramedIssue (or a full IssueAnalysisResult) as input and never
// re-derives issue framing or materiality ranking itself. Like
// packages/issueagent, this package never constructs, mutates, or
// persists any irac tree node or edge — its output (Argument /
// ArgumentSet) references existing FactNode/RuleNode IDs by string only.
//
// # Anti-fabrication grounding
//
// Every FactNode/RuleNode ID an Argument cites is cross-checked against
// the case's actual tree via knowledgeapi.KnowledgeAPI.GetNode before the
// Argument is accepted — a model response inventing a node ID that does
// not exist in the tree is rejected/flagged rather than trusted. See
// ground.go and doc/first-party-agent.md for the full design.
//
// # Model-agnostic by construction
//
// Every model call is routed through a caller-supplied *router.Router,
// via a packages/agentframework Runner — this package never imports
// packages/adapters or talks to a provider.LLMProvider directly.
//
// See doc/first-party-agent.md for the prompt template, the exact output
// shape, the strength-scoring formula, and the explicit boundary against
// packages/issueagent and packages/timeline.
package firstpartyagent
