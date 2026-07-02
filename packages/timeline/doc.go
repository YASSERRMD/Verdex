// Package timeline models the two parties (and their roles, counsel, and
// relationship) in a case, and assembles the chronological case timeline
// from extracted facts and statements, flagging conflicts where different
// parties assert incompatible claims about the same subject — so
// downstream IRAC (Issue/Rule/Application/Conclusion) reasoning has a
// structured "who said what happened, and when" view of the case to
// reason over.
//
// Core concepts:
//
//   - PartyRole / Party: the case-role taxonomy (first/second/third party)
//     and the party entity itself — ID, Role, Name, and optional Counsel
//     (party.go). This is a local equivalent of
//     packages/evidence.PartyRole rather than a hard cross-module
//     dependency, since this package's Party concept (role + name +
//     counsel + relationships) is a distinct concern from
//     packages/evidence's per-segment attribution heuristic.
//   - PartyFact / NewPartyFact: a factual assertion attributed to a party,
//     traceable back to its exact source segment and span
//     (fact_attribution.go).
//   - Event / ExtractDate / ExtractEvent: a dated (or undated) occurrence
//     in the case, extracted from segment text via deterministic,
//     regex-based date-pattern matching — no ML models (event.go).
//   - Timeline / AssembleTimeline: the ordered chronology — dated events
//     ascending by date, undated events grouped at the end, with stable
//     ordering for ties (assemble.go).
//   - Conflict / DetectConflicts: a starting heuristic that flags two
//     same-subject PartyFact entries from different parties whose text
//     contains a contradictory keyword pair (e.g. "did not"/"did") as a
//     potential conflict, optionally gated to facts whose underlying
//     events share a date (conflict.go).
//   - Claim / ValidateClaimLinkage: links a Party to the Event/PartyFact
//     entries it relies on, with an integrity check that every reference
//     resolves against known IDs (claim.go).
//   - Relationship: how the case's two (or more) parties relate to one
//     another — landlord-tenant, employer-employee, or any free-form Kind
//     (relationship.go).
//   - TimelineStore / InMemoryTimelineStore: the persistence contract for
//     a case's full party/timeline graph, keyed by case ID, with an
//     in-memory implementation requiring no real database dependency
//     (store.go).
//   - TimelineService: orchestrates the full pipeline — extract events
//     from segments -> attribute party facts -> assemble timeline ->
//     detect conflicts -> link claims -> persist -> return the assembled
//     Timeline (service.go).
//
// Design principles:
//
//   - No ML models. Date extraction and conflict detection are both
//     deterministic functions of regular-expression/lexical pattern
//     matching, mirroring packages/segmentation, packages/evidence, and
//     packages/category's "no ML models, rule based" design principle.
//   - Confidence, not certainty. Every Event carries a Confidence score in
//     the closed interval [0, 1], reflecting how specific the matched date
//     pattern was.
//   - Conflict detection is a starting heuristic, not a legal
//     determination. DetectConflicts flags candidates for human review; it
//     does not (and cannot, from lexical pattern matching alone) determine
//     which party's account is correct.
//
// See doc/party-timeline-model.md for a detailed model write-up.
package timeline
