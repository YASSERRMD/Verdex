// Package synthesisagent implements Verdex's capstone reasoning agent: it
// weighs both parties' constructed arguments (packages/firstpartyagent,
// packages/secondpartyagent), the case's weighed evidence
// (packages/evidenceweighing), and its applied law
// (packages/lawapplication), resolved per issue
// (packages/issueagent), into a single draft, non-binding reasoned
// opinion.
//
// # Not a verdict
//
// Every TentativeConclusion this package produces is a draft analysis,
// never a binding determination. The mandatory non-binding guardrail is
// enforced structurally, not just by convention: Opinion's
// ConclusionProvider adapter (provider.go) only ever constructs
// irac.ConclusionNodes via irac.NewConclusionNode, which unconditionally
// attaches the draft_analysis label, and it additionally rejects any
// TentativeConclusion whose Text trips irac.ContainsVerdictLanguage
// before it ever reaches that constructor. Full guardrail POLICY
// enforcement (blocking a run entirely, human-review gating, etc.) is
// Phase 057's job — this package's job is narrower: never itself emit
// verdict-flavored output.
//
// # An LLM agent, like Phases 050-052
//
// Unlike packages/evidenceweighing and packages/lawapplication (Phases
// 053-054, deterministic bookkeeping with no model call), this package IS
// an LLM reasoning agent, per the implementation plan's Phase 055 title.
// It implements agentframework.Agent and is driven by
// agentframework.Runner, routing its single structured-synthesis model
// call through packages/router — never a hardcoded provider.
//
// # Composition
//
//   - packages/issueagent supplies the FramedIssues to resolve.
//   - packages/firstpartyagent and packages/secondpartyagent supply the
//     two parties' competing ArgumentSets for those issues.
//   - packages/evidenceweighing supplies per-fact Weight/Contradicted
//     signals.
//   - packages/lawapplication supplies each issue's ControllingRuleIDs,
//     Citations, Conflicts, and its own Confidence/Steps trail.
//   - packages/knowledgeapi is this package's only access path to the
//     case's actual tree, used both to gather per-issue prompt context
//     (fetch.go) and to cross-check the model's proposed supporting node
//     IDs against reality (ground.go), mirroring
//     packages/firstpartyagent's and packages/secondpartyagent's own
//     anti-fabrication convention exactly.
//
// # The ConclusionProvider extension point
//
// packages/treeassembly has, since Phase 039, defined a
// ConclusionProvider interface and documented that "once Phase 055 lands,
// it need only implement ConclusionProvider and be wired in" — this
// package is that Phase 055. See provider.go's Provider type and
// doc/synthesis-agent.md for the wiring: Provider converts each
// TentativeConclusion into an irac.ConclusionNode (or rejects/logs it, if
// its Text carries verdict language), letting a caller pass a
// synthesisagent.Provider directly to treeassembly.ComposeTree in place
// of treeassembly.NoOpConclusionProvider.
//
// # What this package deliberately does not do
//
// See the closing section of doc/synthesis-agent.md.
package synthesisagent
