// Package fact builds packages/irac FactNodes with evidence backing,
// party attribution, temporal anchoring, and reliability scoring, from
// classified segments (packages/evidence.Classification).
//
// Core concepts:
//
//   - BuildFactNode: converts a single evidence.Classification, plus its
//     originating segment text and source span, into a candidate
//     irac.FactNode (build.go). evidence.Classification carries only a
//     SegmentID, not the segment's own text/span, so callers supply both
//     explicitly, mirroring packages/issue's "segmentText
//     map[string]string" bridging convention.
//   - EvidenceRef: links a fact node to its originating
//     evidence.EvidenceType and classification ID, so every fact traces
//     to why it is believed — testimony, documentary/physical exhibit,
//     statutory citation, or argument (evidence_ref.go).
//   - PartyAttribution: attaches a case-side PartyID/PartyRole to a fact
//     node by matching the originating evidence.PartyRole against the
//     case's timeline.Party roster (party_attribution.go).
//   - DisputeStatus / DetermineDisputeStatus: flags a fact Disputed when
//     a peer fact attributed to a different party asserts a
//     contradictory version of the same claim, via a lexical
//     contradiction heuristic re-implemented locally (conceptually
//     mirroring, but not depending on the internals of,
//     packages/timeline/conflict.go) (dispute.go).
//   - TemporalAnchor / AnchorToEvent: links a fact to a timeline.Event by
//     shared originating segment (strongest signal) or description-text
//     overlap, carrying the event's OccurredAt forward (temporal.go).
//   - CorroborationLink / DetectCorroboration: detects candidate
//     corroboration between independent (different-party) fact nodes via
//     a symmetric text/subject-overlap heuristic (corroboration.go).
//   - ReliabilityScore: combines classification confidence,
//     corroboration count, and dispute status into a single [0, 1]
//     reliability signal, deliberately separate from a node's raw
//     extraction Confidence (reliability.go).
//   - PersistFacts: persists irac.FactNodes via packages/graph's
//     GraphStore.CreateNode, and creates Fact--supports-->Application
//     edges (the only legal edge triple with a FactNode source, per
//     packages/irac/edge.go's constraint table) linking facts to any
//     existing irac.ApplicationNodes for the case (persist.go).
//   - FactConstructionService: orchestrates the full pipeline — build ->
//     attach evidence ref -> attribute party -> flag dispute -> anchor
//     temporally -> link corroboration -> score reliability -> persist
//     -> return []irac.FactNode (service.go), mirroring
//     packages/issue's IssueExtractionService orchestration pattern.
//
// Design principles:
//
//   - No ML models. Every heuristic in this package (dispute detection,
//     corroboration detection, temporal anchoring's text-overlap
//     fallback) is a deterministic function of lexical/token-overlap
//     pattern matching, mirroring packages/evidence, packages/timeline,
//     and packages/issue's shared "no ML models, rule based" design
//     principle. A future phase can swap in a real model for any of
//     these heuristics without changing this package's public API.
//   - Reliability is distinct from confidence. A fact node's raw
//     irac.Node.Confidence reflects only extraction/classification
//     confidence. ReliabilityScore is a separate signal folding in
//     corroboration and dispute status, exposed alongside (not in place
//     of) the raw Confidence via FactDetail.
//   - No hard dependency on packages/timeline/conflict.go's internals.
//     Dispute detection re-implements the contradiction-keyword
//     heuristic locally rather than importing packages/timeline's
//     Conflict/DetectConflicts machinery, since this package operates
//     over irac.FactNode text and party attribution, not
//     timeline.PartyFact.
//   - No edges without both endpoints. Fact--supports-->Application
//     edges are only created when the target ApplicationNode already
//     exists in the store for the case; PersistFacts never creates
//     ApplicationNodes itself.
//
// See doc/fact-model.md for a detailed prose write-up.
package fact
