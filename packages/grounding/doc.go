// Package grounding verifies that every assertion in a synthesized
// packages/synthesisagent Opinion is actually grounded in the case's own
// facts and applicable law, rather than trusting a model's prose at face
// value.
//
// This package sits downstream of packages/synthesisagent (Phase 055),
// the final stage of Part 5's reasoning pipeline: synthesisagent already
// grounds each TentativeConclusion's SupportingFactIDs/SupportingRuleIDs
// against the case's tree at the per-conclusion level (see
// packages/synthesisagent/ground.go) and packages/firstpartyagent /
// packages/secondpartyagent do the same for their own per-issue
// arguments. What none of those packages do is verify the assembled
// *opinion text itself* — the prose a human reviewer actually reads —
// still matches the facts and rules it claims to rely on once everything
// has been composed into one Opinion. This package is that final
// consistency pass: it extracts claims from Opinion.Conclusions[].Text,
// cross-checks every SupportingFactIDs/SupportingRuleIDs reference
// against the case's real irac.Node set, verifies every citation via
// packages/citation, checks numeric and date figures mentioned in the
// text against the underlying fact nodes, and produces a structured
// Report a caller can gate finalization on.
//
// See doc/grounding.md for the full design, including exactly what
// "grounded" means here and how this package relates to
// packages/citation, packages/treevalidation, and packages/guardrail.
package grounding
