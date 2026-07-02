// Package lawapplication applies a case's controlling legal rules to its
// weighed facts, per issue, at the reasoning stage that sits between the
// evidence-weighing module (packages/evidenceweighing, Phase 053) and the
// downstream synthesis stage (Phase 055).
//
// # Not an LLM agent
//
// Like packages/evidenceweighing, this package is a deterministic,
// heuristic bookkeeping module — plain Go, unit-testable without any
// model call, in the same style as packages/application's
// WeightByLegalFamily (Phase 037) and packages/evidenceweighing's Weigh
// (Phase 053). See doc/law-application.md's "Design decision" section
// for the full rationale. This package does not import
// packages/agentframework, packages/router, or any provider — nothing in
// its analysis path makes a model call.
//
// # Scope
//
// Given a case's assembled reasoning tree (read via packages/knowledgeapi),
// both parties' already-constructed ArgumentSets (packages/firstpartyagent,
// packages/secondpartyagent), the issue framings from packages/issueagent,
// and the weighed-facts Result from packages/evidenceweighing, this
// package:
//
//   - Maps each issue to its controlling rules: the union of rules linked
//     via the tree's Rule--governs-->Issue edges and rules either party's
//     arguments actually cited for that issue.
//   - Builds an element-to-fact map per controlling rule: which weighed
//     facts, cited by which parties, back that rule's application to the
//     issue.
//   - Weights statute versus precedent by legal family (WeightByOrigin),
//     mirroring packages/application's WeightByLegalFamily concept at
//     this later reasoning stage.
//   - Flags ConflictingAuthority when two controlling rules for the same
//     issue were invoked by disjoint, opposing parties, rather than
//     silently picking one.
//   - Attaches a resolved citation to every controlling rule, tracking
//     unresolved/unverified citations as a quality signal rather than
//     dropping them.
//   - Derives a per-issue Confidence score with an explicit Steps
//     reasoning trail, and persists the full Result per case via a
//     Repository.
//
// See doc/law-application.md for the origin-inference limitation, the
// conflicting-authority handling, and how this feeds Phase 055.
package lawapplication
