// Package uncertainty performs a cross-cutting honesty pass over the full
// Verdex reasoning pipeline's output for a case: the framed issues
// (packages/issueagent, Phase 050), the weighed evidence
// (packages/evidenceweighing, Phase 053), the applied law
// (packages/lawapplication, Phase 054), and the draft opinion
// (packages/synthesisagent, Phase 055). Its job is to make that combined
// output honest about where it is weak, per the plan's Phase 056 goal.
//
// # Not an LLM agent
//
// Like packages/evidenceweighing and packages/lawapplication, this
// package is a deterministic, heuristic scoring and text-scanning module
// — plain Go, unit-testable without any model call. The plan's language
// for this phase — "identify," "rank," "generate," "attach," "output" —
// matches that deterministic/heuristic style, not the LLM-agent style of
// packages/issueagent, packages/firstpartyagent, packages/secondpartyagent,
// or packages/synthesisagent. This package does not import
// packages/agentframework, packages/router, or any provider.
//
// # Composes with, does not duplicate
//
// packages/synthesisagent already computes a single WeakestLink string
// per TentativeConclusion, as part of its own synthesis output — the
// single supporting element that most threatens that one conclusion's
// reliability. This package is deliberately not a reimplementation of
// that: it is a separate, cross-cutting analysis that runs on top of the
// full pipeline's output (issueagent + evidenceweighing + lawapplication
// + synthesisagent, all four, together) to do more than a single string:
//
//   - It ranks uncertainties by their impact on the case's OUTCOME, not
//     just per-conclusion — an Uncertainty attached to a highly material
//     issue (issueagent.FramedIssue.MaterialityRank) outranks an
//     equally-severe Uncertainty attached to a peripheral one. See
//     rank.go.
//   - It surfaces multiple, distinct KINDS of uncertainty per issue, not
//     one: low confidence at each of three separate upstream stages
//     (issue framing, law application, and the conclusion itself),
//     evidentiary contradictions and gaps, and unsettled/conflicting
//     controlling authority — see identify.go.
//   - It generates explicit, human-readable Caveat text per finding,
//     worded for a reviewer rather than as an internal debugging label.
//     See caveat.go.
//   - It scans conclusion TEXT for over-confident phrasing as a
//     distinct, additional signal — WeakestLink says nothing about how a
//     conclusion is worded. See overconfidence.go.
//
// A caller wanting "what's the single weakest point of conclusion X"
// should keep reading synthesisagent.TentativeConclusion.WeakestLink.
// A caller wanting "rank every doubt across this whole case by how much
// it matters, and tell me if any conclusion is over-stating its
// confidence in its own wording" should call this package's Surface.
//
// # Distinct from irac.ContainsVerdictLanguage
//
// irac.ContainsVerdictLanguage checks for a narrow, fixed wordlist of
// verdict/directive phrasing ("guilty", "liable", "is hereby ordered",
// ...) — binding-outcome language that must never appear in reasoning
// output at all. This package's over-confidence scan checks for a
// different, disjoint wordlist of absolutist/hedge-free phrasing
// ("definitely", "certainly", "beyond doubt", "clearly proves",
// "undeniably", "without question") — language that may not assert a
// binding outcome but overstates the epistemic confidence a draft,
// non-binding analysis should ever claim. See overconfidence.go's
// wordlist and doc/uncertainty-surfacing.md.
//
// # What this package deliberately does not do
//
// See the closing section of doc/uncertainty-surfacing.md: in short,
// this package never mutates a synthesisagent.Opinion or rewrites
// TentativeConclusion.Text, and it never blocks a run — flagging
// over-confident phrasing here is a quality signal, not enforcement.
// Turning irac.ContainsVerdictLanguage (or any other check) into a hard
// gate that can block a run is Phase 057's job, layered on top of this
// package rather than duplicated here.
package uncertainty
