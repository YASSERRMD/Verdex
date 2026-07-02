// Package issue derives legal issues from case facts and claims,
// producing packages/irac IssueNodes in the case's reasoning tree.
//
// Core concepts:
//
//   - CandidateIssue / IssueIdentifier / RuleBasedIdentifier: the
//     pluggable issue-identification interface and its default,
//     deterministic implementation, which detects dispute/
//     question-indicating language patterns ("whether", "dispute",
//     "claims that", "denies", "alleges", "contends") and contradictory
//     statement pairs over packages/segmentation Segments (identify.go).
//   - MapClaimsToIssues / ClaimLink: maps packages/evidence
//     Classification entries of type TypeArgument or
//     TypeWitnessStatement to the CandidateIssues they relate to via a
//     keyword-overlap heuristic (claim_map.go).
//   - Dedup: a Jaccard token-overlap similarity heuristic that merges
//     near-duplicate CandidateIssues, keeping the union of source spans
//     and the max confidence (dedup.go).
//   - Decompose: splits a compound issue's text (one joining two
//     distinct legal questions with "and") into a parent plus
//     sub-issues, each carrying ParentIssueID back to the parent
//     (subissue.go).
//   - IssueLink / LinkIssues: associates each issue with the
//     packages/timeline Party IDs it mentions and any related
//     fact/segment IDs it concerns (link.go).
//   - ScoreConfidence: aggregates identification, claim-mapping, and
//     dedup-corroboration signals into a single normalized Confidence in
//     [0, 1] (confidence.go).
//   - ManualOverride / ApplyOverride: human-correction support that lets
//     a reviewer's judgment take precedence over the extractor, with the
//     original candidate preserved distinctly on
//     OverriddenIssue.Override.Previous rather than discarded
//     (override.go), mirroring packages/evidence's ManualOverride/
//     ApplyOverride pattern.
//   - ToIssueNode / PersistIssues: converts a CandidateIssue into an
//     irac.IssueNode and persists it via packages/graph's
//     GraphStore.CreateNode (persist.go).
//   - IssueExtractionService: orchestrates the full pipeline — identify
//     -> map claims -> dedup/merge -> decompose sub-issues -> link
//     parties/facts -> score confidence -> apply any override -> persist
//     -> return []irac.IssueNode (service.go).
//
// Design principles:
//
//   - No ML models. Every heuristic in this package is a deterministic
//     function of segmentation.SegmentType and regular-expression/
//     lexical pattern matching, mirroring packages/evidence and
//     packages/segmentation's "no ML models, rule based" design
//     principle. A future phase can swap in a real identification model
//     by implementing the IssueIdentifier interface; no caller needs to
//     change.
//   - Confidence, not certainty. Every CandidateIssue carries a
//     Confidence score in the closed interval [0, 1], aggregated across
//     every pipeline stage that produces a signal about how likely the
//     candidate is to be a genuine issue.
//   - Human correction is first-class, not a patch. ManualOverride and
//     ApplyOverride are dedicated types, not a mutation of CandidateIssue
//     in place — the extractor's original determination is always
//     recoverable from OverriddenIssue.Override.Previous.
//   - No edges without both endpoints. irac's edge-constraint table
//     (packages/irac/edge.go) has no legal edge whose source and target
//     are both NodeIssue, and this package produces no RuleNodes, so
//     PersistIssues persists nodes only; edge creation is deferred to
//     whichever future phase produces RuleNodes.
//
// See doc/issue-extraction.md for a detailed prose write-up.
package issue
