// Package guardrail is Verdex's project-wide, hard-enforcement policy
// layer for the platform's single most important guarantee: every
// reasoning output the platform produces is a non-binding DRAFT ANALYSIS,
// never a verdict. CONTRIBUTING.md states this as a rule ("Every module
// that produces reasoning output must attach the draft_analysis label.
// Verdict or directive language is rejected by the output pipeline.");
// this package is what turns that sentence into code that every
// reasoning-output-producing package can (and, per policy, must) call.
//
// # Composes with, does not duplicate
//
// This package does not reimplement any of the primitives it depends on.
// It wraps and strengthens them:
//
//   - packages/irac (guardrail.go) already defines DraftAnalysisLabel,
//     NewConclusionNode (the only ConclusionNode constructor, which
//     unconditionally attaches the label), IsDraftAnalysis, and
//     ContainsVerdictLanguage. This package's Label/OutputLabel type
//     wraps irac.DraftAnalysisLabel rather than redefining it, and
//     CheckText wraps irac.ContainsVerdictLanguage rather than
//     reimplementing keyword matching.
//
//   - packages/synthesisagent's Provider (provider.go, Phase 055)
//     already called irac.ContainsVerdictLanguage directly, inline,
//     before ever constructing an irac.ConclusionNode. This package
//     generalizes that inline check into a reusable policy function
//     (CheckText) and packages/synthesisagent has been refactored to
//     call it instead of duplicating the check — see
//     packages/synthesisagent/provider.go and its doc/synthesis-agent.md.
//
//   - packages/prompts (render.go) already injects a non-binding
//     disclaimer into rendered prompt bodies via
//     PromptTemplate.NonBindingLabel. That mechanism is for
//     LLM-facing INPUT (a prompt about to be sent to a model). This
//     package's RequireDisclaimer/EnsureDisclaimer is the equivalent
//     mechanism for reasoning OUTPUT (e.g. a rendered Opinion or an
//     exported report shown to a human) — a structurally similar but
//     distinct surface, deliberately mirrored rather than reused,
//     since prompts.Render operates on *prompts.PromptTemplate and has
//     no notion of an arbitrary output string.
//
//   - packages/treevalidation (gate.go) already defines the hard gate
//     for tree STRUCTURAL integrity (CanFinalize / ErrCriticalFindings).
//     This package's CanFinalize is a separate, narrower gate for
//     HUMAN SIGN-OFF state — a different precondition entirely, not a
//     replacement for treevalidation's gate. A caller that needs both
//     guarantees calls both gates; neither package imports the other.
//
//   - packages/knowledgeisolation (audit.go, Phase 047) already
//     established the AccessAttempt/AlertSink audit convention for
//     "record every violation, forward to a pluggable sink, default to
//     a no-op." This package's Event/AlertSink mirrors that
//     shape exactly, rather than inventing a new audit style.
//
// # Primary surface
//
//   - Label / OutputLabel (label.go): the draft_analysis label,
//     wrapping irac.DraftAnalysisLabel, plus the Labeled interface any
//     reasoning-output type can satisfy.
//   - RequireLabel / ValidateLabeled (label.go): hard checks that a
//     label (or a Labeled value) carries the mandatory label.
//   - CheckText (verdict.go): rejects verdict/directive phrasing with a
//     typed, errors.Is-compatible error instead of a bare bool.
//   - RequireDisclaimer / EnsureDisclaimer (disclaimer.go): idempotent,
//     mandatory disclaimer injection for reasoning-output text.
//   - SignoffStatus / SignoffGate / NoSignoffRecordedGate / CanFinalize
//     (signoff.go): the forward-looking, fail-closed extension point for
//     Phase 068's human sign-off workflow.
//   - Event / ViolationKind / AlertSink (audit.go): the audit
//     trail for every guardrail check that fails.
//
// # Why override-prevention matters here specifically
//
// Every other policy bug in this platform is recoverable: a bad
// citation can be corrected, a mis-weighted fact can be reweighted. A
// reasoning output that reaches a human, a filing, or a downstream
// system carrying verdict/directive language — even once — violates the
// platform's core legal and ethical premise, because a human reader has
// no way to know, after the fact, that what they read was supposed to
// have been blocked. That is why every check in this package returns
// only an error (never a bool a caller could silently ignore), why there
// is no "skip validation" flag anywhere in this package, and why
// CanFinalize fails closed (SignoffPending, not SignoffApproved) when no
// sign-off decision has ever been recorded. See
// doc/guardrail-policy.md for the full policy write-up.
package guardrail
