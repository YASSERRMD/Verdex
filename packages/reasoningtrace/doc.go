// Package reasoningtrace assembles a fully auditable trace of one case's
// run through packages/reasoningorchestration: every agent step and tool
// call, every retrieved node and citation, a human-readable narrative
// linking back to tree nodes, an expandable evidence/authority trail per
// conclusion, export formats, access control, and a tamper-evidence
// integrity hash.
//
// This is the final auditability layer over Part 5 (Reasoning &
// Adversarial Synthesis): issueagent, firstpartyagent, secondpartyagent,
// evidenceweighing, lawapplication, synthesisagent, uncertainty, and
// guardrail each already know how to do their own piece of reasoning;
// this package is the first one whose entire job is explaining, after
// the fact, what the pipeline did and why — never re-deriving or
// second-guessing any conclusion.
//
// See doc/reasoning-trace.md for the full design.
package reasoningtrace
