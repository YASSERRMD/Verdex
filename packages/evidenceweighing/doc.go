// Package evidenceweighing assesses the reliability and relative weight of
// competing evidence for a case, at the reasoning stage that sits between
// the adversarial argument agents (packages/firstpartyagent, Phase 051;
// packages/secondpartyagent, Phase 052) and the downstream law-application
// and synthesis stages (Phase 054, Phase 055).
//
// # Not an LLM agent
//
// Unlike the argument-construction agents this package consumes, this
// package is a deterministic, heuristic scoring module — plain Go,
// unit-testable without any model call, in the same style as
// packages/fact's ReliabilityScore (Phase 034), packages/application's
// WeightByLegalFamily (Phase 037), and packages/precedent's AuthorityScore
// (Phase 037). See doc/evidence-weighing.md's "Design decision: heuristic,
// not LLM-backed" section for the full rationale. This package does not
// import packages/agentframework, packages/router, or any provider —
// nothing in its scoring path makes a model call.
//
// # Scope
//
// Given a case's assembled reasoning tree (read via packages/knowledgeapi)
// and the two parties' already-constructed ArgumentSets
// (packages/firstpartyagent, packages/secondpartyagent), this package:
//
//   - Scores each cited FactNode's reliability from the signals actually
//     available at this stage: its Confidence, how many independent
//     arguments/claims corroborate its use, and whether it is used to
//     support mutually exclusive claims by opposing parties.
//   - Flags Contradictions: a fact cited by both parties' arguments in
//     support of mutually exclusive claims for the same issue.
//   - Applies a jurisdiction-aware LegalFamily weighting profile (e.g.
//     common-law testimony-heavy vs civil-law documentary-heavy emphasis)
//     as a multiplier on the base score.
//   - Surfaces gaps in the evidentiary record: SupportingFactIDs that do
//     not resolve to a real FactNode in the tree, and issues neither party
//     cited any fact for.
//   - Produces one FactWeight per referenced fact, each carrying a
//     human-readable Rationale explaining how its score was derived, and
//     persists the full EvidenceWeighingResult per case via a Repository.
//
// See doc/evidence-weighing.md for the rubric's exact coefficients, the
// jurisdiction weighting profiles, and how this feeds Phase 054 and
// Phase 055.
package evidenceweighing
